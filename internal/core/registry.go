package core

import (
	"fnos-store/internal/source"
	"sort"
	"strings"
	"time"
)

type AppStatus string

const (
	AppStatusNotInstalled      AppStatus = "not_installed"
	AppStatusInstalledUpToDate AppStatus = "installed_up_to_date"
	AppStatusUpdateAvailable   AppStatus = "update_available"
)

type AppInfo struct {
	AppName           string
	DisplayName       string
	Description       string
	HomepageURL       string
	UpdatedAt         string
	ServicePort       int
	Platform          string
	Source            string
	IconURL           string
	Installed         bool
	InstalledVersion  string
	LatestVersion     string
	ReleaseTag        string
	FpkVersion        string
	DownloadURL       string
	MirrorURL         string
	DownloadCount     int
	Status            AppStatus
	HasRevisionUpdate bool
}

type Registry struct {
	apps       map[string]AppInfo
	updatedAt  time.Time
	lastResult []AppInfo
}

func NewRegistry() *Registry {
	return &Registry{
		apps: make(map[string]AppInfo),
	}
}

func (r *Registry) Merge(local []Manifest, remote []source.RemoteApp, installedTags map[string]string) []AppInfo {
	localByName := make(map[string]Manifest, len(local))
	for _, item := range local {
		localByName[item.AppName] = item
	}

	r.apps = make(map[string]AppInfo, len(remote))
	result := make([]AppInfo, 0, len(remote))
	for _, item := range remote {
		localManifest, installed := localByName[item.AppName]
		app := AppInfo{
			AppName:       item.AppName,
			DisplayName:   item.DisplayName,
			Description:   item.Description,
			HomepageURL:   item.HomepageURL,
			UpdatedAt:     item.UpdatedAt,
			ServicePort:   item.ServicePort,
			Platform:      strings.Join(item.Platforms, ","),
			Source:        item.Source,
			IconURL:       item.IconURL,
			Installed:     installed,
			LatestVersion: item.Version,
			ReleaseTag:    item.ReleaseTag,
			FpkVersion:    item.FpkVersion,
			DownloadURL:   item.FpkURL,
			MirrorURL:     item.MirrorURL,
			DownloadCount: item.DownloadCount,
			Status:        AppStatusNotInstalled,
		}

		if installed {
			app.InstalledVersion = localManifest.Version
			if app.ServicePort == 0 {
				app.ServicePort = localManifest.ServicePort
			}
			if app.Platform == "" {
				app.Platform = localManifest.Platform
			}

			versionCmp := CompareVersions(localManifest.Version, item.Version)
			installedTag := installedTags[item.AppName]
			revisionUpdate := versionCmp == 0 && installedTag != item.ReleaseTag && hasRevisionUpdate(item.ReleaseTag, localManifest.Version)
			if versionCmp < 0 || revisionUpdate {
				app.Status = AppStatusUpdateAvailable
			} else {
				app.Status = AppStatusInstalledUpToDate
			}
			app.HasRevisionUpdate = revisionUpdate
		}

		r.apps[app.AppName] = app
		result = append(result, app)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].UpdatedAt != result[j].UpdatedAt {
			return result[i].UpdatedAt > result[j].UpdatedAt
		}
		return result[i].DisplayName < result[j].DisplayName
	})

	r.updatedAt = time.Now()
	r.lastResult = result
	return result
}

func (r *Registry) List() []AppInfo {
	out := make([]AppInfo, len(r.lastResult))
	copy(out, r.lastResult)
	return out
}

func (r *Registry) Get(appname string) (AppInfo, bool) {
	app, ok := r.apps[appname]
	return app, ok
}

func hasRevisionUpdate(releaseTag, installedVersion string) bool {
	prefix, ok := releaseTagPrefix(releaseTag)
	if !ok {
		return false
	}
	expectedTag := prefix + "/v" + installedVersion
	return releaseTag != expectedTag
}

func releaseTagPrefix(releaseTag string) (string, bool) {
	idx := strings.Index(releaseTag, "/v")
	if idx <= 0 {
		return "", false
	}
	return releaseTag[:idx], true
}
