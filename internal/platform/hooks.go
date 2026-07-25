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
// still succeed, and the CLI surfaces the real error if it does not.
func EnsureHooksExecutable(dir string) {
	cmdDir := filepath.Join(dir, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Mode()&0o111 != 0 {
			continue
		}
		_ = os.Chmod(filepath.Join(cmdDir, e.Name()), info.Mode()|0o755)
	}
}
