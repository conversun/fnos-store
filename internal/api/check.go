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
	qs := s.queue.Status()
	writeJSON(w, http.StatusOK, statusResponse{
		Status:    "ok",
		Busy:      qs.Busy,
		Operation: qs.Operation,
		AppName:   qs.AppName,
		StartedAt: formatTimestamp(qs.StartedAt),
		LastCheck: formatTimestamp(s.getLastCheck()),
		Platform:  s.platform,
	})
}
