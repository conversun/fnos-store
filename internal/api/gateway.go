package api

import (
	"net/http"
	"strings"
)

const gatewayPrefix = "/app/fnos-apps-store"

func (s *Server) handleCSRFToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	writeJSON(w, http.StatusOK, map[string]string{"csrf_token": s.gatewayCSRFToken})
}

// serveGateway handles the prefix fnOS preserves when forwarding over Unix HTTP.
// It runs only on the explicitly selected socket listener, after authorization.
func (s *Server) serveGateway(w http.ResponseWriter, r *http.Request) {
	escaped := r.URL.EscapedPath()
	lower := strings.ToLower(escaped)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") || strings.Contains(r.URL.Path, "\\") || strings.Contains(r.URL.Path, "//") {
		writeAPIError(w, http.StatusBadRequest, "无效的网关路径")
		return
	}
	for segment := range strings.SplitSeq(r.URL.Path, "/") {
		if segment == "." || segment == ".." {
			writeAPIError(w, http.StatusBadRequest, "无效的网关路径")
			return
		}
	}
	if escaped == gatewayPrefix {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		location := gatewayPrefix + "/"
		if r.URL.RawQuery != "" {
			location += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, location, http.StatusPermanentRedirect)
		return
	}
	if !strings.HasPrefix(escaped, gatewayPrefix+"/") {
		http.NotFound(w, r)
		return
	}
	forwarded := r.Clone(r.Context())
	forwarded.URL.Path = strings.TrimPrefix(r.URL.Path, gatewayPrefix)
	if r.URL.RawPath != "" {
		forwarded.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, gatewayPrefix)
	}
	s.Mux.ServeHTTP(w, forwarded)
}
