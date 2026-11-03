package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fnos-store/internal/config"
)

func TestAccessProtection(t *testing.T) {
	const token = "a-long-test-token-123456789"
	manager := config.NewManager(t.TempDir())
	if err := manager.SaveConfig(config.Config{AccessTokenHash: accessTokenHash(token)}); err != nil {
		t.Fatal(err)
	}
	s := &Server{Mux: http.NewServeMux(), configMgr: manager}
	s.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tc := range []struct {
		name, method, path, user, password, origin, site string
		want                                             int
	}{
		{"document", "GET", "/", "", "", "", "", 401},
		{"download", "GET", "/api/apps/x/download", "", "", "", "", 401},
		{"install", "POST", "/api/apps/x/install", "", "", "", "", 401},
		{"wrong token", "GET", "/api/apps", "fnos", "incorrect", "", "", 401},
		{"wrong user", "GET", "/api/apps", "admin", token, "", "", 401},
		{"authenticated SSE", "GET", "/api/events", "fnos", token, "", "", 204},
		{"same origin", "PUT", "/api/settings", "fnos", token, "http://example.com", "same-origin", 204},
		{"cross origin", "POST", "/api/check", "fnos", token, "http://evil.example", "", 403},
		{"null origin", "POST", "/api/check", "fnos", token, "null", "", 403},
		{"cross site", "POST", "/api/check", "fnos", token, "", "cross-site", 403},
		{"CLI", "POST", "/api/check", "fnos", token, "", "", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://example.com"+tc.path, nil)
			if tc.user != "" {
				r.SetBasicAuth(tc.user, tc.password)
			}
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
			if tc.want == 401 && w.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing browser login challenge")
			}
		})
	}
	if err := manager.SaveConfig(config.Config{}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 204 {
		t.Fatal("disabling access protection did not take effect")
	}
}

func TestSettingsAccessTokenLifecycle(t *testing.T) {
	dir := t.TempDir()
	manager := config.NewManager(dir)
	s := &Server{configMgr: manager, ac: &stubAppCenter{}}
	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handlePutSettings(w, httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body)))
		return w
	}
	if w := put(`{"access_token":"short"}`); w.Code != 400 {
		t.Fatalf("short token: %d", w.Code)
	}
	const token = "my-secret-access-token-123456789"
	if w := put(`{"access_token":"` + token + `"}`); w.Code != 200 {
		t.Fatalf("enable: %d %s", w.Code, w.Body.String())
	}
	hash := manager.Get().AccessTokenHash
	if hash != accessTokenHash(token) {
		t.Fatal("hash not stored")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), token) {
		t.Fatal("plaintext token persisted")
	}
	reloaded := config.NewManager(dir)
	if cfg, err := reloaded.LoadConfig(); err != nil || cfg.AccessTokenHash != hash {
		t.Fatalf("restart lost protection: %v", err)
	}
	if w := put(`{"mirror":"direct"}`); w.Code != 200 || manager.Get().AccessTokenHash != hash {
		t.Fatal("unrelated settings reset protection")
	}
	w := httptest.NewRecorder()
	s.handleGetSettings(w, httptest.NewRequest("GET", "/api/settings", nil))
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["access_protection"] != true || strings.Contains(w.Body.String(), hash) {
		t.Fatal("settings must expose only enabled state")
	}
	if w := put(`{"access_token":""}`); w.Code != 200 || manager.Get().AccessTokenHash != "" {
		t.Fatal("disable failed")
	}
}
