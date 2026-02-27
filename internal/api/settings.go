package api

import (
	"encoding/json"
	"net/http"
	"time"

	"fnos-store/internal/config"
)

type mirrorOptionResponse struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type settingsResponse struct {
	CheckIntervalHours  int                    `json:"check_interval_hours"`
	Mirror              string                 `json:"mirror"`
	MirrorOptions       []mirrorOptionResponse `json:"mirror_options"`
	DockerMirror        string                 `json:"docker_mirror"`
	DockerMirrorOptions []mirrorOptionResponse `json:"docker_mirror_options"`
	CustomGitHubMirror  string                 `json:"custom_github_mirror,omitempty"`
	CustomDockerMirror  string                 `json:"custom_docker_mirror,omitempty"`
}

type settingsRequest struct {
	CheckIntervalHours int    `json:"check_interval_hours"`
	Mirror             string `json:"mirror"`
	DockerMirror       string `json:"docker_mirror"`
	CustomGitHubMirror string `json:"custom_github_mirror"`
	CustomDockerMirror string `json:"custom_docker_mirror"`
}

func githubMirrorOptionsResponse() []mirrorOptionResponse {
	mirrors := config.GitHubMirrorOptions()
	opts := make([]mirrorOptionResponse, len(mirrors))
	for i, m := range mirrors {
		opts[i] = mirrorOptionResponse{Key: m.Key, Label: m.Label, Description: m.Description}
	}
	return opts
}

func dockerMirrorOptionsResponse() []mirrorOptionResponse {
	mirrors := config.DockerMirrorOptions()
	opts := make([]mirrorOptionResponse, len(mirrors))
	for i, m := range mirrors {
		opts[i] = mirrorOptionResponse{Key: m.Key, Label: m.Label, Description: m.Description}
	}
	return opts
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	if s.configMgr == nil {
		writeAPIError(w, http.StatusInternalServerError, "config not available")
		return
	}

	cfg := s.configMgr.Get()
	writeJSON(w, http.StatusOK, settingsResponse{
		CheckIntervalHours:  cfg.CheckIntervalHours,
		Mirror:              cfg.Mirror,
		MirrorOptions:       githubMirrorOptionsResponse(),
		DockerMirror:        cfg.DockerMirror,
		DockerMirrorOptions: dockerMirrorOptionsResponse(),
		CustomGitHubMirror:  cfg.CustomGitHubMirror,
		CustomDockerMirror:  cfg.CustomDockerMirror,
	})
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	if s.configMgr == nil {
		writeAPIError(w, http.StatusInternalServerError, "config not available")
		return
	}

	var req settingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.CheckIntervalHours < 1 {
		req.CheckIntervalHours = config.DefaultCheckIntervalHours
	}

	if req.Mirror == "" {
		req.Mirror = config.DefaultMirror
	}
	if req.DockerMirror == "" {
		req.DockerMirror = config.DefaultDockerMirror
	}

	cfg := config.Config{
		CheckIntervalHours: req.CheckIntervalHours,
		Mirror:             req.Mirror,
		DockerMirror:       req.DockerMirror,
		CustomGitHubMirror: req.CustomGitHubMirror,
		CustomDockerMirror: req.CustomDockerMirror,
	}

	if err := s.configMgr.SaveConfig(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.scheduler != nil {
		s.scheduler.SetInterval(time.Duration(req.CheckIntervalHours) * time.Hour)
	}

	writeJSON(w, http.StatusOK, settingsResponse{
		CheckIntervalHours:  req.CheckIntervalHours,
		Mirror:              req.Mirror,
		MirrorOptions:       githubMirrorOptionsResponse(),
		DockerMirror:        req.DockerMirror,
		DockerMirrorOptions: dockerMirrorOptionsResponse(),
		CustomGitHubMirror:  req.CustomGitHubMirror,
		CustomDockerMirror:  req.CustomDockerMirror,
	})
}
