package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFailedSavePreservesLiveConfig(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	old := Config{AccessTokenHash: "original"}
	if err := m.SaveConfig(old); err != nil {
		t.Fatal(err)
	}
	// A directory in place of the file forces the atomic rename to fail,
	// even when tests run as root.
	if err := os.Remove(m.filePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(m.filePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := m.SaveConfig(Config{AccessTokenHash: "new"}); err == nil {
		t.Fatal("save should fail")
	}
	if m.Get().AccessTokenHash != "original" {
		t.Fatal("failed save changed live token")
	}
	files, err := filepath.Glob(filepath.Join(dir, ".config-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files remain: %v %v", files, err)
	}
}

func TestConfigFileIsPrivate(t *testing.T) {
	m := NewManager(t.TempDir())
	if err := m.SaveConfig(Config{}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(m.filePath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("permissions: %v", st.Mode())
	}
}
