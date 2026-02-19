package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"fnos-store/internal/core"
	"fnos-store/internal/platform"
)

type refreshDebouncer struct {
	mu      sync.Mutex
	timer   *time.Timer
	pending bool
}

func (s *Server) refreshRegistry(ctx context.Context) error {
	if s.source == nil || s.registry == nil {
		return errors.New("source/registry not configured")
	}

	remoteApps, err := s.source.FetchApps(ctx)
	if err != nil {
		return err
	}

	localApps, err := core.ScanInstalled(s.appsDir)
	if err != nil {
		return err
	}

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
	return nil
}

func (s *Server) RefreshRegistry(ctx context.Context) error {
	return s.refreshRegistry(ctx)
}

func (s *Server) refreshRuntimeStatus() {
	if s.ac == nil || s.queue == nil {
		return
	}

	var apps []platform.InstalledApp
	err := s.queue.WithCLI(func() error {
		var listErr error
		apps, listErr = s.ac.List()
		return listErr
	})
	if err != nil {
		return
	}

	status := make(map[string]string, len(apps))
	for _, app := range apps {
		status[app.AppName] = app.Status
	}

	s.mu.Lock()
	s.statusByApp = status
	s.mu.Unlock()
}

func (s *Server) listRegistryApps() []core.AppInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.registry == nil {
		return nil
	}
	return s.registry.List()
}

func (s *Server) getRegistryApp(name string) (core.AppInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.registry == nil {
		return core.AppInfo{}, false
	}
	return s.registry.Get(name)
}

func (s *Server) getRuntimeStatus(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusByApp[name]
}

func (s *Server) getLastCheck() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastCheck
}

func (s *Server) refreshRegistryDebounced(ctx context.Context) {
	s.refreshDebouncer.mu.Lock()
	defer s.refreshDebouncer.mu.Unlock()

	if s.refreshDebouncer.pending {
		if s.refreshDebouncer.timer != nil {
			s.refreshDebouncer.timer.Stop()
		}
	}

	s.refreshDebouncer.pending = true
	s.refreshDebouncer.timer = time.AfterFunc(2*time.Second, func() {
		s.refreshDebouncer.mu.Lock()
		s.refreshDebouncer.pending = false
		s.refreshDebouncer.mu.Unlock()

		_ = s.refreshRegistry(context.Background())
	})
}
