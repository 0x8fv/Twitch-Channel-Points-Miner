package dashboard

import (
	"net/http"
	"time"
)

// Server serves the dashboard UI and its JSON API.
type Server struct {
	store *Store
}

func NewServer(store *Store) *Server {
	return &Server{store: store}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /api/streamers", s.handleStreamers)
	mux.HandleFunc("GET /api/streamer/{name}", s.handleStreamer)
	return mux
}

// ListenAndServe blocks serving the dashboard on addr.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	return srv.ListenAndServe()
}
