package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"fnos-store/internal/core"
	"fnos-store/internal/platform"
)

type installPipeline struct {
	downloads *core.Downloader
	ac        platform.AppCenter
	queue     *OperationQueue
}

// Caller must os.Remove the returned file.
func (p *installPipeline) downloadFpk(ctx context.Context, stream *sseStream, app core.AppInfo) (string, error) {
	if p.downloads == nil {
		return "", errors.New("downloader not configured")
	}

	fileName := path.Base(app.DownloadURL)
	if fileName == "." || fileName == "/" || fileName == "" {
		return "", errors.New("invalid download url")
	}

	_ = stream.sendProgress(progressPayload{Step: "downloading", Progress: 0})

	fpkPath, err := p.downloads.Download(ctx, core.DownloadRequest{
		MirrorURL: app.MirrorURL,
		DirectURL: app.DownloadURL,
		FileName:  fileName,
	}, func(downloaded, total int64) {
		if total <= 0 {
			return
		}
		pct := int(float64(downloaded) * 100 / float64(total))
		if pct > 100 {
			pct = 100
		}
		_ = stream.sendProgress(progressPayload{Step: "downloading", Progress: pct})
	})

	return fpkPath, err
}

func (p *installPipeline) resolveVolume() (int, error) {
	var volume int
	err := p.queue.WithCLI(func() error {
		var e error
		volume, e = p.ac.DefaultVolume()
		return e
	})
	return volume, err
}

func (p *installPipeline) installFpk(fpkPath string, volume int) error {
	return p.queue.WithCLI(func() error {
		return p.ac.InstallFpk(fpkPath, volume)
	})
}

func (p *installPipeline) verifyInstalled(appname string) error {
	var installed bool
	err := p.queue.WithCLI(func() error {
		var e error
		installed, e = p.ac.Check(appname)
		return e
	})
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("app not installed after operation")
	}
	return nil
}

func (p *installPipeline) runStandard(ctx context.Context, stream *sseStream, opName string, app core.AppInfo, refreshFn func(context.Context) error) {
	fpkPath, err := p.downloadFpk(ctx, stream, app)
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}
	defer os.Remove(fpkPath)

	_ = stream.sendProgress(progressPayload{Step: "installing", Message: "appcenter-cli install-fpk"})

	volume, err := p.resolveVolume()
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if err := p.installFpk(fpkPath, volume); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "verifying", Message: "checking install status"})

	if err := p.verifyInstalled(app.AppName); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if err := refreshFn(ctx); err != nil && !errors.Is(err, context.Canceled) {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "done", NewVersion: app.LatestVersion, Message: fmt.Sprintf("%s completed", opName)})
}

// fnOS kills this process during install-fpk; no verification afterwards.
func (p *installPipeline) runSelfUpdate(ctx context.Context, stream *sseStream, app core.AppInfo) {
	fpkPath, err := p.downloadFpk(ctx, stream, app)
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}
	defer os.Remove(fpkPath)

	volume, err := p.resolveVolume()
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "self_update", Message: "商店正在重启..."})

	// Process will be killed by fnOS during install-fpk; no verification afterwards.
	_ = p.installFpk(fpkPath, volume)
}
