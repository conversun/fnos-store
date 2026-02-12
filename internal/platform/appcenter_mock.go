//go:build !linux

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type MockAppCenter struct {
	ScriptPath string
}

func NewAppCenter(projectRoot string) AppCenter {
	return NewMockAppCenter(projectRoot)
}

func NewMockAppCenter(projectRoot string) *MockAppCenter {
	return &MockAppCenter{
		ScriptPath: filepath.Join(projectRoot, "dev", "mock-appcenter-cli.sh"),
	}
}

func (m *MockAppCenter) run(args ...string) (string, error) {
	cmd := exec.Command("bash", append([]string{m.ScriptPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("mock-appcenter-cli %s: %w: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (m *MockAppCenter) List() ([]InstalledApp, error) {
	out, err := m.run("list")
	if err != nil {
		return nil, err
	}

	var apps []InstalledApp
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] == "APPNAME" {
			continue
		}
		app := InstalledApp{
			AppName: fields[0],
			Version: fields[1],
			Status:  fields[2],
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func (m *MockAppCenter) Check(appname string) (bool, error) {
	out, err := m.run("check", appname)
	if err != nil {
		return false, err
	}
	return out == "Installed", nil
}

func (m *MockAppCenter) Status(appname string) (string, error) {
	return m.run("status", appname)
}

func (m *MockAppCenter) InstallFpk(fpkPath string, volume int) error {
	_, err := m.run("install-fpk", fpkPath, "-v", strconv.Itoa(volume))
	return err
}

func (m *MockAppCenter) Uninstall(appname string) error {
	_, err := m.run("uninstall", appname)
	return err
}

func (m *MockAppCenter) Start(appname string) error {
	_, err := m.run("start", appname)
	return err
}

func (m *MockAppCenter) Stop(appname string) error {
	_, err := m.run("stop", appname)
	return err
}

func (m *MockAppCenter) DefaultVolume() (int, error) {
	out, err := m.run("default-volume")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}
