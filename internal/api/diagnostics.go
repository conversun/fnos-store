package api

import (
	"net/http"
	"runtime"
	"time"

	"fnos-store/internal/diagnostics"
)

func (s *Server) handleGetAppDiagnostic(w http.ResponseWriter, r *http.Request) {
	app := r.PathValue("appname")
	step := r.URL.Query().Get("step")
	if step == "" {
		writeAPIError(w, http.StatusBadRequest, "step is required")
		return
	}

	displayName := app
	version := ""
	appType := ""
	if info, ok := s.getRegistryApp(app); ok {
		displayName = info.DisplayName
		version = info.FpkVersion
		if version == "" {
			version = info.LatestVersion
		}
		appType = info.AppType
	}

	rawTail, _, _ := fetchAppLogTail(r.Context(), s.appsDir, app, 200, 0)
	tail, truncated := diagnostics.TruncateLogTail(rawTail, diagnostics.MaxLogLines, diagnostics.MaxLogBytes)

	report := diagnostics.DiagnosticReport{
		App:          app,
		DisplayName:  displayName,
		Version:      version,
		Arch:         diagnostics.NormalizeArch(runtime.GOARCH),
		AppType:      appType,
		FailedStep:   step,
		ErrorMessage: diagnostics.TruncateError(r.URL.Query().Get("error")),
		LogTail:      tail,
		LogTruncated: truncated,
		StoreVersion: s.storeVersion(),
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}

	issueURL, err := diagnostics.BuildIssueURL(report)
	if err != nil {
		issueURL = ""
	}

	writeJSON(w, http.StatusOK, diagnosticResponse{Report: report, IssueURL: issueURL})
}
