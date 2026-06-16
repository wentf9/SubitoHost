package api

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/wentf9/subitohost/internal/engine"
	"github.com/wentf9/subitohost/ui"
)

type Server struct {
	engine *engine.Engine
	mux    *http.ServeMux
	server *http.Server
}

func NewServer(e *engine.Engine, addr string) *Server {
	s := &Server{engine: e, mux: http.NewServeMux()}
	s.registerRoutes()
	s.server = &http.Server{Addr: addr, Handler: s.mux}
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/v1/setlist", s.handleGetSetlist)
	s.mux.HandleFunc("PUT /api/v1/setlist", s.handleLoadSetlist)
	s.mux.HandleFunc("POST /api/v1/setlist/next", s.handleNext)
	s.mux.HandleFunc("POST /api/v1/setlist/prev", s.handlePrev)
	s.mux.HandleFunc("POST /api/v1/setlist/goto", s.handleGoTo)
	s.mux.HandleFunc("GET /api/v1/midi/devices", s.handleListDevices)
	s.mux.HandleFunc("POST /api/v1/midi/connect", s.handleConnect)
	s.mux.HandleFunc("GET /api/v1/stream", s.handleWebSocket)
	s.mux.HandleFunc("GET /api/v1/gain", s.handleGetGain)
	s.mux.HandleFunc("PUT /api/v1/gain", s.handleSetGain)
	s.mux.HandleFunc("POST /api/v1/record/start", s.handleRecordStart)
	s.mux.HandleFunc("POST /api/v1/record/stop", s.handleRecordStop)
	s.mux.HandleFunc("GET /api/v1/record/status", s.handleRecordStatus)

	// Frontend SPA routing
	distFS, err := fs.Sub(ui.DistFS, "dist")
	if err != nil {
		log.Printf("UI assets not found: %v", err)
	} else {
		fileServer := http.FileServer(http.FS(distFS))
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Don't intercept API routes
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}

			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			if _, err := fs.Stat(distFS, path); errors.Is(err, fs.ErrNotExist) {
				// Fallback to index.html for SPA
				r.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, r)
		})
	}
}

func (s *Server) Start() error {
	log.Printf("API server listening on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
