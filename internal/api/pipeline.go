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
	// Self-update feeds this directory straight to install-local, so the hooks
	// must be executable here too — see platform.EnsureHooksExecutable.
	platform.EnsureHooksExecutable(dir)
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

// requireSafeUpgrade refuses an update on fnOS builds where the upgrade path
// is known to destroy the app.
//
// This must run BEFORE anything is downloaded or written, because the
// destructive step is irreversible: install-local uninstalls the app first,
// the reinstall then fails, and there is no retained copy of the old version
// to restore. verifyPayloadLanded detects the loss afterwards but cannot undo
// it (conversun/fnos-apps#189).
func (p *installPipeline) requireSafeUpgrade() error {
	cap := p.ac.UpgradeCapability()
	if cap.Allowed {
		return nil
	}
	return errors.New(cap.Reason)
}

// resolveVolume picks the volume for a FRESH install: the user's explicit
// choice if set, otherwise fnOS's default.
//
// The CLI's default-volume getter is unreliable — on fnOS 1.2.0203 it returns
// 0 while the daemon's own database holds a valid index — and vol0 does not
// exist. Rather than dead-ending the user on a default they never chose and
// cannot see is broken, an unusable value falls back to the single mounted
// volume when there is exactly one. Only a genuinely ambiguous system (several
// volumes, no usable default) asks the user to pick, because there is no safe
// way to guess which disk their apps belong on.
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
	mounted, listErr := p.mountedVolumes()
	if err != nil {
		// The getter itself failed. A single mounted volume is still an
		// unambiguous answer.
		if listErr == nil && len(mounted) == 1 {
			log.Printf("resolveVolume: DefaultVolume() failed (%v), using the only mounted volume vol%d", err, mounted[0])
			return mounted[0], nil
		}
		return 0, fmt.Errorf("无法获取默认安装硬盘（%w）。请在设置中选择安装硬盘后重试", err)
	}
	if listErr != nil {
		// Cannot enumerate volumes to validate; preflightInstall re-checks
		// before anything destructive happens.
		return volume, nil
	}
	for _, idx := range mounted {
		if idx == volume {
			return volume, nil
		}
	}
	if len(mounted) == 1 {
		log.Printf("resolveVolume: fnOS reported unusable default vol%d, using the only mounted volume vol%d", volume, mounted[0])
		return mounted[0], nil
	}
	return 0, fmt.Errorf("fnOS 返回的默认安装硬盘 vol%d 不可用，且当前有多个存储空间。请在设置 → 应用安装位置 中选择要安装到哪个存储空间后重试", volume)
}

// mountedVolumes returns the indexes of the currently mounted volumes.
func (p *installPipeline) mountedVolumes() ([]int, error) {
	volumes, err := p.ac.ListVolumes()
	if err != nil {
		return nil, err
	}
	idx := make([]int, 0, len(volumes))
	for _, v := range volumes {
		idx = append(idx, v.Index)
	}
	return idx, nil
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

// setDefaultVolume pins fnOS's default install volume and PROVES it took
// effect by reading the value back inside the same CLI critical section.
//
// The read-back is the whole point. `appcenter-cli default-volume <n>` exits 0
// and prints the unchanged value when the daemon declines the change, so a nil
// error from the setter is NOT evidence the volume was pinned. Worse, the CLI's
// own help documents -v/--volume as "(ignored during upgrades)", which means
// default-volume is the ONLY effective placement control for the destructive
// install-local path. Trusting an unverified setter is exactly how an update
// proceeded into an uninstall it could not undo (conversun/fnos-apps#189).
//
// Set and get share one WithCLI block so a concurrent operation cannot slip in
// between them and invalidate the verification.
func (p *installPipeline) setDefaultVolume(volume int) error {
	return p.queue.WithCLI(func() error {
		if err := p.ac.SetDefaultVolume(volume); err != nil {
			return err
		}
		got, err := p.ac.DefaultVolume()
		if err != nil {
			return fmt.Errorf("设置后无法读回默认安装卷: %w", err)
		}
		if got != volume {
			return fmt.Errorf("默认安装卷设置未生效（请求 vol%d，实际仍为 vol%d）。当前系统有多个存储空间，为避免应用被迁移到其他硬盘导致数据丢失，已中止本次更新。请使用飞牛系统自带应用中心手动更新", volume, got)
		}
		return nil
	})
}

// upgradeFpk drives the daemon's data-preserving upgrade. It deliberately has
// no install-local fallback: a failed RPC upgrade leaves the app untouched,
// whereas falling back would destroy it.
func (p *installPipeline) upgradeFpk(ctx context.Context, fpkPath string) error {
	return p.queue.WithCLI(func() error {
		return p.ac.UpgradeFpk(ctx, fpkPath, nil)
	})
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

// verifyPayloadLanded proves an install/update actually produced files, on the
// volume we pinned, at the version we shipped. It runs AFTER verifyInstalled
// because the appcenter control plane is not trustworthy on its own:
//
//   - `check` returned "Installed" for an app whose entire directory had been
//     deleted (the store itself was in that state: its binary showed as
//     "(deleted)" in /proc while check still claimed Installed).
//   - install-local can uninstall the old copy, fail the reinstall, and still
//     exit 0 — leaving a stale manifest that the control-plane checks happily
//     accept as success.
//
// wantVersion is the version the fpk was supposed to deliver — app.FpkVersion,
// which for a repackaged build carries a revision suffix (1.9.3-r2) that the
// manifest's own `version` field does NOT have. It is therefore compared
// against the manifest's fpk_version first, falling back to version only when
// the package ships no fpk_version. Comparing it against `version` alone would
// reject a perfectly good install of any revision package — 29 of 145 apps in
// the catalog carry a -rN suffix (1panel, alist, gitea, gopeed, embyserver …).
func (p *installPipeline) verifyPayloadLanded(appname string, wantVolume int, wantVersion string) error {
	targetLink := filepath.Join(p.appsDir, appname, "target")
	resolved, err := filepath.EvalSymlinks(targetLink)
	if err != nil {
		return fmt.Errorf("安装校验失败：无法定位 %s 的安装目录（%v）。应用可能未成功安装，请在应用中心确认", appname, err)
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return fmt.Errorf("安装校验失败：无法读取 %s 的安装目录 %s（%v）", appname, resolved, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("安装校验失败：%s 的安装目录 %s 为空，安装包未落盘。请在应用中心手动重新安装", appname, resolved)
	}

	// Confirm the payload landed on the volume we pinned. A mismatch means the
	// app was relocated, which orphans its data under the old volume.
	if wantVolume > 0 {
		volumes, err := p.ac.ListVolumes()
		if err == nil {
			if idx, ok := volumeIndexOf(resolved, volumes); ok && idx != wantVolume {
				return fmt.Errorf("安装校验失败：%s 被安装到 vol%d，而非预期的 vol%d，原有数据可能已被遗留在 vol%d。请在应用中心确认",
					appname, idx, wantVolume, wantVolume)
			}
		}
	}

	if wantVersion == "" {
		return nil
	}
	m, err := core.ParseManifest(filepath.Join(p.appsDir, appname, "manifest"))
	if err != nil {
		// Manifest unreadable but files landed: don't fail the operation on a
		// parse problem alone — the payload check above already passed.
		log.Printf("verifyPayloadLanded: %s manifest unreadable: %v", appname, err)
		return nil
	}
	// Prefer fpk_version: that is the field wantVersion (app.FpkVersion) actually
	// corresponds to, including any -rN revision suffix.
	installed := m.FpkVersion
	if installed == "" {
		installed = m.Version
	}
	if installed != "" && installed != wantVersion {
		return fmt.Errorf("更新校验失败：%s 仍是 %s，未升级到 %s。安装程序报告成功但未生效，请在应用中心手动更新",
			appname, installed, wantVersion)
	}
	return nil
}

// volumeIndexOf reports which mounted volume contains path.
func volumeIndexOf(path string, volumes []platform.VolumeInfo) (int, bool) {
	bestIdx, bestLen := 0, -1
	clean := filepath.Clean(path)
	for _, v := range volumes {
		vp := filepath.Clean(v.Path)
		if clean != vp && !strings.HasPrefix(clean, vp+string(filepath.Separator)) {
			continue
		}
		if len(vp) > bestLen {
			bestIdx, bestLen = v.Index, len(vp)
		}
	}
	return bestIdx, bestLen >= 0
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
	// Guard first: refuse before downloading, so an affected system never
	// reaches the uninstall-then-failed-reinstall path.
	if opName == "update" {
		if err := p.requireSafeUpgrade(); err != nil {
			_ = stream.sendError(err.Error())
			return
		}
	}

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

	// Updates go through the daemon's own upgrade channel, which preserves
	// @appdata and can roll back. install-local (used for fresh installs) is
	// uninstall-then-reinstall and destroys the app when the reinstall fails
	// — conversun/fnos-apps#189. A failure here must NEVER fall back to it.
	installStep := func() error { return p.installFpk(fpkPath, volume) }
	if opName == "update" {
		installStep = func() error { return p.upgradeFpk(ctx, fpkPath) }
	}

	if err := runWithVirtualProgress(ctx, stream, "installing", "正在安装...", installStep); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	expectedVersion := app.FpkVersion
	if expectedVersion == "" {
		expectedVersion = app.LatestVersion
	}

	if err := runWithVirtualProgress(ctx, stream, "verifying", "正在验证安装...", func() error {
		if err := p.verifyInstalled(ctx, app.AppName); err != nil {
			return err
		}
		// The control-plane checks above can pass on a destroyed app, so the
		// operation is only really successful once the payload is proven on
		// disk, on the pinned volume, at the shipped version.
		return p.verifyPayloadLanded(app.AppName, volume, expectedVersion)
	}); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

	// Only fresh installs need an explicit start. The daemon's upgrade already
	// restores the app to its previous running state, so starting again yields
	// "[Info]Application [x] is already started." — which the CLI reports as a
	// failure, turning a successful upgrade into a user-visible error.
	if opName != "update" {
		if err := runWithVirtualProgress(ctx, stream, "starting", "正在启动...", func() error {
			return p.startApp(app.AppName)
		}); err != nil {
			_ = stream.sendError(err.Error())
			return
		}
	}

	if p.cacheStore != nil && app.ReleaseTag != "" {
		p.cacheStore.SetInstalledTag(app.AppName, app.ReleaseTag)
	}

	_ = refreshFn(ctx)

	newVersion := expectedVersion
	_ = stream.sendProgress(progressPayload{Step: "done", NewVersion: newVersion, Message: "操作完成"})
}

func (p *installPipeline) runSelfUpdate(ctx context.Context, stream *sseStream, app core.AppInfo) {
	// Self-update is the riskiest path: the child is detached and this process
	// is killed partway through, so a failure cannot even be reported.
	if err := p.requireSafeUpgrade(); err != nil {
		_ = stream.sendError(err.Error())
		return
	}

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

	if err := p.setDefaultVolume(volume); err != nil {
		_ = stream.sendError(fmt.Sprintf("无法锁定安装目标卷 vol%d，已中止商店更新以保护现有数据: %v", volume, err))
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
	//
	// Still routed through WithCLI even though it returns immediately: the
	// scheduler's registry refresh calls List() through the same lock, and the
	// launch must not interleave with it. cmd.Start() does not wait for the
	// child, so holding the lock here costs nothing.
	if err := p.queue.WithCLI(func() error {
		return p.ac.InstallLocal(dir, volume, true)
	}); err != nil {
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
