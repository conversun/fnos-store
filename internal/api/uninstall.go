package api

import "net/http"

func (s *Server) handleUninstall(w http.ResponseWriter, r *http.Request) {
	appname := r.PathValue("appname")
	if appname == "" {
		writeAPIError(w, http.StatusBadRequest, "appname is required")
		return
	}

	if !s.queue.TryStart("uninstall", appname) {
		writeAPIError(w, http.StatusConflict, "another operation is already running")
		return
	}
	defer s.queue.Finish()

	_ = s.queue.WithCLI(func() error { return s.ac.Stop(appname) })
	if err := s.queue.WithCLI(func() error { return s.ac.Uninstall(appname) }); err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}

	if err := s.refreshRegistry(r.Context()); err != nil {
		writeAPIError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "appname": appname})
}
