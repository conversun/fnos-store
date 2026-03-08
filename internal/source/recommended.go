package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fnos-store/internal/config"
)

const defaultRecommendedJSONURL = "https://raw.githubusercontent.com/conversun/fnos-apps/main/recommended.json"

type RecommendedApp struct {
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	IconURL       string `json:"icon_url"`
	SourceURL     string `json:"source_url"`
	GitHubRepo    string `json:"github_repo"`
	LatestVersion string `json:"latest_version"`
	UpdatedAt     string `json:"updated_at"`
}

type recommendedPayload struct {
	Apps []RecommendedApp `json:"apps"`
}

type RecommendedSource struct {
	httpClient     *http.Client
	recommendedURL string
	cachePath      string
	localPath      string
	configMgr      *config.Manager
}

func NewRecommendedSource(cachePath, localPath string, cfgMgr *config.Manager) *RecommendedSource {
	return &RecommendedSource{
		httpClient:     &http.Client{Timeout: 20 * time.Second},
		recommendedURL: defaultRecommendedJSONURL,
		cachePath:      cachePath,
		localPath:      localPath,
		configMgr:      cfgMgr,
	}
}

func (s *RecommendedSource) FetchRecommendedApps(ctx context.Context) ([]RecommendedApp, error) {
	var cfg config.Config
	if s.configMgr != nil {
		cfg = s.configMgr.Get()
	} else {
		cfg = config.Config{Mirror: config.DefaultMirror}
	}

	for _, prefix := range config.GitHubFallbackPrefixes(cfg.Mirror, cfg) {
		url := s.recommendedURL
		if prefix != "" {
			url = prefix + s.recommendedURL
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			continue
		}

		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			continue
		}

		apps, err := s.decodeRecommended(raw)
		if err != nil {
			continue
		}

		_ = s.writeCache(raw)
		return apps, nil
	}

	if cached, err := s.readCache(); err == nil {
		return cached, nil
	}

	if local, err := s.readLocal(); err == nil {
		return local, nil
	}

	return []RecommendedApp{}, nil
}

func (s *RecommendedSource) decodeRecommended(raw []byte) ([]RecommendedApp, error) {
	var payload recommendedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode recommended.json: %w", err)
	}

	prefix := s.mirrorPrefix()
	apps := make([]RecommendedApp, 0, len(payload.Apps))
	for _, app := range payload.Apps {
		if prefix != "" && strings.Contains(app.IconURL, "raw.githubusercontent.com") {
			app.IconURL = prefix + app.IconURL
		}
		apps = append(apps, app)
	}

	return apps, nil
}

func (s *RecommendedSource) writeCache(raw []byte) error {
	if s.cachePath == "" {
		return nil
	}

	cacheDir := filepath.Dir(s.cachePath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir %q: %w", cacheDir, err)
	}

	if err := os.WriteFile(s.cachePath, raw, 0o644); err != nil {
		return fmt.Errorf("write recommended cache %q: %w", s.cachePath, err)
	}

	return nil
}

func (s *RecommendedSource) readCache() ([]RecommendedApp, error) {
	if s.cachePath == "" {
		return nil, errors.New("cache path is empty")
	}

	raw, err := os.ReadFile(s.cachePath)
	if err != nil {
		return nil, fmt.Errorf("read recommended cache %q: %w", s.cachePath, err)
	}

	apps, err := s.decodeRecommended(raw)
	if err != nil {
		return nil, fmt.Errorf("decode cached recommended apps: %w", err)
	}

	return apps, nil
}

func (s *RecommendedSource) readLocal() ([]RecommendedApp, error) {
	if s.localPath == "" {
		return nil, errors.New("local path is empty")
	}

	raw, err := os.ReadFile(s.localPath)
	if err != nil {
		return nil, fmt.Errorf("read local recommended apps %q: %w", s.localPath, err)
	}

	apps, err := s.decodeRecommended(raw)
	if err != nil {
		return nil, fmt.Errorf("decode local recommended apps %q: %w", s.localPath, err)
	}

	_ = s.writeCache(raw)
	return apps, nil
}

func (s *RecommendedSource) mirrorPrefix() string {
	if s.configMgr == nil {
		return config.GitHubMirrorPrefix(config.DefaultMirror, config.Config{})
	}

	cfg := s.configMgr.Get()
	return config.GitHubMirrorPrefix(cfg.Mirror, cfg)
}
