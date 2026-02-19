package api

import (
	"net/http"

	"fnos-store/internal/core"
)

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
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

	s.runInstallLikeOperation(w, r, "install", appname, app)
}

func (s *Server) runInstallLikeOperation(w http.ResponseWriter, r *http.Request, opName, appname string, app core.AppInfo) {
	if !s.queue.TryStart(opName, appname) {
		writeAPIError(w, http.StatusConflict, "another operation is already running")
		return
	}
	defer s.queue.FinishApp(appname)

	stream, err := newSSEStream(w, r, appname)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.pipeline.runStandard(r.Context(), stream, opName, app, s.refreshRegistry)
}

func (s *Server) runSelfUpdate(w http.ResponseWriter, r *http.Request, app core.AppInfo) {
	if !s.queue.TryStartExclusive("update", s.storeApp) {
		writeAPIError(w, http.StatusConflict, "another operation is already running")
		return
	}
	defer s.queue.FinishExclusive(s.storeApp)

	stream, err := newSSEStream(w, r, s.storeApp)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.pipeline.runSelfUpdate(r.Context(), stream, app)
}
