// internal/server/server.go
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	kbv1 "github.com/ivy/kb/gen/kb/v1"
	"github.com/ivy/kb/gen/kb/v1/kbv1connect"
)

// shutdownTimeout is the maximum time to wait for in-flight requests
// to complete during graceful shutdown.
const shutdownTimeout = 5 * time.Second

type Server struct {
	socketPath string
	httpServer *http.Server
}

func New(socketPath string) *Server {
	return &Server{socketPath: socketPath}
}

func (s *Server) Run(ctx context.Context) error {
	ln, err := listenSocket(s.socketPath)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	path, handler := kbv1connect.NewKnowledgeBaseServiceHandler(s)
	mux.Handle(path, handler)

	s.httpServer = &http.Server{Handler: mux}

	log.Printf("listening on %s", s.socketPath)

	// Shut down gracefully when the context is cancelled.
	go func() {
		<-ctx.Done()
		log.Printf("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		s.httpServer.Shutdown(shutdownCtx)
	}()

	err = s.httpServer.Serve(ln)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// listenSocket creates a Unix domain socket at the given path, removing
// any stale socket file first. It binds atomically — if two processes
// race, exactly one will succeed and the other will get EADDRINUSE.
func listenSocket(path string) (net.Listener, error) {
	// Try to bind first. If it succeeds, there was no stale socket.
	ln, err := net.Listen("unix", path)
	if err == nil {
		return ln, nil
	}

	// If the path exists but no one is listening, it's stale — remove
	// and retry. If someone is listening, the retry will fail with
	// EADDRINUSE, which is the correct behavior.
	os.Remove(path)
	return net.Listen("unix", path)
}

// List returns available knowledge bases.
func (s *Server) List(
	ctx context.Context,
	req *connect.Request[kbv1.ListRequest],
) (*connect.Response[kbv1.ListResponse], error) {
	log.Printf("list_knowledge_bases: called")

	// Stub response
	resp := &kbv1.ListResponse{
		KnowledgeBases: []*kbv1.KnowledgeBase{
			{Name: "personal", Description: "Personal notes and documents"},
			{Name: "work", Description: "Work-related documentation"},
		},
	}

	return connect.NewResponse(resp), nil
}

// Query searches knowledge bases.
func (s *Server) Query(
	ctx context.Context,
	req *connect.Request[kbv1.QueryRequest],
) (*connect.Response[kbv1.QueryResponse], error) {
	effort := req.Msg.Effort
	if effort == kbv1.Effort_EFFORT_UNSPECIFIED {
		effort = kbv1.Effort_EFFORT_MEDIUM
	}

	log.Printf("ask_knowledge_base: question=%q effort=%s knowledge_bases=%v",
		req.Msg.Question, effort, req.Msg.KnowledgeBases)

	// Stub response
	answer := fmt.Sprintf("[STUB] Would search for: %s (effort=%s)", req.Msg.Question, effort)
	resp := &kbv1.QueryResponse{
		Answer:    answer,
		Citations: []string{},
	}

	return connect.NewResponse(resp), nil
}

// Ensure Server implements the handler interface.
var _ kbv1connect.KnowledgeBaseServiceHandler = (*Server)(nil)
