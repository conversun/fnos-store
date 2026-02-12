package api

import (
	"encoding/json"
	"net/http"
	"time"

	"fnos-store/internal/config"
)

type settingsResponse struct {
	CheckIntervalHours int `json:"check_interval_hours"`
}

type settingsRequest struct {
	CheckIntervalHours int `json:"check_interval_hours"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	if s.configMgr == nil {
		writeAPIError(w, http.StatusInternalServerError, "config not available")
		return
	}

	cfg := s.configMgr.Get()
	writeJSON(w, http.StatusOK, settingsResponse{
		CheckIntervalHours: cfg.CheckIntervalHours,
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

	cfg := config.Config{
		CheckIntervalHours: req.CheckIntervalHours,
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
	})
}
