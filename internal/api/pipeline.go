package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"

	"fnos-store/internal/config"
	"fnos-store/internal/core"
	"fnos-store/internal/platform"
)

type installPipeline struct {
	downloads  *core.Downloader
	ac         platform.AppCenter
	queue      *OperationQueue
	configMgr  *config.Manager
	cacheStore cacheTagStore
}

type cacheTagStore interface {
	SetInstalledTag(appname, releaseTag string)
	RemoveInstalledTag(appname string)
}

func (p *installPipeline) extractFpk(fpkPath string) (string, error) {
	dir, err := os.MkdirTemp("", "fpk-install-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	cmd := exec.Command("tar", "xzf", fpkPath, "-C", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("解压 fpk 失败: %w: %s", err, string(out))
	}
	return dir, nil
}

func (p *installPipeline) downloadFpk(ctx context.Context, stream *sseStream, app core.AppInfo) (string, error) {
	if p.downloads == nil {
		return "", errors.New("下载器未配置")
	}

	fileName := path.Base(app.DownloadURL)
	if fileName == "." || fileName == "/" || fileName == "" {
		return "", errors.New("下载地址无效")
	}

	_ = stream.sendProgress(progressPayload{Step: "downloading", Progress: 0, Message: "正在下载..."})

	startTime := time.Now()
	var lastSend time.Time

	var cfg config.Config
	if p.configMgr != nil {
		cfg = p.configMgr.Get()
	} else {
		cfg = config.Config{Mirror: config.DefaultMirror, DockerMirror: config.DefaultDockerMirror}
	}

	dockerPrefix := config.DockerMirrorPrefix(cfg.DockerMirror, cfg)
	if dockerPrefix != "" {
		os.Setenv("DOCKER_MIRROR", dockerPrefix)
	} else {
		os.Unsetenv("DOCKER_MIRROR")
	}

	prefixes := config.GitHubFallbackPrefixes(cfg.Mirror, cfg)
	downloadURLs := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		if prefix != "" {
			downloadURLs = append(downloadURLs, prefix+app.DownloadURL)
		} else {
			downloadURLs = append(downloadURLs, app.DownloadURL)
		}
	}

	fpkPath, err := p.downloads.Download(ctx, core.DownloadRequest{
		URLs:     downloadURLs,
		FileName: fileName,
		AppName:  app.AppName,
	}, func(downloaded, total int64) {
		if total <= 0 {
			return
		}

		now := time.Now()
		isFinal := downloaded >= total
		if !isFinal && now.Sub(lastSend) < 200*time.Millisecond {
			return
		}
		lastSend = now

		pct := int(float64(downloaded) * 100 / float64(total))
		if pct > 100 {
			pct = 100
		}

		var speed int64
		if elapsed := now.Sub(startTime).Seconds(); elapsed > 0 {
			speed = int64(float64(downloaded) / elapsed)
		}

		_ = stream.sendProgress(progressPayload{
			Step:       "downloading",
			Progress:   pct,
			Speed:      speed,
			Downloaded: downloaded,
			Total:      total,
		})
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

func (p *installPipeline) startApp(appname string) error {
	return p.queue.WithCLI(func() error {
		return p.ac.Start(appname)
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
		return fmt.Errorf("安装后验证失败，应用未正确安装")
	}
	return nil
}

func runWithVirtualProgress(ctx context.Context, stream *sseStream, step, message string, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	progress := 0
	_ = stream.sendProgress(progressPayload{Step: step, Progress: 0, Message: message})

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err == nil {
				_ = stream.sendProgress(progressPayload{Step: step, Progress: 100, Message: message})
			}
			return err
		case <-ticker.C:
			remaining := 95 - progress
			if remaining <= 0 {
				continue
			}
			inc := remaining / 8
			if inc < 1 {
				inc = 1
			}
			progress += inc
			_ = stream.sendProgress(progressPayload{Step: step, Progress: progress, Message: message})
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *installPipeline) runStandard(ctx context.Context, stream *sseStream, opName string, app core.AppInfo, refreshFn func(context.Context) error) {
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

	if err := runWithVirtualProgress(ctx, stream, "installing", "正在安装...", func() error {
		return p.installFpk(fpkPath, volume)
	}); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if err := runWithVirtualProgress(ctx, stream, "verifying", "正在验证安装...", func() error {
		return p.verifyInstalled(app.AppName)
	}); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if err := runWithVirtualProgress(ctx, stream, "starting", "正在启动...", func() error {
		return p.startApp(app.AppName)
	}); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if p.cacheStore != nil && app.ReleaseTag != "" {
		p.cacheStore.SetInstalledTag(app.AppName, app.ReleaseTag)
	}

	_ = refreshFn(ctx)

	newVersion := app.FpkVersion
	if newVersion == "" {
		newVersion = app.LatestVersion
	}
	_ = stream.sendProgress(progressPayload{Step: "done", NewVersion: newVersion, Message: "操作完成"})
}

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

	dir, err := p.extractFpk(fpkPath)
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "self_update", Message: "商店正在重启..."})

	// Detached: appcenter-cli runs in a new session so it survives
	// this process being killed during the uninstall phase.
	_ = p.ac.InstallLocal(dir, volume, true)
}
