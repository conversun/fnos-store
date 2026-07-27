//go:build linux

package platform

import (
	"os"
	"strings"
)

// fnosVersionPath is where fnOS records its build, e.g. "1.2.0203".
const fnosVersionPath = "/usr/trim/etc/version"

// UpgradeCapability reports whether this system can update an installed app
// without destroying it.
//
// The answer is a runtime probe, not a version lookup. Two upgrade mechanisms
// exist and they behave very differently:
//
//   - The app-center daemon's own RPC upgrade (Operation.Upgrade, with
//     rollback). Data-preserving. Verified on fnOS 1.2.0203: beszel r1 -> r2
//     with a canary file in @appdata surviving byte-identical, and the daemon
//     logging class=upgrade rather than an uninstall/install pair.
//   - `appcenter-cli install-local`, which is uninstall-then-reinstall. On
//     1.2.0203 the reinstall always fails (error 10237) and the app plus its
//     @appdata are gone.
//
// So the question is not "is this fnOS version safe" but "can we reach the
// daemon's upgrade channel". If we can, updates are safe regardless of the
// version string. If we cannot, refuse rather than fall back to the destroyer
// (conversun/fnos-apps#189).
func (a *LinuxAppCenter) UpgradeCapability() UpgradeCapability {
	version := ""
	if b, err := os.ReadFile(fnosVersionPath); err == nil {
		version = strings.TrimSpace(string(b))
	}

	if a.DaemonUpgradeAvailable() {
		return UpgradeCapability{Allowed: true, PlatformVersion: version}
	}

	reason := "无法连接飞牛应用中心的升级服务，为避免退回到会删除应用数据的旧升级方式，已中止本次更新。"
	if known, unsafe := upgradeUnsafeVersions[version]; unsafe {
		reason = known
	}
	return UpgradeCapability{
		Allowed:         false,
		PlatformVersion: version,
		Reason: reason + "\n\n请改用飞牛系统自带的「应用中心 → 手动安装」上传 fpk 完成更新" +
			"（切勿先卸载原应用，否则数据同样会丢失）。",
	}
}
