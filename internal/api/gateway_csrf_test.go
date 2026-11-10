package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayCSRFWithPortStrippedHost(t *testing.T) {
	for _, tc := range []struct {
		name, origin, site, token string
		want                      int
	}{
		{name: "same origin browser", origin: "http://nas.test:5666", site: "same-origin", want: 204},
		{name: "HTTPS same origin browser", origin: "https://nas.test:5667", site: "same-origin", want: 204},
		{name: "foreign hostname despite metadata", origin: "https://evil.test", site: "same-origin", want: 403},
		{name: "different port same site", origin: "http://nas.test:7777", site: "same-site", want: 403},
		{name: "different port missing metadata", origin: "http://nas.test:7777", want: 403},
		{name: "portless origin missing metadata", origin: "http://nas.test", want: 403},
		{name: "HTTP browser token", origin: "http://nas.test:5666", token: "test-csrf-token", want: 204},
		{name: "wrong token", origin: "http://nas.test:5666", token: "wrong", want: 403},
		{name: "foreign hostname with token", origin: "http://evil.test:5666", token: "test-csrf-token", want: 403},
		{name: "cross site cannot use token", origin: "http://nas.test:5666", site: "cross-site", token: "test-csrf-token", want: 403},
		{name: "malformed origin", origin: "http://nas.test:5666/path", site: "same-origin", want: 403},
		{name: "null origin", origin: "null", want: 403},
		{name: "CLI without origin", want: 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{Mux: http.NewServeMux(), gatewayMode: true, gatewayCSRFToken: "test-csrf-token"}
			s.Mux.HandleFunc("POST /api/check", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			req := httptest.NewRequest("POST", "http://nas.test/app/fnos-apps-store/api/check", nil)
			req.Header.Set("X-Trim-Userid", "1000")
			req.Header.Set("X-Trim-Isadmin", "true")
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Sec-Fetch-Site", tc.site)
			req.Header.Set("X-FNOS-CSRF-Token", tc.token)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s; want %d", w.Code, w.Body.String(), tc.want)
			}
		})
	}
}

func TestGatewayCSRFTokenEndpoint(t *testing.T) {
	s := &Server{Mux: http.NewServeMux(), gatewayMode: true, gatewayCSRFToken: "endpoint-token"}
	s.routes()
	s.Mux.HandleFunc("PUT /api/csrf-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	request := func(method, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://nas.test/app/fnos-apps-store"+path, nil)
		r.Header.Set("X-Trim-Userid", "1000")
		r.Header.Set("X-Trim-Isadmin", "true")
		if method == "PUT" {
			r.Header.Set("Origin", "http://nas.test:5666")
		}
		r.Header.Set("X-FNOS-CSRF-Token", token)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	response := request("GET", "/api/csrf", "")
	if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("token response status=%d headers=%v", response.Code, response.Header())
	}
	var body struct {
		Token string `json:"csrf_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatalf("token body=%s err=%v", response.Body.String(), err)
	}
	if w := request("PUT", "/api/csrf-fixture", body.Token); w.Code != 204 {
		t.Fatalf("HTTP mutation with fetched token: %d %s", w.Code, w.Body.String())
	}
	for _, admin := range []string{"", "false"} {
		r := httptest.NewRequest("GET", "http://nas.test/app/fnos-apps-store/api/csrf", nil)
		if admin != "" {
			r.Header.Set("X-Trim-Userid", "1001")
			r.Header.Set("X-Trim-Isadmin", admin)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 401 && w.Code != 403 {
			t.Fatalf("token exposed without administrator identity: %d", w.Code)
		}
	}
}
