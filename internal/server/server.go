package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"

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
	tsCfg    config.Tailscale
	srvCfg   config.Server
	mux      *http.ServeMux

	mu   sync.Mutex
	addr net.Addr
}

// New creates a new Server with the given dependencies.
func New(ag agent.Agent, sess session.Manager, q queue.Queue, srvCfg config.Server, tsCfg config.Tailscale) *Server {
	s := &Server{
		agent:    ag,
		sessions: sess,
		queue:    q,
		tsCfg:    tsCfg,
		srvCfg:   srvCfg,
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

// ServeHTTP implements http.Handler for testing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Addr returns the address the server is listening on, or nil if not yet started.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Serve starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	handler := s.authMiddleware(s.mux)

	srv := &http.Server{
		Handler: handler,
	}

	var ln net.Listener
	var err error

	if s.tsCfg.Enabled {
		// TODO: tsnet listener
		return errors.New("tsnet listener not implemented")
	}

	addr := s.srvCfg.Listen
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	ln, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.addr = ln.Addr()
	s.mu.Unlock()

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
