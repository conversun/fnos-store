package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"fnos-store/internal/core"
	"fnos-store/internal/platform"
)

type installPipeline struct {
	downloads  *core.Downloader
	ac         platform.AppCenter
	queue      *OperationQueue
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

func (p *installPipeline) runStandard(ctx context.Context, stream *sseStream, opName string, app core.AppInfo, refreshFn func(context.Context) error) {
	fpkPath, err := p.downloadFpk(ctx, stream, app)
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}
	defer os.Remove(fpkPath)

	_ = stream.sendProgress(progressPayload{Step: "installing", Message: "正在安装..."})

	volume, err := p.resolveVolume()
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if err := p.installFpk(fpkPath, volume); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "verifying", Message: "正在验证安装..."})

	if err := p.verifyInstalled(app.AppName); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	_ = stream.sendProgress(progressPayload{Step: "starting", Message: "正在启动..."})
	if err := p.startApp(app.AppName); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if p.cacheStore != nil && app.ReleaseTag != "" {
		p.cacheStore.SetInstalledTag(app.AppName, app.ReleaseTag)
	}

	if err := refreshFn(ctx); err != nil && !errors.Is(err, context.Canceled) {
		_ = stream.sendError(err.Error())
		return
	}

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
