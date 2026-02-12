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
)

// Config holds the persistent store configuration.
type Config struct {
	CheckIntervalHours int `json:"check_interval_hours"`
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

	m.cfg = cfg
	return m.cfg, nil
}

// SaveConfig writes the config to disk.
func (m *Manager) SaveConfig(cfg Config) error {
	if cfg.CheckIntervalHours < 1 {
		cfg.CheckIntervalHours = DefaultCheckIntervalHours
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
