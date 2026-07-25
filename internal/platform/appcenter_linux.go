//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type LinuxAppCenter struct {
	CLIPath string
}

func NewAppCenter(_ string) AppCenter {
	return NewLinuxAppCenter()
}

func NewLinuxAppCenter() *LinuxAppCenter {
	return &LinuxAppCenter{
		CLIPath: "/usr/local/bin/appcenter-cli",
	}
}

// run executes appcenter-cli and treats an error envelope in the output as a
// failure even when the process exits 0 — which it always does. See clierr.go
// for the measured evidence and why exit status cannot be trusted.
func (a *LinuxAppCenter) run(args ...string) (string, error) {
	cmd := exec.Command(a.CLIPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("appcenter-cli %s: %w: %s", strings.Join(args, " "), err, string(out))
	}
	text := strings.TrimSpace(string(out))
	if cliOutputFailure(text) {
		return "", fmt.Errorf("appcenter-cli %s: %w: %s",
			strings.Join(args, " "), ErrCLIFailure, cliErrorDetail(text))
	}
	return text, nil
}

func (a *LinuxAppCenter) List() ([]InstalledApp, error) {
	out, err := a.run("list")
	if err != nil {
		return nil, err
	}
	return parseListTable(out)
}

func (a *LinuxAppCenter) Check(appname string) (bool, error) {
	out, err := a.run("check", appname)
	if err != nil {
		return false, err
	}
	return parseCheckOutput(out)
}

func (a *LinuxAppCenter) Status(appname string) (string, error) {
	out, err := a.run("status", appname)
	if err != nil {
		return "", err
	}
	return parseStatusOutput(out)
}

func (a *LinuxAppCenter) InstallFpk(fpkPath string, volume int) error {
	dir, err := a.extractFpk(fpkPath)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return a.InstallLocal(dir, volume, false)
}

// SelfUpdateLogPath is where a detached install-local writes its output.
// The store process is killed by fnOS partway through its own update, so this
// file is the only record of whether the update actually succeeded — the
// previous implementation discarded both streams, making every self-update
// failure invisible after the restart.
const SelfUpdateLogPath = "/tmp/fnos-store-selfupdate.log"

// InstallLocal runs install-local, which is destructive: fnOS uninstalls the
// existing app before reinstalling it. The -v/--volume flag is documented by
// the CLI itself as "(ignored during upgrades)", so it does NOT protect an
// update from being placed elsewhere — only the daemon's default-volume does.
//
// When detach is set the child runs in its own session so it survives this
// process being killed during the uninstall phase, and its output is captured
// to SelfUpdateLogPath for post-restart diagnosis.
func (a *LinuxAppCenter) InstallLocal(dir string, volume int, detach bool) error {
	args := []string{"install-local", "--dir", dir, "-v", strconv.Itoa(volume)}
	if detach {
		cmd := exec.Command(a.CLIPath, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		// Best-effort: losing the log must not block the update itself, it
		// only costs us the post-restart failure diagnosis.
		if f, err := os.OpenFile(SelfUpdateLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			defer f.Close()
			cmd.Stdout = f
			cmd.Stderr = f
		}
		return cmd.Start()
	}
	_, err := a.run(args...)
	return err
}

func (a *LinuxAppCenter) extractFpk(fpkPath string) (string, error) {
	dir, err := os.MkdirTemp("", "fpk-install-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	cmd := exec.Command("tar", "xzf", fpkPath, "-C", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("extract fpk: %w: %s", err, string(out))
	}
	EnsureHooksExecutable(dir)
	return dir, nil
}

func (a *LinuxAppCenter) Uninstall(appname string) error {
	_, err := a.run("uninstall", appname)
	return err
}

func (a *LinuxAppCenter) Start(appname string) error {
	_, err := a.run("start", appname)
	return err
}

func (a *LinuxAppCenter) Stop(appname string) error {
	_, err := a.run("stop", appname)
	return err
}

func (a *LinuxAppCenter) DefaultVolume() (int, error) {
	out, err := a.run("default-volume")
	if err != nil {
		return 0, err
	}
	return parseVolumeOutput(out)
}

// SetDefaultVolume asks the CLI to move fnOS's default install volume.
// It reports only whether the command was accepted; on some fnOS builds the
// set is a silent no-op, so callers MUST read the value back and compare
// rather than treating a nil error as proof the volume was pinned.
func (a *LinuxAppCenter) SetDefaultVolume(volume int) error {
	_, err := a.run("default-volume", strconv.Itoa(volume))
	return err
}

// isMountPoint returns true if path is a mount point (its device differs
// from the parent directory's device). This filters out plain directories
// like /vol00 or /vol02 that sit on the root partition and are not real
// storage volumes.
func isMountPoint(path string) bool {
	var pathStat, parentStat syscall.Stat_t
	if err := syscall.Stat(path, &pathStat); err != nil {
		return false
	}
	if err := syscall.Stat(filepath.Dir(path), &parentStat); err != nil {
		return false
	}
	return pathStat.Dev != parentStat.Dev
}

func (a *LinuxAppCenter) ListVolumes() ([]VolumeInfo, error) {
	matches, err := filepath.Glob("/vol*")
	if err != nil {
		return nil, err
	}

	sort.Slice(matches, func(i, j int) bool {
		if len(matches[i]) != len(matches[j]) {
			return len(matches[i]) < len(matches[j])
		}
		return matches[i] < matches[j]
	})

	seen := make(map[int]bool)
	var volumes []VolumeInfo
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || !info.IsDir() {
			continue
		}
		name := filepath.Base(m)
		if !strings.HasPrefix(name, "vol") {
			continue
		}
		idx, err := strconv.Atoi(name[3:])
		if err != nil {
			continue
		}
		if !isMountPoint(m) {
			continue
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true

		vol := VolumeInfo{Index: idx, Path: m}
		var stat syscall.Statfs_t
		if err := syscall.Statfs(m, &stat); err == nil {
			vol.TotalBytes = stat.Blocks * uint64(stat.Bsize)
			vol.FreeBytes = stat.Bavail * uint64(stat.Bsize)
		}
		volumes = append(volumes, vol)
	}

	sort.Slice(volumes, func(i, j int) bool {
		return volumes[i].Index < volumes[j].Index
	})
	return volumes, nil
}

// AppInstallVolume resolves the volume an app currently lives on by reading its
// /var/apps/<app>/target symlink (-> /volN/@appcenter/<app>) and matching the
// resolved path against the mounted volumes. This is the CLI-independent source
// of truth used to pin an update to the app's existing volume instead of a
// re-resolved global default, which would relocate the app and orphan its data.
// found is false when the app is not installed or its layout cannot be mapped
// to a known volume.
func (a *LinuxAppCenter) AppInstallVolume(appname string) (int, bool, error) {
	volumes, err := a.ListVolumes()
	if err != nil {
		return 0, false, err
	}
	// Prefer the binary target; fall back to the runtime data dir.
	for _, sub := range []string{"target", "var"} {
		resolved, err := filepath.EvalSymlinks(filepath.Join("/var/apps", appname, sub))
		if err != nil {
			continue
		}
		if idx, ok := volumeIndexForPath(resolved, volumes); ok {
			return idx, true, nil
		}
	}
	return 0, false, nil
}
