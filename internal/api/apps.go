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
		if s.storeApp != "" && app.AppName == s.storeApp {
			continue
		}
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

		hasUpdate := app.Status == core.AppStatusUpdateAvailable
		availableVersion := ""
		if hasUpdate && app.FpkVersion != "" {
			availableVersion = app.FpkVersion
		}

		respApps = append(respApps, appResponse{
			AppName:          app.AppName,
			DisplayName:      app.DisplayName,
			Description:      app.Description,
			Installed:        app.Installed,
			InstalledVersion: app.InstalledVersion,
			LatestVersion:    app.LatestVersion,
			AvailableVersion: availableVersion,
			HasUpdate:        hasUpdate,
			Platform:         app.Platform,
			ReleaseURL:       releaseURL,
			ReleaseNotes:     "",
			Status:           status,
			ServicePort:      app.ServicePort,
			Homepage:         app.HomepageURL,
			IconURL:          app.IconURL,
			UpdatedAt:        app.UpdatedAt,
			DownloadCount:    app.DownloadCount,
			AppType:          app.AppType,
		})
	}

	writeJSON(w, http.StatusOK, appsListResponse{
		Apps:      respApps,
		LastCheck: formatTimestamp(s.getLastCheck()),
	})
}
