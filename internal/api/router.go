package api

import (
	"context"
	"fnos-store/internal/cache"
	"fnos-store/internal/config"
	"fnos-store/internal/core"
	"fnos-store/internal/platform"
	"fnos-store/internal/scheduler"
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
	queue       *OperationQueue
	pipeline    *installPipeline
	configMgr   *config.Manager
	cacheStore  *cache.Store
	scheduler   *scheduler.Scheduler
	appsDir     string
	platform    string
	storeApp    string
	staticFS    fs.FS
	lastCheck   time.Time
	statusByApp map[string]string

	mu               sync.RWMutex
	refreshDebouncer *refreshDebouncer
}

type Config struct {
	AppCenter  platform.AppCenter
	Source     source.Source
	Registry   *core.Registry
	Downloader *core.Downloader
	ConfigMgr  *config.Manager
	CacheStore *cache.Store
	Scheduler  *scheduler.Scheduler
	AppsDir    string
	Platform   string
	StoreApp   string
	StaticFS   fs.FS
}

func NewServer(cfg Config) *Server {
	queue := NewOperationQueue()
	s := &Server{
		Mux:      http.NewServeMux(),
		ac:       cfg.AppCenter,
		source:   cfg.Source,
		registry: cfg.Registry,
		queue:    queue,
		pipeline: &installPipeline{
			downloads:  cfg.Downloader,
			ac:         cfg.AppCenter,
			queue:      queue,
			configMgr:  cfg.ConfigMgr,
			cacheStore: cfg.CacheStore,
		},
		configMgr:        cfg.ConfigMgr,
		cacheStore:       cfg.CacheStore,
		scheduler:        cfg.Scheduler,
		appsDir:          cfg.AppsDir,
		platform:         cfg.Platform,
		storeApp:         cfg.StoreApp,
		staticFS:         cfg.StaticFS,
		statusByApp:      make(map[string]string),
		refreshDebouncer: &refreshDebouncer{},
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
	s.Mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.Mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	s.Mux.HandleFunc("GET /api/store-update", s.handleGetStoreUpdate)
	s.Mux.HandleFunc("POST /api/store-update", s.handlePostStoreUpdate)
	s.Mux.HandleFunc("POST /api/mirrors/check", s.handleCheckMirrors)
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

func (s *Server) SetScheduler(sched *scheduler.Scheduler) {
	s.scheduler = sched
}
