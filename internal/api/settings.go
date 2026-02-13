package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"fnos-store/internal/config"
)

func mirrorOptionKeys() []string {
	keys := make([]string, 0, len(config.MirrorOptions))
	for k := range config.MirrorOptions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type settingsResponse struct {
	CheckIntervalHours int      `json:"check_interval_hours"`
	Mirror             string   `json:"mirror"`
	MirrorOptions      []string `json:"mirror_options"`
}

type settingsRequest struct {
	CheckIntervalHours int    `json:"check_interval_hours"`
	Mirror             string `json:"mirror"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	if s.configMgr == nil {
		writeAPIError(w, http.StatusInternalServerError, "config not available")
		return
	}

	cfg := s.configMgr.Get()
	writeJSON(w, http.StatusOK, settingsResponse{
		CheckIntervalHours: cfg.CheckIntervalHours,
		Mirror:             cfg.Mirror,
		MirrorOptions:      mirrorOptionKeys(),
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

	cfg := config.Config{
		CheckIntervalHours: req.CheckIntervalHours,
		Mirror:             req.Mirror,
	}

	if err := s.configMgr.SaveConfig(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.scheduler != nil {
		s.scheduler.SetInterval(time.Duration(req.CheckIntervalHours) * time.Hour)
	}

	writeJSON(w, http.StatusOK, settingsResponse{
		CheckIntervalHours: req.CheckIntervalHours,
		Mirror:             req.Mirror,
		MirrorOptions:      mirrorOptionKeys(),
	})
}
