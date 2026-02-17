package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	question := strings.TrimSpace(string(body))
	if question == "" {
		http.Error(w, "empty question", http.StatusBadRequest)
		return
	}

	model := r.Header.Get("X-Model")

	answer, err := s.agent.Ask(r.Context(), question, nil, model)
	if err != nil {
		http.Error(w, fmt.Sprintf("agent error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = fmt.Fprint(w, answer)
}

func (s *Server) handleSessionAsk(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	question := strings.TrimSpace(string(body))
	if question == "" {
		http.Error(w, "empty question", http.StatusBadRequest)
		return
	}

	model := r.Header.Get("X-Model")

	answer, err := s.agent.Ask(r.Context(), question, nil, model)
	if err != nil {
		http.Error(w, fmt.Sprintf("agent error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = fmt.Fprint(w, answer)
}
