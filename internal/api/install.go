package api

import (
	"encoding/json"
	"net/http"

	"fnos-store/internal/core"
	"fnos-store/internal/platform"
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
	// Reject an already-installed app. install-local treats this as an UPGRADE:
	// it would uninstall the existing copy before reinstalling, yet the "install"
	// operation name skips the update-only volume pin in runStandard(), so the
	// destructive step would run without the guard that protects existing data.
	// Callers that mean to upgrade must use /update, which pins the app's current
	// volume and fails closed when it cannot.
	if app.Installed {
		writeAPIError(w, http.StatusBadRequest, "应用已安装，请使用更新功能")
		return
	}

	// Wizard answers ride along as a query param so the SSE POST body stays
	// free; the browser sends them from the form rendered off /wizard.
	s.runInstallLikeOperation(w, r, "install", appname, app, parseWizardParams(r))
}

// parseWizardParams reads the user's install-wizard answers from the request.
// Absent or malformed input yields no params, which installs with defaults —
// the behavior before wizards were supported.
func parseWizardParams(r *http.Request) []platform.WizardParam {
	raw := r.URL.Query().Get("wizard")
	if raw == "" {
		return nil
	}
	var params []platform.WizardParam
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return nil
	}
	return params
}

func (s *Server) runInstallLikeOperation(w http.ResponseWriter, r *http.Request, opName, appname string, app core.AppInfo, params []platform.WizardParam) {
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

	s.pipeline.runStandard(r.Context(), stream, opName, app, params, s.refreshRegistry)
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
