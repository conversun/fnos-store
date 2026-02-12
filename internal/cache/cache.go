package cache

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	mu       sync.RWMutex
	cacheDir string
	meta     metadata
}

type metadata struct {
	LastCheckAt time.Time `json:"last_check_at"`
}

func NewStore(dataDir string) *Store {
	return &Store{
		cacheDir: filepath.Join(dataDir, "cache"),
	}
}

func (s *Store) Init() error {
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.metaPath())
	if err == nil {
		_ = json.Unmarshal(raw, &s.meta)
	}
	return nil
}

func (s *Store) metaPath() string {
	return filepath.Join(s.cacheDir, "meta.json")
}

func (s *Store) LastCheckAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.meta.LastCheckAt
}

func (s *Store) SetLastCheckAt(t time.Time) {
	s.mu.Lock()
	s.meta.LastCheckAt = t
	s.mu.Unlock()

	s.persistMeta()
}

func (s *Store) persistMeta() {
	s.mu.RLock()
	raw, err := json.MarshalIndent(s.meta, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(s.metaPath(), raw, 0o644)
}

// CleanupStaleFiles removes temporary/orphaned cache files on startup.
func (s *Store) CleanupStaleFiles() {
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == "meta.json" || name == "apps.json" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > 7*24*time.Hour {
			path := filepath.Join(s.cacheDir, name)
			if err := os.Remove(path); err == nil {
				log.Printf("cache: cleaned stale file %s", name)
			}
		}
	}
}
