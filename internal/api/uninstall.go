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
	defer s.queue.FinishApp(appname)

	stream, err := newSSEStream(w, r, appname)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "stopping", Message: "正在停止..."})
	if err := s.queue.WithCLI(func() error { return s.ac.Stop(appname) }); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "uninstalling", Message: "正在卸载..."})
	if err := s.queue.WithCLI(func() error { return s.ac.Uninstall(appname) }); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if s.cacheStore != nil {
		s.cacheStore.RemoveInstalledTag(appname)
	}

	if err := s.refreshRegistry(r.Context()); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "done", Message: "卸载完成"})
}
