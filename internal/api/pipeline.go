package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
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

// selfUpdateFlushDelay is how long runSelfUpdate waits between sending the
// 'self_update' SSE event and forking the detached appcenter-cli child.
// The delay lets the SSE bytes reach the client's fetch reader before the
// child kills this process during install-local's uninstall phase. Without
// this delay the client sees a connection reset BEFORE the self_update
// event arrives, producing a false-positive '更新请求失败' toast
// even though the update succeeds in the background.
// 750ms is empirically sufficient on localhost; the upper bound is bounded
// by appcenter-cli's own startup time, which is well above 1s.
const selfUpdateFlushDelay = 750 * time.Millisecond

type installPipeline struct {
	downloads  *core.Downloader
	ac         platform.AppCenter
	queue      *OperationQueue
	appsDir    string
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

// resolveVolumeFor picks the installation volume for an operation. For an
// update of an already-installed app it PINS to the volume the app currently
// lives on: fnOS install-local does an uninstall-then-reinstall, so handing it
// a different volume relocates the app and orphans its existing data (the data
// loss reported in conversun/fnos-apps#189). It therefore fails closed rather
// than falling back to the global default. Fresh installs use the configured
// or default volume.
func (p *installPipeline) resolveVolumeFor(opName, appname string) (int, error) {
	if opName == "update" {
		vol, found, err := p.ac.AppInstallVolume(appname)
		if err != nil {
			return 0, fmt.Errorf("无法确定 %s 当前所在的存储卷，已中止更新以保护现有数据: %w", appname, err)
		}
		if !found {
			return 0, fmt.Errorf("无法确定 %s 当前所在的存储卷，已中止更新以保护现有数据。请在应用中心手动更新", appname)
		}
		return vol, nil
	}
	return p.resolveVolume()
}

// preflightInstall fails closed BEFORE the destructive install-local step,
// which uninstalls the existing app before reinstalling. It confirms the target
// volume is actually mounted and has generous free space, so a missing or full
// volume aborts the operation instead of leaving the app uninstalled with its
// data orphaned — the failure mode behind conversun/fnos-apps#189.
func (p *installPipeline) preflightInstall(volume int, fpkPath string) error {
	fi, err := os.Stat(fpkPath)
	if err != nil {
		return fmt.Errorf("安装包不可读，已中止安装以保护现有数据: %w", err)
	}
	volumes, err := p.ac.ListVolumes()
	if err != nil {
		return fmt.Errorf("无法枚举存储卷，已中止安装以保护现有数据: %w", err)
	}
	var target *platform.VolumeInfo
	for i := range volumes {
		if volumes[i].Index == volume {
			target = &volumes[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("目标存储卷 vol%d 不可用（未挂载或不存在），已中止安装以保护现有数据。请在设置中选择正确的安装硬盘后重试", volume)
	}
	if need := uint64(fi.Size()) * 3; target.FreeBytes < need {
		return fmt.Errorf("目标存储卷 vol%d 空间不足（可用 %d 字节，约需 %d 字节），已中止安装以保护现有数据", volume, target.FreeBytes, need)
	}
	return nil
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

// verifyRetryDelays are the sleep durations between successive Check() attempts
// inside verifyInstalled. The first entry MUST be 0 so the first Check fires
// immediately; subsequent entries pace the retry loop out to ~52s total to
// survive fnOS appcenter-cli's async post-install registration commit.
// See GitHub issue conversun/fnos-apps#181.
var verifyRetryDelays = []time.Duration{
	0,
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
	3 * time.Second,
	3 * time.Second,
	4 * time.Second,
	6 * time.Second,
	6 * time.Second,
	8 * time.Second,
	8 * time.Second,
	10 * time.Second,
}

// verifyWait sleeps for d respecting ctx. Returns ctx.Err() if canceled.
// It is a package variable so tests can override it to skip real sleeps
// while preserving ctx-cancellation semantics.
var verifyWait = func(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// verifyInstalled polls appcenter-cli check with backoff to survive fnOS's
// asynchronous post-install registration commit. A single-shot check races
// the DB write and reports the app as not installed, producing the
// "安装后验证失败" toast reported in conversun/fnos-apps#181.
//
// Semantics:
//   - Attempt 1 fires immediately (verifyRetryDelays[0] == 0).
//   - Subsequent attempts pace to ~52s total budget.
//   - Hard CLI errors (Check returns err != nil) get one filesystem-manifest
//     confirmation, then short-circuit — no retry.
//   - ctx cancellation is honored between attempts via verifyWait.
//   - Final fallbacks: accept a sane List() row (running|stopped), or an
//     on-disk manifest when appcenter-cli output has drifted.
func (p *installPipeline) verifyInstalled(ctx context.Context, appname string) error {
	for i, delay := range verifyRetryDelays {
		if err := verifyWait(ctx, delay); err != nil {
			return err
		}
		var installed bool
		err := p.queue.WithCLI(func() error {
			var e error
			installed, e = p.ac.Check(appname)
			return e
		})
		if err != nil {
			if p.manifestExists(appname) {
				log.Printf("verifyInstalled: %s matched via filesystem fallback after Check() error", appname)
				return nil
			}
			return err
		}
		log.Printf("verifyInstalled: %s attempt %d/%d installed=%v", appname, i+1, len(verifyRetryDelays), installed)
		if installed {
			return nil
		}
	}
	// Fallback: List() is a broader signal that tolerates Check() output drift
	// (locale, status suffixes). Accept only when the app row is present AND
	// has a sane status — reject unknown/empty to avoid blessing broken installs.
	var apps []platform.InstalledApp
	listErr := p.queue.WithCLI(func() error {
		var e error
		apps, e = p.ac.List()
		return e
	})
	if listErr != nil {
		log.Printf("verifyInstalled: %s List() fallback failed: %v", appname, listErr)
	} else {
		for _, a := range apps {
			if a.AppName == appname && (a.Status == "running" || a.Status == "stopped") {
				log.Printf("verifyInstalled: %s matched via List() fallback status=%s", appname, a.Status)
				return nil
			}
		}
	}
	if p.manifestExists(appname) {
		log.Printf("verifyInstalled: %s matched via filesystem fallback", appname)
		return nil
	}
	return fmt.Errorf("安装后验证失败：应用未在 appcenter 注册（重试 %d 次共 %s 后仍未检出）。请查看应用日志或稍后重试", len(verifyRetryDelays), verifyTotal())
}

func (p *installPipeline) manifestExists(appname string) bool {
	info, err := os.Stat(filepath.Join(p.appsDir, appname, "manifest"))
	return err == nil && !info.IsDir()
}

// verifyTotal returns the total wall-clock time verifyInstalled will spend
// on Check retries before giving up and consulting List().
func verifyTotal() time.Duration {
	var t time.Duration
	for _, d := range verifyRetryDelays {
		t += d
	}
	return t
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
			// Don't orphan the goroutine: wait for fn() to actually finish so
			// any caller-deferred cleanup (e.g. os.Remove(fpkPath)) doesn't race
			// with an in-flight CLI invocation that's still reading the file.
			// cliMu already serializes operations, so blocking here doesn't add
			// queueing pressure beyond what the next op would face anyway.
			<-done
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

	mirror := os.Getenv("DOCKER_MIRROR")
	var multiRegistry bool
	if p.configMgr != nil {
		multiRegistry = config.IsDockerMirrorMultiRegistry(p.configMgr.Get().DockerMirror)
	}

	images := parseDockerImages(string(data), app, mirror)
	if len(images) == 0 {
		return nil // no images found — not a docker app
	}

	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintf(os.Stderr, "dockerPull: docker not found, skipping pre-pull\n")
		return nil
	}

	for i, composeRef := range images {
		msg := fmt.Sprintf("正在拉取 Docker 镜像 (%d/%d)...", i+1, len(images))
		if len(images) == 1 {
			msg = "正在拉取 Docker 镜像..."
		}
		_ = stream.sendProgress(progressPayload{Step: "pulling", Progress: 0, Message: msg})

		pullRef := normalizeImageForPull(composeRef, mirror, multiRegistry)
		if err := p.pullSingleImage(ctx, stream, pullRef, msg); err != nil {
			return err
		}
		if pullRef != composeRef {
			_ = exec.CommandContext(ctx, "docker", "tag", pullRef, composeRef).Run()
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

func parseDockerImages(content string, app core.AppInfo, mirror string) []string {
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

func normalizeImageForPull(image, mirror string, multiRegistry bool) string {
	if mirror == "" || multiRegistry {
		return image
	}
	if !strings.HasPrefix(image, mirror) {
		return image
	}
	afterMirror := image[len(mirror):]

	if strings.HasPrefix(afterMirror, "docker.io/") {
		return mirror + afterMirror[len("docker.io/"):]
	}

	idx := strings.IndexByte(afterMirror, '/')
	if idx > 0 && strings.ContainsRune(afterMirror[:idx], '.') {
		return afterMirror
	}

	return image
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

	volume, err := p.resolveVolumeFor(opName, app.AppName)
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if err := p.preflightInstall(volume, fpkPath); err != nil {
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
		return p.verifyInstalled(ctx, app.AppName)
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

	volume, err := p.resolveVolumeFor("update", app.AppName)
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	if err := p.preflightInstall(volume, fpkPath); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	dir, err := p.extractFpk(fpkPath)
	if err != nil {
		_ = stream.sendError(err.Error())
		return
	}
	// dir cleanup is conditional on the InstallLocal outcome below.

	_ = stream.sendProgress(progressPayload{Step: "self_update", Message: "商店正在重启..."})

	// Wait for the SSE bytes to actually reach the client. See the comment on
	// selfUpdateFlushDelay for why this is necessary.
	select {
	case <-time.After(selfUpdateFlushDelay):
	case <-ctx.Done():
	}

	// Detached: appcenter-cli runs in a new session so it survives this
	// process being killed during install-local's uninstall phase.
	if err := p.ac.InstallLocal(dir, volume, true); err != nil {
		// The fork itself failed - the child never started, so it's safe
		// (and necessary) to clean up the extracted directory here.
		log.Printf("runSelfUpdate: InstallLocal launch failed: %v", err)
		_ = stream.sendError(fmt.Sprintf("商店更新启动失败: %v", err))
		_ = os.RemoveAll(dir)
		return
	}
	// Success path: dir is intentionally NOT cleaned up - the detached child
	// reads it asynchronously after cmd.Start() returns, and fnOS will kill
	// this process before any deferred cleanup could run. /tmp is wiped on
	// reboot.
}
