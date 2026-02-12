package api

import (
	"fmt"
	"net/http"
	"time"

	"fnos-store/internal/core"
)

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	apps := s.listRegistryApps()
	respApps := make([]map[string]any, 0, len(apps))
	for _, app := range apps {
		status := ""
		if app.Installed {
			status = s.getRuntimeStatus(app.AppName)
			if status == "" {
				status = "stopped"
			}
		}

		releaseURL := ""
		if app.ReleaseTag != "" {
			releaseURL = fmt.Sprintf("https://github.com/conversun/fnos-apps/releases/tag/%s", app.ReleaseTag)
		}

		respApps = append(respApps, map[string]any{
			"appname":           app.AppName,
			"display_name":      app.DisplayName,
			"installed":         app.Installed,
			"installed_version": app.InstalledVersion,
			"latest_version":    app.LatestVersion,
			"has_update":        app.Status == core.AppStatusUpdateAvailable,
			"platform":          app.Platform,
			"release_url":       releaseURL,
			"release_notes":     "",
			"status":            status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"apps":       respApps,
		"last_check": formatTimestamp(s.getLastCheck()),
	})
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
