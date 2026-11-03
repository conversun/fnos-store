package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
)

func accessTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ServeHTTP protects every route, including downloads and streaming operations.
// Browser HTTP authentication also works with EventSource and desktop windows.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		// Browsers cache Basic credentials. Reject cross-site mutations so
		// another website cannot act with those ambient credentials.
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			origin := r.Header.Get("Origin")
			u, err := url.Parse(origin)
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" || (origin != "" && (err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https"))) {
				writeAPIError(w, http.StatusForbidden, "不允许跨站操作")
				return
			}
		}
	}
	s.Mux.ServeHTTP(w, r)
}
