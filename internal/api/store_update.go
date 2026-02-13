package api

import (
	"net/http"

	"fnos-store/internal/core"
)

func (s *Server) handleGetStoreUpdate(w http.ResponseWriter, _ *http.Request) {
	if s.storeApp == "" {
		writeAPIError(w, http.StatusNotFound, "store app not configured")
		return
	}

	app, ok := s.getRegistryApp(s.storeApp)
	if !ok {
		writeJSON(w, http.StatusOK, storeUpdateResponse{
			CurrentVersion: s.storeVersion(),
		})
		return
	}

	hasUpdate := app.Status == core.AppStatusUpdateAvailable
	available := ""
	if hasUpdate {
		available = app.FpkVersion
		if available == "" {
			available = app.LatestVersion
		}
	}

	writeJSON(w, http.StatusOK, storeUpdateResponse{
		CurrentVersion:   s.storeVersion(),
		AvailableVersion: available,
		HasUpdate:        hasUpdate,
	})
}

func (s *Server) handlePostStoreUpdate(w http.ResponseWriter, r *http.Request) {
	if s.storeApp == "" {
		writeAPIError(w, http.StatusNotFound, "store app not configured")
		return
	}

	app, ok := s.getRegistryApp(s.storeApp)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "store app not found in registry")
		return
	}

	s.runSelfUpdate(w, r, app)
}

func (s *Server) storeVersion() string {
	app, ok := s.getRegistryApp(s.storeApp)
	if ok && app.InstalledVersion != "" {
		return app.InstalledVersion
	}
	return ""
}
