package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fnos-store/internal/config"
	"fnos-store/internal/core"
	"fnos-store/internal/platform"
	"fnos-store/internal/source"
)

type cleanupListFailure struct{ *stubAppCenter }

func (cleanupListFailure) List() ([]platform.InstalledApp, error) {
	return nil, errors.New("appcenter unavailable")
}

func TestCleanupRefusesWhenRegisteredStateIsUnknown(t *testing.T) {
	srv := newCleanupServer(t, nil)
	srv.ac = cleanupListFailure{&stubAppCenter{}}
	residue := filepath.Join(srv.appsDir, "stale-app", "manifest")
	if err := os.MkdirAll(filepath.Dir(residue), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(residue, []byte("appname = stale-app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := doCleanup(srv, "stale-app")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(residue); err != nil {
		t.Fatalf("residue was changed while appcenter was unavailable: %v", err)
	}
}

// newCleanupServer builds the minimal Server the cleanup path needs. source
// stays nil on purpose: refreshRegistry then fails and the handler answers
// 200-with-warning, which keeps the residue assertions free of remote
// catalog plumbing.
func newCleanupServer(t *testing.T, daemonApps []platform.InstalledApp) *Server {
	t.Helper()
	return &Server{
		ac:        &stubAppCenter{listResult: daemonApps},
		appsDir:   t.TempDir(),
		queue:     NewOperationQueue(),
		configMgr: config.NewManager(t.TempDir()),
	}
}

func doCleanup(s *Server, appname string) *httptest.ResponseRecorder {
	// The URL stays benign: invalid names (spaces, slashes) would break
	// NewRequest's parser before the handler ever runs. SetPathValue feeds
	// the handler exactly what the route would have.
	req := httptest.NewRequest(http.MethodPost, "/api/apps/x/cleanup", nil)
	req.SetPathValue("appname", appname)
	rec := httptest.NewRecorder()
	s.handleCleanup(rec, req)
	return rec
}

// The handler deletes directories as root: an appname that could escape the
// fixed prefixes (traversal, separators) must be rejected before anything is
// touched on disk.
func TestCleanupRejectsInvalidAppName(t *testing.T) {
	for _, name := range []string{"../evil", "a/b", ".hidden", "", "sp ace"} {
		rec := doCleanup(newCleanupServer(t, nil), name)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("appname %q: status = %d, want 400", name, rec.Code)
		}
	}
}

// Cleanup exists ONLY for apps app center no longer knows; a registered app
// must go through the normal uninstall so its hooks run.
func TestCleanupRefusesRegisteredApp(t *testing.T) {
	s := newCleanupServer(t, []platform.InstalledApp{{AppName: "clouddrive2", Version: "1.0.20"}})

	rec := doCleanup(s, "clouddrive2")

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for a daemon-registered app", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "请使用卸载") {
		t.Errorf("body = %s, want it to point at the uninstall action", rec.Body.String())
	}
}

// The happy path: scan says installed, daemon says unknown — the exact
// #312 dead-end. Residue under appsDir must be gone and reported.
func TestCleanupRemovesResidue(t *testing.T) {
	s := newCleanupServer(t, nil)
	residue := filepath.Join(s.appsDir, "clouddrive2", "manifest")
	if err := os.MkdirAll(filepath.Dir(residue), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(residue, []byte("appname = clouddrive2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := doCleanup(s, "clouddrive2")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(residue); !os.IsNotExist(err) {
		t.Errorf("residue still present after cleanup")
	}
	if !strings.Contains(rec.Body.String(), "removed") {
		t.Errorf("body = %s, want the removed list", rec.Body.String())
	}
}

// Every path the cleanup can touch must sit under the fixed prefixes and
// reference the appname — the reviewable statement of the path-safety
// contract. The appsDir residue is created so at least one glob match exists
// (Glob only returns paths that are actually present).
func TestResiduePathsStayUnderFixedPrefixes(t *testing.T) {
	appsDir := t.TempDir()
	residue := filepath.Join(appsDir, "clouddrive2", "manifest")
	if err := os.MkdirAll(filepath.Dir(residue), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(residue, []byte("appname = clouddrive2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths := residuePaths(appsDir, "clouddrive2")
	if len(paths) == 0 {
		t.Fatal("expected the appsDir residue to be discovered")
	}
	for _, p := range paths {
		if !strings.HasPrefix(p, appsDir) && !strings.HasPrefix(p, "/vol") {
			t.Errorf("path %q escapes the fixed prefixes", p)
		}
		if !strings.Contains(p, "clouddrive2") {
			t.Errorf("path %q does not reference the appname", p)
		}
	}
}

// half_registered must be true exactly when the scan marks an app installed
// while the daemon's (loaded, non-empty) list does not contain it — and never
// when the daemon list is unavailable.
func TestHalfRegisteredFlag(t *testing.T) {
	const appName = "clouddrive2"
	newSrv := func(statusByApp map[string]string) *Server {
		registry := core.NewRegistry()
		registry.Merge([]core.Manifest{{AppName: appName, Version: "1.0.20"}},
			[]source.RemoteApp{{AppName: appName, DisplayName: "CloudDrive2", Version: "1.0.20"}}, nil)
		return &Server{
			registry:    registry,
			ac:          &stubAppCenter{},
			configMgr:   config.NewManager(t.TempDir()),
			queue:       NewOperationQueue(),
			appsDir:     t.TempDir(),
			statusByApp: statusByApp,
		}
	}

	cases := []struct {
		name        string
		statusByApp map[string]string
		want        bool
	}{
		{"daemon does not know the app", map[string]string{"other": "running"}, true},
		{"daemon knows the app", map[string]string{appName: "stopped"}, false},
		{"daemon list unavailable", map[string]string{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSrv(tc.statusByApp)
			req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
			rec := httptest.NewRecorder()
			s.handleListApps(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			body := rec.Body.String()
			has := strings.Contains(body, `"half_registered":true`)
			if tc.want && !has {
				t.Errorf("response lacks half_registered:true: %s", body[:min(200, len(body))])
			}
			if !tc.want && has {
				t.Errorf("response must not mark half_registered: %s", body[:min(200, len(body))])
			}
		})
	}
}

// A stale registry can offer the CURRENT version as the store update; the
// self-update must refuse instead of re-installing the same bytes (observed
// live during the 1.9.6 rollout).
func TestSelfUpdateRefusesSameVersion(t *testing.T) {
	const storeApp = "fnos-apps-store"
	registry := core.NewRegistry()
	registry.Merge([]core.Manifest{{AppName: storeApp, Version: "1.9.6"}},
		[]source.RemoteApp{{AppName: storeApp, DisplayName: "fnOS Apps", Version: "1.9.6", FpkVersion: "1.9.6"}}, nil)
	s := &Server{
		registry:  registry,
		storeApp:  storeApp,
		appsDir:   t.TempDir(),
		queue:     NewOperationQueue(),
		configMgr: config.NewManager(t.TempDir()),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/store-update", nil)
	rec := httptest.NewRecorder()
	s.handlePostStoreUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 refusing a same-version self-update", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "已是最新版本") {
		t.Errorf("body = %s, want the already-latest refusal", rec.Body.String())
	}
}
