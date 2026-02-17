package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"

	"github.com/ivy/kb/internal/agent"
	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/session"
)

// Server is the KB HTTP server.
type Server struct {
	agent    agent.Agent
	sessions session.Manager
	queue    queue.Queue
	cfg      config.Tailscale
	mux      *http.ServeMux
}

// New creates a new Server with the given dependencies.
func New(ag agent.Agent, sess session.Manager, q queue.Queue, cfg config.Tailscale) *Server {
	s := &Server{
		agent:    ag,
		sessions: sess,
		queue:    q,
		cfg:      cfg,
		mux:      http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("POST /v1/ask", s.handleAsk)
	s.mux.HandleFunc("POST /v1/sessions/{id}/ask", s.handleSessionAsk)
	s.mux.HandleFunc("POST /webhook", s.handleWebhook)
	s.mux.HandleFunc("GET /llms.txt", s.handleLLMsTxt)
}

// Serve starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	handler := s.authMiddleware(s.mux)

	srv := &http.Server{
		Handler: handler,
	}

	var ln net.Listener
	var err error

	if s.cfg.Enabled {
		// TODO: tsnet listener
		return errors.New("tsnet listener not implemented")
	}

	ln, err = net.Listen("tcp", ":8080")
	if err != nil {
		return err
	}

	slog.Info("server listening", "addr", ln.Addr())

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down server")
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}
