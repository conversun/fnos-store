package api

import (
	"fnos-store/internal/core"
	"fnos-store/internal/platform"
	"net/http"
)

type Server struct {
	Mux      *http.ServeMux
	AC       platform.AppCenter
	Registry *core.Registry
}

func NewServer(ac platform.AppCenter, reg *core.Registry) *Server {
	s := &Server{
		Mux:      http.NewServeMux(),
		AC:       ac,
		Registry: reg,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Mux.HandleFunc("GET /api/apps", s.handleListApps)
	s.Mux.HandleFunc("POST /api/apps/{appname}/install", s.handleInstall)
	s.Mux.HandleFunc("POST /api/apps/{appname}/update", s.handleUpdate)
	s.Mux.HandleFunc("POST /api/apps/{appname}/uninstall", s.handleUninstall)
	s.Mux.HandleFunc("POST /api/check", s.handleCheck)
	s.Mux.HandleFunc("GET /api/events", s.handleSSE)
}
