package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"fnos-store/internal/platform"
)

const (
	defaultAppsJSONURL = "https://raw.githubusercontent.com/conversun/fnos-apps/main/apps.json"
	githubReleaseBase  = "https://github.com/conversun/fnos-apps/releases/download"
	ghfastPrefix       = "https://ghfast.top/"
)

type FNOSAppsSource struct {
	httpClient *http.Client
	appsURL    string
	cachePath  string
	platform   string
	name       string
}

type appsJSONPayload struct {
	Apps []appsJSONEntry `json:"apps"`
}

type appsJSONEntry struct {
	AppName     string   `json:"appname"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	FpkVersion  string   `json:"fpk_version"`
	ReleaseTag  string   `json:"release_tag"`
	FilePrefix  string   `json:"file_prefix"`
	ServicePort int      `json:"service_port"`
	IconURL     string   `json:"icon_url"`
	Platforms   []string `json:"platforms"`
}

func NewFNOSAppsSource(cachePath string) *FNOSAppsSource {
	return NewFNOSAppsSourceWithEndpoint(defaultAppsJSONURL, cachePath)
}

func NewFNOSAppsSourceWithEndpoint(appsURL, cachePath string) *FNOSAppsSource {
	if appsURL == "" {
		appsURL = defaultAppsJSONURL
	}

	return &FNOSAppsSource{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		appsURL:    appsURL,
		cachePath:  cachePath,
		platform:   platform.DetectPlatform(),
		name:       "fnos-apps",
	}
}

func (s *FNOSAppsSource) Name() string {
	if s.name == "" {
		return "fnos-apps"
	}
	return s.name
}

func (s *FNOSAppsSource) FetchApps(ctx context.Context) ([]RemoteApp, error) {
	apps, raw, err := s.fetchRemote(ctx)
	if err == nil {
		_ = s.writeCache(raw)
		return apps, nil
	}

	cached, cacheErr := s.readCache()
	if cacheErr == nil {
		return cached, nil
	}

	return nil, fmt.Errorf("fetch apps from remote failed: %w", err)
}

func (s *FNOSAppsSource) fetchRemote(ctx context.Context) ([]RemoteApp, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.appsURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build apps.json request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("apps.json http status: %s", resp.Status)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read apps.json response: %w", err)
	}

	apps, err := s.decodeApps(raw)
	if err != nil {
		return nil, nil, err
	}

	return apps, raw, nil
}

func (s *FNOSAppsSource) decodeApps(raw []byte) ([]RemoteApp, error) {
	var payload appsJSONPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode apps.json: %w", err)
	}

	apps := make([]RemoteApp, 0, len(payload.Apps))
	for _, item := range payload.Apps {
		if !s.supportsPlatform(item.Platforms) {
			continue
		}

		directURL := fmt.Sprintf(
			"%s/%s/%s_%s_%s.fpk",
			githubReleaseBase,
			item.ReleaseTag,
			item.FilePrefix,
			item.FpkVersion,
			s.platform,
		)

		apps = append(apps, RemoteApp{
			AppName:     item.AppName,
			DisplayName: item.DisplayName,
			Version:     item.Version,
			Description: item.Description,
			ReleaseTag:  item.ReleaseTag,
			FilePrefix:  item.FilePrefix,
			FpkVersion:  item.FpkVersion,
			ServicePort: item.ServicePort,
			Platforms:   item.Platforms,
			FpkURL:      directURL,
			MirrorURL:   ghfastPrefix + directURL,
			IconURL:     item.IconURL,
			Source:      s.Name(),
		})
	}

	return apps, nil
}

func (s *FNOSAppsSource) supportsPlatform(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	return slices.Contains(platforms, s.platform)
}

func (s *FNOSAppsSource) writeCache(raw []byte) error {
	if s.cachePath == "" {
		return nil
	}

	cacheDir := filepath.Dir(s.cachePath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir %q: %w", cacheDir, err)
	}

	if err := os.WriteFile(s.cachePath, raw, 0o644); err != nil {
		return fmt.Errorf("write apps cache %q: %w", s.cachePath, err)
	}

	return nil
}

func (s *FNOSAppsSource) readCache() ([]RemoteApp, error) {
	if s.cachePath == "" {
		return nil, errors.New("cache path is empty")
	}

	raw, err := os.ReadFile(s.cachePath)
	if err != nil {
		return nil, fmt.Errorf("read apps cache %q: %w", s.cachePath, err)
	}

	apps, err := s.decodeApps(raw)
	if err != nil {
		return nil, fmt.Errorf("decode cached apps: %w", err)
	}

	return apps, nil
}
