//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
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

func (a *LinuxAppCenter) run(args ...string) (string, error) {
	cmd := exec.Command(a.CLIPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("appcenter-cli %s: %w: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
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
	return out == "Installed", nil
}

func (a *LinuxAppCenter) Status(appname string) (string, error) {
	return a.run("status", appname)
}

func (a *LinuxAppCenter) InstallFpk(fpkPath string, volume int) error {
	dir, err := a.extractFpk(fpkPath)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return a.InstallLocal(dir, volume, false)
}

func (a *LinuxAppCenter) InstallLocal(dir string, volume int, detach bool) error {
	args := []string{"install-local", "--dir", dir, "-v", strconv.Itoa(volume)}
	if detach {
		cmd := exec.Command(a.CLIPath, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		cmd.Stdout = nil
		cmd.Stderr = nil
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
	return strconv.Atoi(out)
}
