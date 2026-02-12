package api

import "net/http"

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"apps":[]}`))
}
