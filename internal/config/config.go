package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	MultiRegistry bool // supports proxying multiple registries via path prefix (docker.io, ghcr.io, ...)
	// RegistryRewrites maps an upstream registry host to this mirror's
	// replacement base for that registry (e.g. "ghcr.io" -> "ghcr.nju.edu.cn/").
	// Unlike URL prefixing the registry host is REPLACED, not carried in the
	// path — the shape ghcr-only mirrors use. Empty for prefix-style mirrors.
	RegistryRewrites map[string]string
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
	// ghcr.io has effectively NO public path-prefix proxy: every prefix-style
	// mirror below denies ghcr refs (measured on a live fnOS box — allowlist or
	// manifest failures across 1ms/rat.dev/1panel/dockerproxy/registry.cyou),
	// so ghcr-sourced apps (paperless-ngx #286 and friends) fell through to a
	// direct ghcr.io pull that EOFs behind the GFW. NJU mirrors ghcr by host
	// rewrite (ghcr.io/owner/img -> ghcr.nju.edu.cn/owner/img), verified by a
	// full layer pull on fnOS 1.2.0203. It serves ONLY ghcr.io.
	{Key: "nju-ghcr", Label: "NJU ghcr", Description: "南京大学 ghcr.io 专用镜像（ghcr 应用推荐）", RegistryRewrites: map[string]string{"ghcr.io": "ghcr.nju.edu.cn/"}},
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

// RewriteRefsForMirror returns the refs mirror m serves for canonical by
// rewriting the upstream registry host (ghcr.io/x/y -> ghcr.nju.edu.cn/x/y).
// Empty when m has no rewrite covering the ref's registry.
func RewriteRefsForMirror(m DockerMirror, canonical string) []string {
	var refs []string
	registries := make([]string, 0, len(m.RegistryRewrites))
	for registry := range m.RegistryRewrites {
		registries = append(registries, registry)
	}
	sort.Strings(registries) // deterministic candidate order
	for _, registry := range registries {
		if strings.HasPrefix(canonical, registry+"/") {
			refs = append(refs, m.RegistryRewrites[registry]+canonical[len(registry)+1:])
		}
	}
	return refs
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

// DockerFallbackPrefixes mirrors GitHubFallbackPrefixes for docker pulls: the
// selected mirror first, every other real mirror in declaration order, then a
// direct pull ("") as the last resort. A mirror-level denial (the image is
// not in the mirror's allowlist) must never dead-end an install — this chain
// is what lets the pull fall through to a source that carries the image
// (conversun/fnos-apps#267, #266, #257, #248).
func DockerFallbackPrefixes(selectedKey string, cfg Config) []string {
	prefixes := make([]string, 0, len(dockerMirrors))
	selected := DockerMirrorPrefix(selectedKey, cfg)
	prefixes = append(prefixes, selected)
	for _, m := range dockerMirrors {
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
	AccessTokenHash    string   `json:"access_token_hash,omitempty"`
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

	// Commit on disk before changing live access protection. A failed save
	// must not lock the current browser out with an unpersisted token.
	f, err := os.CreateTemp(filepath.Dir(m.filePath), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), m.filePath); err != nil {
		return err
	}
	m.cfg = cfg
	return nil
}

// Get returns the current in-memory config.
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}
