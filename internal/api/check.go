package api

import (
	"net/http"

	"fnos-store/internal/core"
)

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	fetchErr := s.refreshRegistry(r.Context())

	apps := s.listRegistryApps()
	if fetchErr != nil && len(apps) == 0 {
		writeAPIError(w, http.StatusBadGateway, fetchErr.Error())
		return
	}

	cfg := s.configMgr.Get()
	updates := 0
	for _, app := range apps {
		if app.Status == core.AppStatusUpdateAvailable && !cfg.IsAppIgnored(app.AppName) {
			updates++
		}
	}

	resp := checkResponse{
		Checked:          len(apps),
		UpdatesAvailable: updates,
	}
	if fetchErr != nil {
		resp.Status = "partial"
	} else {
		resp.Status = "ok"
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	activeOps := s.queue.ActiveOps()

	resp := statusResponse{
		Status:    "ok",
		Busy:      s.queue.IsBusy(),
		LastCheck: formatTimestamp(s.getLastCheck()),
		Platform:  s.platform,
		ActiveOps: activeOps,
	}

	// Backward compat: fill single-operation fields from first active op
	if len(activeOps) > 0 {
		resp.Operation = activeOps[0].Operation
		resp.AppName = activeOps[0].AppName
		resp.StartedAt = formatTimestamp(activeOps[0].StartedAt)
	}

	writeJSON(w, http.StatusOK, resp)
}
