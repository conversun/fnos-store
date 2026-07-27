//go:build linux

package platform

import (
	"os"
	"strings"
)

// fnosVersionPath is where fnOS records its build, e.g. "1.2.0203".
const fnosVersionPath = "/usr/trim/etc/version"

// UpgradeCapability reports whether this fnOS build can update apps without
// destroying them. See upgrade.go for the measured failure.
func (a *LinuxAppCenter) UpgradeCapability() UpgradeCapability {
	b, err := os.ReadFile(fnosVersionPath)
	if err != nil {
		// Version unknown: allow, and rely on post-install verification. A
		// missing version file is far more likely to be an unusual layout than
		// the specific broken build.
		return UpgradeCapability{Allowed: true}
	}
	version := strings.TrimSpace(string(b))
	if reason, unsafe := upgradeUnsafeVersions[version]; unsafe {
		return UpgradeCapability{
			Allowed:         false,
			PlatformVersion: version,
			Reason: reason + "\n\n请改用飞牛系统自带的「应用中心 → 手动安装」上传 fpk 完成更新" +
				"（切勿先卸载原应用，否则数据同样会丢失）。",
		}
	}
	return UpgradeCapability{Allowed: true, PlatformVersion: version}
}
