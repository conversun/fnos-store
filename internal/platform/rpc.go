//go:build linux

package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// The fnOS app-center daemon exposes its own HTTP API over a unix socket. This
// is the channel the web UI uses, and it is the ONLY way to upgrade an app
// without destroying it.
//
// Why this exists: `appcenter-cli` offers no data-preserving upgrade. Its
// `install-local` implements an upgrade as uninstall-then-reinstall, and on
// fnOS 1.2.0203 the reinstall always fails with error 10237 — measured twice on
// a live box, taking the app AND its @appdata with it
// (conversun/fnos-apps#189). The daemon, meanwhile, has a proper upgrade
// subsystem (Operation.Upgrade with Prepare/Restore/ActivateUpgradeRollback)
// that the CLI simply never exposes.
//
// Verified end-to-end on fnOS 1.2.0203: beszel 0.18.7-r1 -> r2 with a canary
// file in @appdata surviving byte-identical, and the daemon logging
// class=upgrade rather than a uninstall/install pair.
const daemonSocket = "/var/run/com.trim.app.center.sock"

// Daemon routes, measured. nginx proxies the browser's /app-center/* here
// unchanged, but these internal /rpc/v1 routes need no session token.
const (
	routeDownloadTask   = "/rpc/v1/download/task"
	routeDownloadStatus = "/rpc/v1/download/status"
	routeUpdateInfo     = "/rpc/v1/update/info"
	routeUpdateTask     = "/rpc/v1/update/task"
	routeInstallInfo    = "/rpc/v1/install/info"
	routeInstallTask    = "/rpc/v1/install/task"
	routeCommonStatus   = "/rpc/v1/common/status"
)

// Daemon status codes observed on the staging/task polls.
const (
	daemonStatusRunning = 1
	daemonStatusSuccess = 2
)

// rpcEnvelope is the daemon's uniform response shape.
type rpcEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// newDaemonClient dials the unix socket. The store runs as root on the same
// box, so no authentication is involved.
func newDaemonClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", daemonSocket)
			},
		},
	}
}

// daemonCall posts a JSON body and decodes data into out.
//
// A non-zero code is ALWAYS a failure. The daemon returns HTTP 200 with an
// error code in the body, so ignoring it would repeat the exact mistake that
// made appcenter-cli's exit status meaningless.
func daemonCall(ctx context.Context, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode %s request: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost"+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := newDaemonClient(60 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("app center daemon unreachable (%s): %w", path, err)
	}
	defer resp.Body.Close()

	var env rpcEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	if env.Code != 0 {
		return &DaemonError{Path: path, Code: env.Code, Msg: env.Msg}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decode %s data: %w", path, err)
		}
	}
	return nil
}

// DaemonError carries the daemon's own error code so callers can react to
// known ones (e.g. 19000 = a required wizard field is missing).
type DaemonError struct {
	Path string
	Code int
	Msg  string
}

func (e *DaemonError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("app center 返回错误 %d: %s", e.Code, e.Msg)
	}
	return fmt.Sprintf("app center 返回错误 %d (%s)", e.Code, e.Path)
}

// Daemon error codes worth naming.
const (
	daemonCodeValidation     = 10030 // malformed request
	daemonCodePackageMissing = 10100 // package not staged
	daemonCodeWizardRequired = 19000 // a required wizard field was not supplied
)

// StagedPackage describes an fpk the daemon has unpacked and identified.
type StagedPackage struct {
	AppName     string `json:"appName"`
	Version     string `json:"version"`
	Name        string `json:"name"`
	PackageType string `json:"packageType"`
	Installed   bool   `json:"installed"`
	Path        string `json:"path"`
}

type stageStatus struct {
	Status      int     `json:"status"`
	Message     string  `json:"message"`
	Progress    float64 `json:"progress"`
	PackageType string  `json:"packageType"`
	Path        string  `json:"path"`
	AppName     string  `json:"appName"`
	Version     string  `json:"version"`
	Name        string  `json:"name"`
	Installed   bool    `json:"installed"`
}

// StageFpk hands a local fpk to the daemon, which unpacks and identifies it.
// Both install and upgrade require this first: without it the info calls
// answer 10100 (package not found).
func (a *LinuxAppCenter) StageFpk(ctx context.Context, fpkPath string) (*StagedPackage, error) {
	var task struct {
		DownloadTaskID string `json:"downloadTaskId"`
	}
	err := daemonCall(ctx, routeDownloadTask, map[string]any{
		"packageSourceType": "file",
		"path":              fpkPath,
	}, &task)
	if err != nil {
		return nil, fmt.Errorf("暂存安装包失败: %w", err)
	}
	if task.DownloadTaskID == "" {
		return nil, fmt.Errorf("暂存安装包失败: app center 未返回任务 ID")
	}

	// Staging is fast (sub-second on a 12 MB fpk) but poll generously; a slow
	// volume should not look like a failure.
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		var st stageStatus
		if err := daemonCall(ctx, routeDownloadStatus, map[string]any{
			"downloadTaskId": task.DownloadTaskID,
		}, &st); err != nil {
			return nil, fmt.Errorf("查询暂存状态失败: %w", err)
		}
		if st.Status == daemonStatusSuccess {
			if st.AppName == "" || st.Version == "" {
				return nil, fmt.Errorf("暂存完成但 app center 未能识别安装包内容")
			}
			return &StagedPackage{
				AppName: st.AppName, Version: st.Version, Name: st.Name,
				PackageType: st.PackageType, Installed: st.Installed, Path: st.Path,
			}, nil
		}
		if st.Status != daemonStatusRunning {
			return nil, fmt.Errorf("暂存安装包失败: 状态 %d %s", st.Status, st.Message)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("暂存安装包超时")
}

// wizardInfo is the subset of the info response we act on. It also carries the
// form definition the app declares, which is what the native App Center renders.
type wizardInfo struct {
	AppName           string          `json:"appName"`
	Version           string          `json:"version"`
	Name              string          `json:"name"`
	InstallType       string          `json:"installType"`
	InstalledType     string          `json:"installedType"`
	InstalledVolumeID int             `json:"installedVolumeID"`
	HasWizard         bool            `json:"hasWizard"`
	WizardContent     json.RawMessage `json:"wizardContent"`
	Docker            bool            `json:"docker"`
}

type infoResponse struct {
	WizardInfo wizardInfo `json:"wizardInfo"`
}

// UpgradeFpk upgrades an ALREADY-INSTALLED app in place, preserving its data.
//
// This is the whole point of the RPC channel: it drives the daemon's upgrade
// operation (which stops the app, swaps the payload and keeps @appdata) instead
// of install-local's uninstall-then-reinstall.
func (a *LinuxAppCenter) UpgradeFpk(ctx context.Context, fpkPath string, params []WizardParam) error {
	staged, err := a.StageFpk(ctx, fpkPath)
	if err != nil {
		return err
	}
	if !staged.Installed {
		return fmt.Errorf("%s 尚未安装，无法执行升级", staged.AppName)
	}

	var info infoResponse
	if err := daemonCall(ctx, routeUpdateInfo, map[string]any{
		"appName":       staged.AppName,
		"updateVersion": staged.Version,
		"packageType":   staged.PackageType,
		"language":      "zh-CN",
	}, &info); err != nil {
		return fmt.Errorf("升级前检查失败: %w", err)
	}

	// Pin to the volume the app already occupies. The daemon reports it, so we
	// never have to consult the broken `appcenter-cli default-volume` getter.
	volume := info.WizardInfo.InstalledVolumeID
	if volume <= 0 {
		return fmt.Errorf("无法确定 %s 当前所在的存储卷，已中止升级以保护现有数据", staged.AppName)
	}

	if params == nil {
		params = []WizardParam{}
	}
	var task struct {
		TaskID string `json:"taskId"`
	}
	if err := daemonCall(ctx, routeUpdateTask, map[string]any{
		"appName":       staged.AppName,
		"updateVersion": staged.Version,
		"packageType":   staged.PackageType,
		"systemParameters": map[string]any{
			"agreedToProtocol": true,
			"installVolumeID":  volume,
			"dataVolumeId":     volume,
			"immediateStart":   false,
		},
		"customParameters": params,
		"language":         "zh-CN",
	}, &task); err != nil {
		return fmt.Errorf("升级失败: %w", err)
	}
	return a.waitTask(ctx, task.TaskID, "升级")
}

type taskStatus struct {
	Status     int     `json:"status"`
	Message    string  `json:"message"`
	Progress   float64 `json:"progress"`
	OutputText string  `json:"outputText"`
}

// waitTask polls a daemon task to completion.
func (a *LinuxAppCenter) waitTask(ctx context.Context, taskID, what string) error {
	if taskID == "" {
		return fmt.Errorf("%s失败: app center 未返回任务 ID", what)
	}
	deadline := time.Now().Add(15 * time.Minute)
	for time.Now().Before(deadline) {
		var st taskStatus
		if err := daemonCall(ctx, routeCommonStatus, map[string]any{"taskId": taskID}, &st); err != nil {
			return fmt.Errorf("查询%s状态失败: %w", what, err)
		}
		switch {
		case st.Status == daemonStatusSuccess:
			return nil
		case st.Status == daemonStatusRunning:
		default:
			detail := st.Message
			if detail == "" {
				detail = st.OutputText
			}
			return fmt.Errorf("%s失败: 状态 %d %s", what, st.Status, detail)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("%s超时", what)
}

// DaemonUpgradeAvailable reports whether the daemon's upgrade channel is
// usable, so the store can prefer it and fall back to refusing rather than
// assuming either outcome from the fnOS version string alone.
func (a *LinuxAppCenter) DaemonUpgradeAvailable() bool {
	if _, err := net.DialTimeout("unix", daemonSocket, 2*time.Second); err != nil {
		return false
	}
	// An empty body must be REJECTED by a route that exists (validation error),
	// and produce a transport/404-shaped failure when it does not.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := daemonCall(ctx, routeUpdateInfo, map[string]any{}, nil)
	var de *DaemonError
	if errors.As(err, &de) {
		return de.Code == daemonCodeValidation
	}
	return false
}
