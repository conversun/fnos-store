package platform

import (
	"os"
	"path/filepath"
)

// EnsureHooksExecutable restores the executable bit on the cmd/ lifecycle
// hooks of an extracted fpk. fnOS fork/execs them directly, so a hook shipped
// without +x fails the install with the opaque "code 10237"
// (fork/exec ...: permission denied) — and it fails AFTER install-local's
// destructive uninstall phase has already run.
//
// Several published fpks carry install_init, uninstall_init, upgrade_init and
// the matching *_callback hooks as mode 0644 while their byte-identical
// siblings config_init/config_callback are 0755 — an upstream packaging slip.
// Fixing the source only helps future builds; normalizing at extraction time
// makes every fpk already in the wild installable.
//
// Best-effort by design: a chmod failure must not block an install that might
// Symlinks are refused at both levels. `cmd` itself must be a real directory,
// and only regular files inside it are touched — os.Chmod FOLLOWS symlinks, so
// a crafted fpk containing `cmd -> /etc` or `cmd/x -> /etc/shadow` could
// otherwise change permissions on files outside the extraction root.
//
// Best-effort by design: a chmod failure must not block an install that might
// still succeed, and the CLI surfaces the real error if it does not.
func EnsureHooksExecutable(dir string) {
	cmdDir := filepath.Join(dir, "cmd")
	// Lstat, not Stat: a symlinked cmd/ must be refused, not followed.
	if fi, err := os.Lstat(cmdDir); err != nil || !fi.IsDir() {
		return
	}
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		// Regular files only: skips dirs, symlinks, devices and sockets.
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Mode()&0o111 != 0 {
			continue
		}
		// Mirror the read bits rather than forcing 0755: a 0600 hook becomes
		// 0700, not world-readable.
		mode := info.Mode().Perm()
		if mode&0o400 != 0 {
			mode |= 0o100
		}
		if mode&0o040 != 0 {
			mode |= 0o010
		}
		if mode&0o004 != 0 {
			mode |= 0o001
		}
		_ = os.Chmod(filepath.Join(cmdDir, e.Name()), mode)
	}
}
