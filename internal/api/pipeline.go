package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
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
	if p.configMgr != nil {
		if v := p.configMgr.Get().InstallVolume; v > 0 {
			return v, nil
		}
	}
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

func (p *installPipeline) dockerPull(ctx context.Context, stream *sseStream, fpkDir string, app core.AppInfo) error {
	// docker-compose.yaml is inside app.tgz, not at fpk top level
	appTgz := filepath.Join(fpkDir, "app.tgz")
	appDir := filepath.Join(fpkDir, "app-contents")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "dockerPull: create app dir: %v\n", err)
		return nil // non-fatal: let install handle it
	}
	if out, err := exec.CommandContext(ctx, "tar", "xzf", appTgz, "-C", appDir).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "dockerPull: extract app.tgz: %v: %s\n", err, out)
		return nil // non-fatal: let install handle it
	}

	composePath := filepath.Join(appDir, "docker", "docker-compose.yaml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil // no compose file — not a docker app
	}

	images := parseDockerImages(string(data), app)
	if len(images) == 0 {
		return nil // no images found — not a docker app
	}

	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintf(os.Stderr, "dockerPull: docker not found, skipping pre-pull\n")
		return nil
	}

	for i, image := range images {
		msg := fmt.Sprintf("正在拉取 Docker 镜像 (%d/%d)...", i+1, len(images))
		if len(images) == 1 {
			msg = "正在拉取 Docker 镜像..."
		}
		_ = stream.sendProgress(progressPayload{Step: "pulling", Progress: 0, Message: msg})

		if err := p.pullSingleImage(ctx, stream, image, msg); err != nil {
			return err
		}
	}

	_ = stream.sendProgress(progressPayload{Step: "pulling", Progress: 100, Message: "Docker 镜像拉取完成"})
	return nil
}

func (p *installPipeline) pullSingleImage(ctx context.Context, stream *sseStream, image, message string) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", image)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Docker 镜像拉取失败: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Docker 镜像拉取失败: %w", err)
	}

	var totalLayers, completedLayers int
	var lastErrLine string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.Contains(line, "Pulling fs layer"), strings.Contains(line, "Waiting"):
			totalLayers++
		case strings.Contains(line, "Already exists"):
			totalLayers++
			completedLayers++
		case strings.Contains(line, "Pull complete"), strings.Contains(line, "Digest:"),
			strings.Contains(line, "Status:"), strings.Contains(line, "Downloading"),
			strings.Contains(line, "Extracting"), strings.Contains(line, "Verifying"):
			completedLayers++
		default:
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				lastErrLine = trimmed
			}
		}
		if totalLayers > 0 {
			pct := completedLayers * 100 / totalLayers
			if pct > 99 {
				pct = 99
			}
			_ = stream.sendProgress(progressPayload{Step: "pulling", Progress: pct, Message: message})
		}
	}

	if err := cmd.Wait(); err != nil {
		detail := err.Error()
		if lastErrLine != "" {
			detail = lastErrLine
		}
		return fmt.Errorf("Docker 镜像拉取失败: %s\n请尝试在 Docker 设置中更换镜像加速源后重试", detail)
	}

	return nil
}

func parseDockerImages(content string, app core.AppInfo) []string {
	mirror := os.Getenv("DOCKER_MIRROR")
	version := app.FpkVersion

	var images []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "image:") {
			continue
		}
		image := strings.TrimSpace(strings.TrimPrefix(trimmed, "image:"))
		if image == "" {
			continue
		}

		image = strings.ReplaceAll(image, "${DOCKER_MIRROR}", mirror)
		image = strings.ReplaceAll(image, "${VERSION}", version)

		images = append(images, image)
	}
	return images
}

func (p *installPipeline) runStandard(ctx context.Context, stream *sseStream, opName string, app core.AppInfo, refreshFn func(context.Context) error) {
	fpkPath, err := p.downloadFpk(ctx, stream, app)
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}
	defer os.Remove(fpkPath)

	if app.AppType == "docker" {
		dir, err := p.extractFpk(fpkPath)
		if err == nil {
			pullErr := p.dockerPull(ctx, stream, dir, app)
			os.RemoveAll(dir)
			if pullErr != nil {
				_ = stream.sendError(pullErr.Error())
				return
			}
		}
	}

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
