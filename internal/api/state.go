package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"fnos-store/internal/core"
	"fnos-store/internal/platform"
	"fnos-store/internal/source"
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

	localApps, err := core.ScanInstalled(s.appsDir)
	if err != nil {
		return err
	}

	remoteApps, fetchErr := s.source.FetchApps(ctx)

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

	if err := s.refreshRecommended(ctx); err != nil {
		return err
	}

	s.refreshRuntimeStatus()
	return fetchErr
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

func (s *Server) refreshRecommended(ctx context.Context) error {
	if s.recommendedSource == nil {
		return nil
	}

	apps, err := s.recommendedSource.FetchRecommendedApps(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.recommendedApps = append([]source.RecommendedApp(nil), apps...)
	s.mu.Unlock()

	return nil
}

func (s *Server) listRegistryApps() []core.AppInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.registry == nil {
		return nil
	}
	return s.registry.List()
}

func (s *Server) listRecommendedApps() []source.RecommendedApp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]source.RecommendedApp(nil), s.recommendedApps...)
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
