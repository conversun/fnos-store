package main

import (
	"context"
	storeassets "fnos-store"
	"fnos-store/internal/api"
	"fnos-store/internal/core"
	"fnos-store/internal/platform"
	"fnos-store/internal/scheduler"
	"fnos-store/internal/source"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	addr := envOr("LISTEN_ADDR", ":8011")
	projectRoot := envOr("PROJECT_ROOT", findProjectRoot())
	appsDir := envOr("APPS_DIR", defaultAppsDir(projectRoot))
	cachePath := envOr("APPS_CACHE_PATH", filepath.Join(projectRoot, "var", "cache", "apps.json"))
	downloadDir := envOr("DOWNLOAD_DIR", filepath.Join(os.TempDir(), "fnos-store-downloads"))

	ac := platform.NewAppCenter(projectRoot)
	src := source.NewFNOSAppsSource(cachePath)
	reg := core.NewRegistry()
	downloader := core.NewDownloader(downloadDir)
	if err := downloader.CleanupStaleTmpFiles(); err != nil {
		log.Printf("cleanup stale tmp files failed: %v", err)
	}

	srv := api.NewServer(api.Config{
		AppCenter:  ac,
		Source:     src,
		Registry:   reg,
		Downloader: downloader,
		AppsDir:    appsDir,
		Platform:   platform.DetectPlatform(),
		StaticFS:   storeassets.WebFS,
	})

	sched := scheduler.New(6 * time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Start(ctx)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.Mux,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		cancel()
		sched.Stop()
		httpServer.Close()
	}()

	log.Printf("fnos-store listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func findProjectRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	dir := filepath.Dir(exe)
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return "."
}

func defaultAppsDir(projectRoot string) string {
	if _, err := os.Stat("/var/apps"); err == nil {
		return "/var/apps"
	}
	return filepath.Join(projectRoot, "dev", "mock-apps")
}
