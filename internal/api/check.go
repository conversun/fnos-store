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

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"checked":           len(apps),
		"updates_available": updates,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	queueStatus := s.queue.Status()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"busy":       queueStatus.Busy,
		"operation":  queueStatus.Operation,
		"appname":    queueStatus.AppName,
		"started_at": formatTimestamp(queueStatus.StartedAt),
		"last_check": formatTimestamp(s.getLastCheck()),
		"platform":   s.platform,
	})
}
