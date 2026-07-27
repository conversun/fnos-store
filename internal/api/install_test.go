package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"fnos-store/internal/core"
	"fnos-store/internal/source"
)

// TestHandleInstallRejectsInstalledApp locks the guard that keeps an
// already-installed app out of the /install path.
//
// install-local treats a reinstall as an UPGRADE: it uninstalls the existing
// copy before reinstalling. runStandard() only pins the target volume for the
// "update" operation, so an installed app routed through "install" would reach
// that destructive step WITHOUT the volume guard that protects its data
// (conversun/fnos-apps#189). /update is the only safe route for an upgrade.
func TestHandleInstallRejectsInstalledApp(t *testing.T) {
	const appName = "qbittorrent"

	newServer := func(t *testing.T, installed bool) *Server {
		t.Helper()
		registry := core.NewRegistry()
		var local []core.Manifest
		if installed {
			local = []core.Manifest{{AppName: appName, Version: "5.2.3"}}
		}
		registry.Merge(local, []source.RemoteApp{{
			AppName:     appName,
			DisplayName: "qBittorrent",
			Version:     "5.2.4",
		}}, nil)
		return &Server{registry: registry, appsDir: t.TempDir(), queue: NewOperationQueue()}
	}

	doInstall := func(t *testing.T, s *Server, name string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/apps/"+name+"/install", nil)
		req.SetPathValue("appname", name)
		rec := httptest.NewRecorder()
		s.handleInstall(rec, req)
		return rec
	}

	t.Run("rejects an app that is already installed", func(t *testing.T) {
		rec := doInstall(t, newServer(t, true), appName)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		if !strings.Contains(rec.Body.String(), "已安装") {
			t.Errorf("body %q should explain the app is already installed", rec.Body.String())
		}
	})

	t.Run("still rejects an unknown app with 404", func(t *testing.T) {
		rec := doInstall(t, newServer(t, false), "nosuchapp")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("requires an appname", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/apps//install", nil)
		rec := httptest.NewRecorder()
		newServer(t, false).handleInstall(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

// TestParseWizardParams locks how the browser's install-wizard answers reach
// the daemon. Malformed or absent input must degrade to "no params" — which
// installs with defaults, exactly as before wizards existed — rather than
// failing the install.
func TestParseWizardParams(t *testing.T) {
	mk := func(query string) *http.Request {
		return httptest.NewRequest(http.MethodPost, "/api/apps/x/install?"+query, nil)
	}

	t.Run("parses answers", func(t *testing.T) {
		q := url.Values{"wizard": {`[{"key":"wizard_token","value":"secret"}]`}}.Encode()
		got := parseWizardParams(mk(q))
		if len(got) != 1 || got[0].Key != "wizard_token" || got[0].Value != "secret" {
			t.Fatalf("params = %+v, want one wizard_token=secret", got)
		}
	})

	t.Run("absent means no params", func(t *testing.T) {
		if got := parseWizardParams(mk("")); got != nil {
			t.Errorf("params = %+v, want nil", got)
		}
	})

	t.Run("malformed degrades to no params, not an error", func(t *testing.T) {
		q := url.Values{"wizard": {`{not json`}}.Encode()
		if got := parseWizardParams(mk(q)); got != nil {
			t.Errorf("params = %+v, want nil so the install still proceeds", got)
		}
	})
}
