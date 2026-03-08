package api

import (
	"fmt"
	"net/http"
	"time"

	"fnos-store/internal/config"
	"fnos-store/internal/core"
	"fnos-store/internal/source"
)

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	apps := s.listRegistryApps()
	cfg := s.configMgr.Get()
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
		updateIgnored := false
		if hasUpdate && cfg.IsAppIgnored(app.AppName) {
			hasUpdate = false
			updateIgnored = true
		}

		availableVersion := ""
		if (hasUpdate || updateIgnored) && app.FpkVersion != "" {
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
			UpdateIgnored:    updateIgnored,
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

func (s *Server) handleIgnoreUpdate(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("appname")
	if appName == "" {
		writeAPIError(w, http.StatusBadRequest, "missing app name")
		return
	}

	cfg := s.configMgr.Get()
	if cfg.IsAppIgnored(appName) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	cfg.IgnoredApps = append(cfg.IgnoredApps, appName)
	if err := s.configMgr.SaveConfig(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUnignoreUpdate(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("appname")
	if appName == "" {
		writeAPIError(w, http.StatusBadRequest, "missing app name")
		return
	}

	cfg := s.configMgr.Get()
	filtered := make([]string, 0, len(cfg.IgnoredApps))
	for _, name := range cfg.IgnoredApps {
		if name != appName {
			filtered = append(filtered, name)
		}
	}
	cfg.IgnoredApps = filtered

	if err := s.configMgr.SaveConfig(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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

func (s *Server) handleReloadApps(w http.ResponseWriter, r *http.Request) {
	stream, err := newSSEStream(w, r, "")
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	src, ok := s.source.(*source.FNOSAppsSource)
	if !ok {
		_ = stream.sendError("unsupported source type")
		return
	}

	onProgress := func(p source.FetchProgress) {
		var msg string
		switch p.Status {
		case "trying":
			msg = fmt.Sprintf("正在使用 %s 加速...", p.Mirror)
		case "failed":
			msg = fmt.Sprintf("%s 连接失败", p.Mirror)
		case "success":
			msg = fmt.Sprintf("通过 %s 加载成功", p.Mirror)
		}
		_ = stream.sendProgress(progressPayload{Step: p.Status, Message: msg})
	}

	remoteApps, fetchErr := src.FetchAppsWithProgress(r.Context(), onProgress)
	if fetchErr != nil {
		_ = stream.sendProgress(progressPayload{
			Step:    "error",
			Message: "所有加速节点均无法连接，请更换加速节点或检查网络",
		})
		return
	}

	localApps, _ := core.ScanInstalled(s.appsDir)
	var installedTags map[string]string
	if s.cacheStore != nil {
		installedTags = s.cacheStore.InstalledTags()
	}

	now := time.Now()
	s.mu.Lock()
	s.registry.Merge(localApps, remoteApps, installedTags)
	s.lastCheck = now
	s.mu.Unlock()

	if s.cacheStore != nil {
		s.cacheStore.SetLastCheckAt(now)
	}
	s.refreshRuntimeStatus()

	apps := s.listRegistryApps()
	_ = stream.sendProgress(progressPayload{
		Step:    "done",
		Message: fmt.Sprintf("加载完成，共 %d 款应用", len(apps)),
	})
}
