package api

import (
	"fmt"
	"net/http"

	"fnos-store/internal/config"
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
			Category:         app.Category,
			PostInstallNote:  app.PostInstallNote,
		})
	}

	writeJSON(w, http.StatusOK, appsListResponse{
		Apps:      respApps,
		LastCheck: formatTimestamp(s.getLastCheck()),
	})
}

func (s *Server) handleDownloadFpk(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("appname")
	if appName == "" {
		writeAPIError(w, http.StatusBadRequest, "missing app name")
		return
	}

	apps := s.listRegistryApps()
	var found *core.AppInfo
	for i := range apps {
		if apps[i].AppName == appName {
			found = &apps[i]
			break
		}
	}
	if found == nil {
		writeAPIError(w, http.StatusNotFound, "app not found")
		return
	}
	if found.DownloadURL == "" {
		writeAPIError(w, http.StatusNotFound, "no download available")
		return
	}

	cfg := s.configMgr.Get()
	prefix := config.GitHubMirrorPrefix(cfg.Mirror, cfg)
	http.Redirect(w, r, prefix+found.DownloadURL, http.StatusFound)
}
