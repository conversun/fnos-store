package api

import (
	"fnos-store/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestGatewayAuthorization(t *testing.T) {
	const token = "gateway-test-token-123456789"
	for _, tc := range []struct {
		name, method, path, uid, admin, password, origin, site string
		protect, standalone                                    bool
		want                                                   int
	}{
		{name: "missing identity", path: "/api/apps", want: 401},
		{name: "whitespace identity", path: "/", uid: " ", admin: "true", want: 401},
		{name: "ordinary user", path: "/api/apps", uid: "1001", admin: "false", want: 403},
		{name: "missing admin", path: "/api/apps", uid: "1000", want: 403},
		{name: "malformed admin", path: "/api/apps", uid: "1000", admin: "TRUE", want: 403},
		{name: "admin", path: "/api/apps", uid: "1000", admin: "true", want: 204},
		{name: "download denied", path: "/api/apps/x/download", uid: "1001", admin: "false", want: 403},
		{name: "diagnostic denied", path: "/api/apps/x/diagnostic", uid: "1001", admin: "false", want: 403},
		{name: "stream denied", method: "POST", path: "/api/apps/x/install", want: 401},
		{name: "asset denied", path: "/assets/index.js", want: 401},
		{name: "same origin", method: "PUT", path: "/api/settings", uid: "1000", admin: "true", origin: "https://nas.test:5667", site: "same-origin", want: 204},
		{name: "cross origin tokenless", method: "POST", path: "/api/check", uid: "1000", admin: "true", origin: "https://evil.test", want: 403},
		{name: "null origin", method: "POST", path: "/api/check", uid: "1000", admin: "true", origin: "null", want: 403},
		{name: "cross site tokenless", method: "POST", path: "/api/check", uid: "1000", admin: "true", site: "cross-site", want: 403},
		{name: "CLI admin", method: "POST", path: "/api/check", uid: "1000", admin: "true", want: 204},
		{name: "token required for admin", path: "/api/apps", uid: "1000", admin: "true", protect: true, want: 401},
		{name: "wrong token", path: "/api/apps", uid: "1000", admin: "true", password: "wrong", protect: true, want: 401},
		{name: "token cannot grant admin", path: "/api/apps", uid: "1001", admin: "false", password: token, protect: true, want: 403},
		{name: "both credentials", path: "/api/apps", uid: "1000", admin: "true", password: token, protect: true, want: 204},
		{name: "both credentials cross origin", method: "POST", path: "/api/check", uid: "1000", admin: "true", password: token, protect: true, origin: "https://evil.test", want: 403},
		{name: "standalone ignores gateway headers", path: "/api/apps", uid: "1001", admin: "false", standalone: true, want: 204},
		{name: "standalone headers cannot bypass token", path: "/api/apps", uid: "1000", admin: "true", protect: true, standalone: true, want: 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{Mux: http.NewServeMux(), gatewayMode: !tc.standalone}
			if tc.protect {
				s.configMgr = config.NewManager(t.TempDir())
				if err := s.configMgr.SaveConfig(config.Config{AccessTokenHash: accessTokenHash(token)}); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			s.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})
			method := tc.method
			if method == "" {
				method = "GET"
			}
			path := tc.path
			if !tc.standalone {
				path = "/app/fnos-apps-store" + path
			}
			r := httptest.NewRequest(method, "https://nas.test:5667"+path, nil)
			r.Header.Set("X-Trim-Userid", tc.uid)
			r.Header.Set("X-Trim-Isadmin", tc.admin)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			if tc.password != "" {
				r.SetBasicAuth("fnos", tc.password)
			}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want || called != (tc.want == 204) {
				t.Fatalf("status=%d dispatched=%v; want status=%d", w.Code, called, tc.want)
			}
		})
	}
}

func TestGatewayPaths(t *testing.T) {
	s := &Server{
		Mux: http.NewServeMux(), gatewayMode: true,
		staticFS: fstest.MapFS{
			"web/index.html":     {Data: []byte("<html>store</html>")},
			"web/assets/test.js": {Data: []byte("window.store=true;")},
		},
	}
	s.Mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"query": r.URL.RawQuery, "host": r.Host})
	})
	s.Mux.HandleFunc("/", s.handleSPA)
	for _, tc := range []struct {
		path, method   string
		want           int
		body, location string
	}{
		{"/app/fnos-apps-store", "GET", 308, "", "/app/fnos-apps-store/"},
		{"/app/fnos-apps-store?x=a%2Fb", "HEAD", 308, "", "/app/fnos-apps-store/?x=a%2Fb"},
		{"/app/fnos-apps-store/", "GET", 200, "<html>store</html>", ""},
		{"/app/fnos-apps-store/assets/test.js", "GET", 200, "window.store=true;", ""},
		{"/app/fnos-apps-store/api/status?x=a%2F%2Fb", "GET", 200, `"query":"x=a%2F%2Fb"`, ""},
		{"/app/fnos-apps-store/api/missing", "GET", 404, "", ""},
		{"/api/status", "GET", 404, "", ""},
		{"/app/fnos-apps-storeevil/api/status", "GET", 404, "", ""},
		{"/app/other/api/status", "GET", 404, "", ""},
		{"/app/fnos-apps-store/api/../status", "GET", 400, "", ""},
		{"/app/fnos-apps-store/%2e%2e/secret", "GET", 400, "", ""},
		{"/app/fnos-apps-store/api%2fstatus", "GET", 400, "", ""},
		{"/app/fnos-apps-store/api%5cstatus", "GET", 400, "", ""},
		{"/app/fnos-apps-store//api/status", "GET", 400, "", ""},
		{"/app/fnos-apps-store%2fapi/status", "GET", 400, "", ""},
		{"/%61pp/fnos-apps-store/api/status", "GET", 404, "", ""},
		{"/app/fnos-apps-store/app/fnos-apps-store/api/status", "GET", 200, "<html>store</html>", ""},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://nas.test"+tc.path, nil)
			originalPath := r.URL.Path
			r.Header.Set("X-Trim-Userid", "1000")
			r.Header.Set("X-Trim-Isadmin", "true")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.want || w.Header().Get("Location") != tc.location {
				t.Fatalf("status=%d location=%q; want %d %q", w.Code, w.Header().Get("Location"), tc.want, tc.location)
			}
			if tc.body != "" && !strings.Contains(w.Body.String(), tc.body) {
				t.Fatalf("body=%q; want %q", w.Body.String(), tc.body)
			}
			if r.URL.Path != originalPath {
				t.Fatal("gateway mutated caller URL")
			}
		})
	}
}
