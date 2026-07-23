package api

import (
	"os"
	"path/filepath"
	"testing"

	"fnos-store/internal/core"
	"fnos-store/internal/source"
)

func TestStoreVersion(t *testing.T) {
	const appName = "fnos-apps-store"

	testCases := []struct {
		name            string
		registryVersion string
		manifestBody    string
		remoteVersion   string
		remoteFpk       string
		want            string
	}{
		{
			name:            "registry installed version wins",
			registryVersion: "1.2.3",
			manifestBody:    "version = 1.4.5\nfpk_version = 1.4.5-fpk\n",
			remoteVersion:   "9.9.9",
			remoteFpk:       "9.9.9-fpk",
			want:            "1.2.3",
		},
		{
			name:          "manifest version is next fallback",
			manifestBody:  "version = 1.4.5\n",
			remoteVersion: "9.9.9",
			remoteFpk:     "9.9.9-fpk",
			want:          "1.4.5",
		},
		{
			name:          "manifest fpk version is next fallback",
			manifestBody:  "fpk_version = 1.4.5-fpk\n",
			remoteVersion: "9.9.9",
			remoteFpk:     "9.9.9-fpk",
			want:          "1.4.5-fpk",
		},
		{
			name:          "dev is final fallback",
			remoteVersion: "9.9.9",
			remoteFpk:     "9.9.9-fpk",
			want:          "dev",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			appsDir := t.TempDir()
			if tc.manifestBody != "" {
				manifestDir := filepath.Join(appsDir, appName)
				if err := os.MkdirAll(manifestDir, 0o755); err != nil {
					t.Fatalf("mkdir manifest dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(manifestDir, "manifest"), []byte("appname = "+appName+"\n"+tc.manifestBody), 0o644); err != nil {
					t.Fatalf("write manifest: %v", err)
				}
			}

			registry := core.NewRegistry()
			var localManifests []core.Manifest
			if tc.registryVersion != "" {
				localManifests = []core.Manifest{{AppName: appName, Version: tc.registryVersion}}
			}
			registry.Merge(localManifests, []source.RemoteApp{{
				AppName:     appName,
				DisplayName: "fnOS Apps",
				Version:     tc.remoteVersion,
				FpkVersion:  tc.remoteFpk,
			}}, nil)

			s := &Server{
				registry: registry,
				appsDir:  appsDir,
				storeApp: appName,
			}

			if got := s.storeVersion(); got != tc.want {
				t.Fatalf("storeVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}
