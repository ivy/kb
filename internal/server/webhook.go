package server

import (
	"fmt"
	"net/http"
)

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "webhook endpoint not implemented")
}
