package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
)

func accessTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ServeHTTP protects every route, including downloads and streaming operations.
// Browser HTTP authentication also works with EventSource and desktop windows.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.gatewayMode {
		if len(r.Header.Values("X-Trim-Userid")) != 1 || strings.TrimSpace(r.Header.Get("X-Trim-Userid")) == "" {
			writeAPIError(w, http.StatusUnauthorized, "请先登录飞牛")
			return
		}
		if len(r.Header.Values("X-Trim-Isadmin")) != 1 || r.Header.Get("X-Trim-Isadmin") != "true" {
			writeAPIError(w, http.StatusForbidden, "需要飞牛管理员权限")
			return
		}
	}
	hash := ""
	if s.configMgr != nil {
		hash = s.configMgr.Get().AccessTokenHash
	}
	if hash != "" {
		username, token, ok := r.BasicAuth()
		if !ok || username != "fnos" || subtle.ConstantTimeCompare([]byte(accessTokenHash(token)), []byte(hash)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="fnOS Apps (username: fnos)", charset="UTF-8"`)
			w.Header().Set("Cache-Control", "no-store")
			writeAPIError(w, http.StatusUnauthorized, "请输入访问令牌（用户名：fnos）")
			return
		}
	}
	if hash != "" || s.gatewayMode {
		// NAS cookies and cached Basic credentials are ambient browser credentials.
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			origin := r.Header.Get("Origin")
			u, err := url.Parse(origin)
			sameHost := err == nil && u.Host == r.Host
			// fnOS nginx forwards Host with $host, dropping the external port.
			// A token fetched from this origin also covers plain HTTP browsers
			// without Fetch Metadata. Hostname alone cannot distinguish ports.
			if s.gatewayMode && err == nil {
				requestURL := url.URL{Host: r.Host}
				sameHost = strings.EqualFold(u.Hostname(), requestURL.Hostname()) &&
					(r.Header.Get("Sec-Fetch-Site") == "same-origin" ||
						(s.gatewayCSRFToken != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-FNOS-CSRF-Token")), []byte(s.gatewayCSRFToken)) == 1))
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" || (origin != "" && (err != nil || !sameHost || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https"))) {
				writeAPIError(w, http.StatusForbidden, "不允许跨站操作")
				return
			}
		}
	}
	if s.gatewayMode {
		s.serveGateway(w, r)
		return
	}
	s.Mux.ServeHTTP(w, r)
}
