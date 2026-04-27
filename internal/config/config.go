package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const (
	DefaultCheckIntervalHours = 6
	DefaultDataDir            = "/var/apps/fnos-apps-store/var"
	DefaultMirror             = "gh-proxy"
	DefaultDockerMirror       = "daocloud"
)

type GitHubMirror struct {
	Key         string
	Label       string
	URL         string
	Description string
}

type DockerMirror struct {
	Key           string
	Label         string
	URL           string
	Description   string
	MultiRegistry bool // supports proxying multiple registries (docker.io, ghcr.io, lscr.io, etc.)
}

var gitHubMirrors = []GitHubMirror{
	{Key: "conversun", Label: "Conversun Hub", URL: "https://hub.conversun.com/", Description: "Conversun 自建 GitHub 加速，仅代理 conversun 仓库"},
	{Key: "gh-proxy", Label: "GH-Proxy", URL: "https://gh-proxy.com/", Description: "公共 GitHub 文件代理，长期稳定运营"},
	{Key: "ghfast", Label: "GHFast", URL: "https://ghfast.top/", Description: "高速 GitHub 文件加速"},
	{Key: "ghproxy-net", Label: "GHProxy.net", URL: "https://ghproxy.net/", Description: "社区维护的 GitHub 加速代理"},
	{Key: "cdn-ghproxy", Label: "CDN GHProxy", URL: "https://cdn.gh-proxy.org/", Description: "CDN 节点 GitHub 加速"},
	{Key: "cors-isteed", Label: "Cors Proxy", URL: "https://cors.isteed.cc/", Description: "Cloudflare Workers GitHub 代理"},
	{Key: "gh-ddlc", Label: "GH DDLC", URL: "https://gh.ddlc.top/", Description: "GitHub 文件下载加速"},
	{Key: "custom", Label: "自定义", URL: "", Description: "使用自定义加速地址"},
	{Key: "direct", Label: "直连 GitHub", URL: "", Description: "直接从 GitHub 下载，适合有代理的用户"},
}

var dockerMirrors = []DockerMirror{
	{Key: "daocloud", Label: "DaoCloud", URL: "m.daocloud.io/", Description: "DaoCloud 公共 Docker 镜像加速", MultiRegistry: true},
	{Key: "docker-1ms", Label: "1ms.run", URL: "docker.1ms.run/", Description: "社区 Docker 镜像加速"},
	{Key: "daocloud-docker", Label: "DaoCloud Docker", URL: "docker.m.daocloud.io/", Description: "DaoCloud Docker 镜像加速（全球可用）"},
	{Key: "ratdev", Label: "Rat.Dev", URL: "hub.rat.dev/", Description: "Rat 社区 Docker 镜像加速"},
	{Key: "1panel", Label: "1Panel", URL: "docker.1panel.live/", Description: "1Panel 官方 Docker 镜像加速（仅限国内）"},
	{Key: "dockerproxy", Label: "DockerProxy", URL: "dockerproxy.net/", Description: "Docker Proxy 社区镜像加速（仅限国内）"},
	{Key: "registry-cyou", Label: "Registry.cyou", URL: "registry.cyou/", Description: "Cloudflare Docker 镜像代理（仅限国内）"},
	{Key: "custom", Label: "自定义", URL: "", Description: "使用自定义加速地址"},
	{Key: "direct", Label: "直连 Docker Hub", URL: "", Description: "直接拉取，适合有代理的用户"},
}

func GitHubMirrorOptions() []GitHubMirror { return gitHubMirrors }
func DockerMirrorOptions() []DockerMirror { return dockerMirrors }

func GitHubMirrorPrefix(key string, cfg Config) string {
	if key == "custom" && cfg.CustomGitHubMirror != "" {
		return cfg.CustomGitHubMirror
	}
	for _, m := range gitHubMirrors {
		if m.Key == key {
			return m.URL
		}
	}
	return gitHubMirrors[0].URL
}

func DockerMirrorPrefix(key string, cfg Config) string {
	if key == "custom" && cfg.CustomDockerMirror != "" {
		return cfg.CustomDockerMirror
	}
	for _, m := range dockerMirrors {
		if m.Key == key {
			return m.URL
		}
	}
	return dockerMirrors[0].URL
}

func IsDockerMirrorMultiRegistry(key string) bool {
	if key == "custom" || key == "direct" || key == "" {
		return false
	}
	for _, m := range dockerMirrors {
		if m.Key == key {
			return m.MultiRegistry
		}
	}
	return false
}

func GitHubFallbackPrefixes(selectedKey string, cfg Config) []string {
	prefixes := make([]string, 0, len(gitHubMirrors))
	selected := GitHubMirrorPrefix(selectedKey, cfg)
	prefixes = append(prefixes, selected)
	for _, m := range gitHubMirrors {
		if m.Key != selectedKey && m.Key != "direct" && m.Key != "custom" && m.URL != "" && m.URL != selected {
			prefixes = append(prefixes, m.URL)
		}
	}
	if selected != "" {
		prefixes = append(prefixes, "")
	}
	return prefixes
}

// Config holds the persistent store configuration.
type Config struct {
	CheckIntervalHours int      `json:"check_interval_hours"`
	Mirror             string   `json:"mirror"`
	DockerMirror       string   `json:"docker_mirror"`
	CustomGitHubMirror string   `json:"custom_github_mirror,omitempty"`
	CustomDockerMirror string   `json:"custom_docker_mirror,omitempty"`
	InstallVolume      int      `json:"install_volume"`
	IgnoredApps        []string `json:"ignored_apps,omitempty"`
}

// IsAppIgnored returns true if the given app is in the ignored list.
func (c Config) IsAppIgnored(appName string) bool {
	for _, name := range c.IgnoredApps {
		if name == appName {
			return true
		}
	}
	return false
}

// Manager handles loading and saving config to disk.
type Manager struct {
	mu       sync.RWMutex
	cfg      Config
	filePath string
}

// NewManager creates a config manager for the given data directory.
// If dataDir is empty, DefaultDataDir is used.
func NewManager(dataDir string) *Manager {
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	return &Manager{
		filePath: filepath.Join(dataDir, "config.json"),
		cfg:      defaultConfig(),
	}
}

func defaultConfig() Config {
	return Config{
		CheckIntervalHours: DefaultCheckIntervalHours,
		Mirror:             DefaultMirror,
		DockerMirror:       DefaultDockerMirror,
	}
}

// LoadConfig reads the config file from disk.
// If the file does not exist, defaults are used.
func (m *Manager) LoadConfig() (Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	raw, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			m.cfg = defaultConfig()
			return m.cfg, nil
		}
		return m.cfg, err
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return m.cfg, err
	}

	if cfg.CheckIntervalHours < 1 {
		cfg.CheckIntervalHours = DefaultCheckIntervalHours
	}
	if cfg.Mirror == "" {
		cfg.Mirror = DefaultMirror
	}
	if cfg.DockerMirror == "" {
		cfg.DockerMirror = DefaultDockerMirror
	}

	m.cfg = cfg
	return m.cfg, nil
}

// SaveConfig writes the config to disk.
func (m *Manager) SaveConfig(cfg Config) error {
	if cfg.CheckIntervalHours < 1 {
		cfg.CheckIntervalHours = DefaultCheckIntervalHours
	}
	if cfg.Mirror == "" {
		cfg.Mirror = DefaultMirror
	}
	if cfg.DockerMirror == "" {
		cfg.DockerMirror = DefaultDockerMirror
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(m.filePath), 0o755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	m.cfg = cfg
	return os.WriteFile(m.filePath, raw, 0o644)
}

// Get returns the current in-memory config.
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}
