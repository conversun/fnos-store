package platform

// UpgradeCapability reports whether in-store updates can run safely on this
// fnOS build.
//
// Background: fnOS offers no data-preserving upgrade command. Measured on the
// CLI's own help output, the full command set is install / uninstall / start /
// stop / check / status / list / install-fpk / install-local / manual-install /
// default-volume — nothing that updates an installed app in place.
//
//   - `install-fpk` REFUSES an already-installed app ("[Info]Application [x] is
//     installed.") and changes nothing. Verified safe but useless for updating.
//   - `install-local` implements an upgrade as uninstall-then-reinstall.
//
// On fnOS 1.2.0203 that reinstall reliably fails with error 10237
// (fork/exec .../cmd/install_init: permission denied — the daemon chowns its
// staging dir to the app user and strips the directory's execute bit), so the
// app is uninstalled and never comes back. Reproduced twice on a live box:
// gopeed went from running to gone, program directory AND @appdata deleted.
// That is the data loss behind conversun/fnos-apps#189.
//
// The store cannot work around it: source-directory permissions make no
// difference and fnOS's own download directory triggers it too. So updates are
// refused on affected builds rather than attempted and lost.
type UpgradeCapability struct {
	Allowed         bool
	PlatformVersion string
	Reason          string
}

// upgradeUnsafeVersions are fnOS builds measured to destroy an app on update.
//
// A deny-list rather than an allow-list is a deliberate trade-off: an
// allow-list would block updates on every build we have not personally
// tested, including ones where updates work today. We only refuse where the
// destruction is confirmed, and the post-update verification
// (verifyPayloadLanded) still catches silent failure elsewhere.
var upgradeUnsafeVersions = map[string]string{
	"1.2.0203": "该 fnOS 版本的应用更新通道存在已确认的数据删除风险（错误 10237）：" +
		"系统会先卸载旧版再安装新版，而安装步骤必定失败，导致应用与其数据一并丢失。",
}
