package api

import "net/http"

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	appname := r.PathValue("appname")
	if appname == "" {
		writeAPIError(w, http.StatusBadRequest, "appname is required")
		return
	}

	app, ok := s.getRegistryApp(appname)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "app not found")
		return
	}
	if !app.Installed {
		writeAPIError(w, http.StatusBadRequest, "app is not installed")
		return
	}

	s.runInstallLikeOperation(w, r, "update", appname, app)
}
