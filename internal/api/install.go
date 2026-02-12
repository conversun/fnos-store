package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"

	"fnos-store/internal/core"
)

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	appname := r.PathValue("appname")
	if appname == "" {
		writeAPIError(w, http.StatusBadRequest, "appname is required")
		return
	}

	app, ok := s.getRegistryApp(appname)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "app not found")
		return
	}

	s.runInstallLikeOperation(w, r, "install", appname, app)
}

func (s *Server) runInstallLikeOperation(w http.ResponseWriter, r *http.Request, opName, appname string, app core.AppInfo) {
	if !s.queue.TryStart(opName, appname) {
		writeAPIError(w, http.StatusConflict, "another operation is already running")
		return
	}
	defer s.queue.Finish()

	stream, err := newSSEStream(w, r)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if s.downloads == nil {
		_ = stream.sendError("downloader not configured")
		return
	}

	fileName := path.Base(app.DownloadURL)
	if fileName == "." || fileName == "/" || fileName == "" {
		_ = stream.sendError("invalid download url")
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "downloading", Progress: 0})
	fpkPath, err := s.downloads.Download(r.Context(), core.DownloadRequest{
		MirrorURL: app.MirrorURL,
		DirectURL: app.DownloadURL,
		FileName:  fileName,
	}, func(downloaded, total int64) {
		if total <= 0 {
			return
		}
		progress := int(float64(downloaded) * 100 / float64(total))
		if progress > 100 {
			progress = 100
		}
		_ = stream.sendProgress(progressPayload{Step: "downloading", Progress: progress})
	})
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}
	defer os.Remove(fpkPath)

	_ = stream.sendProgress(progressPayload{Step: "installing", Message: "appcenter-cli install-fpk"})
	volume := 0
	err = s.queue.WithCLI(func() error {
		var volumeErr error
		volume, volumeErr = s.ac.DefaultVolume()
		return volumeErr
	})
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if err := s.queue.WithCLI(func() error { return s.ac.InstallFpk(fpkPath, volume) }); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "verifying", Message: "checking install status"})
	installed := false
	err = s.queue.WithCLI(func() error {
		var checkErr error
		installed, checkErr = s.ac.Check(appname)
		return checkErr
	})
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}
	if !installed {
		_ = stream.sendError("app not installed after operation")
		return
	}

	if err := s.refreshRegistry(r.Context()); err != nil && !errors.Is(err, context.Canceled) {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "done", NewVersion: app.LatestVersion, Message: fmt.Sprintf("%s completed", opName)})
}
