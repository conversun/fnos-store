package api

import (
	"fmt"
	"net/http"

	"fnos-store/internal/core"
)

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	apps := s.listRegistryApps()
	respApps := make([]appResponse, 0, len(apps))
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

		respApps = append(respApps, appResponse{
			AppName:          app.AppName,
			DisplayName:      app.DisplayName,
			Installed:        app.Installed,
			InstalledVersion: app.InstalledVersion,
			LatestVersion:    app.LatestVersion,
			HasUpdate:        app.Status == core.AppStatusUpdateAvailable,
			Platform:         app.Platform,
			ReleaseURL:       releaseURL,
			ReleaseNotes:     "",
			Status:           status,
			IconURL:          app.IconURL,
		})
	}

	writeJSON(w, http.StatusOK, appsListResponse{
		Apps:      respApps,
		LastCheck: formatTimestamp(s.getLastCheck()),
	})
}
