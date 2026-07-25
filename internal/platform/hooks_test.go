package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEnsureHooksExecutable locks the workaround for the packaging slip that
// makes fnOS fail an install with "code 10237" — fork/exec of a hook that has
// no executable bit — AFTER install-local has already uninstalled the app.
//
// Observed in a real fpk: install_init / uninstall_init / upgrade_init and the
// *_callback hooks were 0644, while byte-identical config_init/config_callback
// were 0755.
func TestEnsureHooksExecutable(t *testing.T) {
	t.Run("adds the executable bit to non-executable hooks", func(t *testing.T) {
		dir := t.TempDir()
		cmdDir := filepath.Join(dir, "cmd")
		if err := os.MkdirAll(cmdDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Mirror the observed real-world mix.
		nonExec := []string{"install_init", "uninstall_init", "upgrade_init", "install_callback"}
		alreadyExec := []string{"common", "config_init", "installer"}
		for _, n := range nonExec {
			if err := os.WriteFile(filepath.Join(cmdDir, n), []byte("#!/bin/sh\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for _, n := range alreadyExec {
			if err := os.WriteFile(filepath.Join(cmdDir, n), []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		EnsureHooksExecutable(dir)

		for _, n := range append(nonExec, alreadyExec...) {
			info, err := os.Stat(filepath.Join(cmdDir, n))
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode()&0o111 == 0 {
				t.Errorf("hook %s mode = %v, want executable", n, info.Mode())
			}
		}
	})

	t.Run("tolerates a missing cmd directory", func(t *testing.T) {
		// A docker-only fpk has no cmd/ hooks; this must not panic.
		EnsureHooksExecutable(t.TempDir())
	})

	t.Run("ignores subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "cmd", "nested")
		if err := os.MkdirAll(sub, 0o700); err != nil {
			t.Fatal(err)
		}
		EnsureHooksExecutable(dir)
		info, err := os.Stat(sub)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Errorf("subdir mode = %v, want unchanged 0700", info.Mode().Perm())
		}
	})
}
