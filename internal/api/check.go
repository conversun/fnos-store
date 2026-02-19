package api

import (
	"net/http"

	"fnos-store/internal/core"
)

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if err := s.refreshRegistry(r.Context()); err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}

	apps := s.listRegistryApps()
	updates := 0
	for _, app := range apps {
		if app.Status == core.AppStatusUpdateAvailable {
			updates++
		}
	}

	writeJSON(w, http.StatusOK, checkResponse{
		Status:           "ok",
		Checked:          len(apps),
		UpdatesAvailable: updates,
	})
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
