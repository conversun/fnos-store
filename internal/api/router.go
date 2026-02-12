package api

import (
	"context"
	"encoding/json"
	"errors"
	"fnos-store/internal/core"
	"fnos-store/internal/platform"
	"fnos-store/internal/source"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Mux         *http.ServeMux
	ac          platform.AppCenter
	source      source.Source
	registry    *core.Registry
	downloads   *core.Downloader
	queue       *OperationQueue
	appsDir     string
	platform    string
	staticFS    fs.FS
	lastCheck   time.Time
	statusByApp map[string]string

	mu sync.RWMutex
}

type Config struct {
	AppCenter  platform.AppCenter
	Source     source.Source
	Registry   *core.Registry
	Downloader *core.Downloader
	AppsDir    string
	Platform   string
	StaticFS   fs.FS
}

func NewServer(cfg Config) *Server {
	s := &Server{
		Mux:         http.NewServeMux(),
		ac:          cfg.AppCenter,
		source:      cfg.Source,
		registry:    cfg.Registry,
		downloads:   cfg.Downloader,
		queue:       NewOperationQueue(),
		appsDir:     cfg.AppsDir,
		platform:    cfg.Platform,
		staticFS:    cfg.StaticFS,
		statusByApp: make(map[string]string),
	}
	s.routes()
	_ = s.refreshRegistry(context.Background())
	return s
}

func (s *Server) routes() {
	s.Mux.HandleFunc("GET /api/apps", s.handleListApps)
	s.Mux.HandleFunc("POST /api/apps/{appname}/install", s.handleInstall)
	s.Mux.HandleFunc("POST /api/apps/{appname}/update", s.handleUpdate)
	s.Mux.HandleFunc("POST /api/apps/{appname}/uninstall", s.handleUninstall)
	s.Mux.HandleFunc("POST /api/check", s.handleCheck)
	s.Mux.HandleFunc("GET /api/status", s.handleStatus)
	s.Mux.HandleFunc("/", s.handleSPA)
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	if s.staticFS == nil {
		http.NotFound(w, r)
		return
	}

	if r.URL.Path == "/" {
		http.ServeFileFS(w, r, s.staticFS, "web/index.html")
		return
	}

	assetPath := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if assetPath == "." {
		http.ServeFileFS(w, r, s.staticFS, "web/index.html")
		return
	}

	fullPath := path.Join("web", assetPath)
	if _, err := fs.Stat(s.staticFS, fullPath); err == nil {
		http.ServeFileFS(w, r, s.staticFS, fullPath)
		return
	}

	http.ServeFileFS(w, r, s.staticFS, "web/index.html")
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

	s.mu.Lock()
	s.registry.Merge(localApps, remoteApps)
	s.lastCheck = time.Now()
	s.mu.Unlock()

	s.refreshRuntimeStatus()
	return nil
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
