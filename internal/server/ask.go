package server

import (
	"fmt"
	"net/http"
)

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "ask endpoint not implemented")
}

func (s *Server) handleSessionAsk(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "session %s ask endpoint not implemented\n", sessionID)
}
