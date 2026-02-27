package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"fnos-store/internal/config"
)

type mirrorCheckResult struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	LatencyMs int    `json:"latency_ms"`
	Status    string `json:"status"` // "ok", "timeout", "error"
}

type mirrorCheckResponse struct {
	GitHubMirrors []mirrorCheckResult `json:"github_mirrors"`
	DockerMirrors []mirrorCheckResult `json:"docker_mirrors"`
}

const checkTimeout = 5 * time.Second

func (s *Server) handleCheckMirrors(w http.ResponseWriter, r *http.Request) {
	checkType := r.URL.Query().Get("type") // "github", "docker", or "" (both)

	ghMirrors := config.GitHubMirrorOptions()
	dkMirrors := config.DockerMirrorOptions()

	cfg := config.Config{}
	if s.configMgr != nil {
		cfg = s.configMgr.Get()
	}

	skipGH := checkType == "docker"
	skipDK := checkType == "github"

	total := len(ghMirrors) + len(dkMirrors)
	type indexedResult struct {
		index  int
		result mirrorCheckResult
		isGH   bool
	}
	results := make(chan indexedResult, total)

	var wg sync.WaitGroup

	// Check GitHub mirrors concurrently
	if !skipGH {
		for i, m := range ghMirrors {
			if m.Key == "direct" || m.Key == "custom" {
				continue
			}
			wg.Add(1)
			go func(idx int, mirror config.GitHubMirror) {
				defer wg.Done()
				testURL := mirror.URL + "https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-amd64"
				latency, status := checkURL(r.Context(), testURL)
				results <- indexedResult{
					index:  idx,
					isGH:   true,
					result: mirrorCheckResult{Key: mirror.Key, Label: mirror.Label, LatencyMs: latency, Status: status},
				}
			}(i, m)
		}
	}

	// Check custom GitHub mirror if configured
	if !skipGH && cfg.CustomGitHubMirror != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testURL := cfg.CustomGitHubMirror + "https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-amd64"
			latency, status := checkURL(r.Context(), testURL)
			results <- indexedResult{
				index:  -1,
				isGH:   true,
				result: mirrorCheckResult{Key: "custom", Label: "自定义", LatencyMs: latency, Status: status},
			}
		}()
	}

	// Check Docker mirrors concurrently
	if !skipDK {
		for i, m := range dkMirrors {
			if m.Key == "direct" || m.Key == "custom" {
				continue
			}
			wg.Add(1)
			go func(idx int, mirror config.DockerMirror) {
				defer wg.Done()
				testURL := fmt.Sprintf("https://%sv2/", mirror.URL)
				latency, status := checkURL(r.Context(), testURL)
				results <- indexedResult{
					index:  idx,
					isGH:   false,
					result: mirrorCheckResult{Key: mirror.Key, Label: mirror.Label, LatencyMs: latency, Status: status},
				}
			}(i, m)
		}
	}

	// Check custom Docker mirror if configured
	if !skipDK && cfg.CustomDockerMirror != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testURL := fmt.Sprintf("https://%sv2/", cfg.CustomDockerMirror)
			latency, status := checkURL(r.Context(), testURL)
			results <- indexedResult{
				index:  -1,
				isGH:   false,
				result: mirrorCheckResult{Key: "custom", Label: "自定义", LatencyMs: latency, Status: status},
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var ghResults, dkResults []mirrorCheckResult
	for r := range results {
		if r.isGH {
			ghResults = append(ghResults, r.result)
		} else {
			dkResults = append(dkResults, r.result)
		}
	}

	if ghResults == nil {
		ghResults = []mirrorCheckResult{}
	}
	if dkResults == nil {
		dkResults = []mirrorCheckResult{}
	}

	writeJSON(w, http.StatusOK, mirrorCheckResponse{
		GitHubMirrors: ghResults,
		DockerMirrors: dkResults,
	})
}

func checkURL(parent context.Context, url string) (latencyMs int, status string) {
	ctx, cancel := context.WithTimeout(parent, checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, "error"
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return int(elapsed.Milliseconds()), "timeout"
		}
		return int(elapsed.Milliseconds()), "error"
	}
	defer resp.Body.Close()

	// Any response (even 401/403) proves the mirror is reachable
	return int(elapsed.Milliseconds()), "ok"
}
