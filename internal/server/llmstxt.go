package server

import (
	"fmt"
	"net/http"
)

func (s *Server) handleLLMsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprint(w, `# KB

> Knowledge base service for team documentation.

## Capabilities

- Index Git repositories (GitLab) into searchable chunks
- Semantic search via embeddings (pgvector)
- Keyword search via PostgreSQL full-text search
- Conversational Q&A via agent with tool use
- Session management for multi-turn conversations

## Endpoints

- POST /v1/ask — Ask a question (plain text in, markdown out)
- POST /v1/sessions/{id}/ask — Ask within a session context
- POST /webhook — GitLab system hook for auto-reindexing
- GET /llms.txt — This file
`)
}
