# Project Scaffold

## Goal

Set up the Go module and directory structure so the project compiles and is ready for stub implementations.

## Deliverables

### 1. Go Module

Initialize with:
```bash
go mod init github.com/ivy/kb
```

### 2. Directory Structure

```
kb/
├── cmd/
│   ├── kb-server/main.go
│   └── kb-mcp/main.go
├── internal/
│   ├── server/doc.go
│   ├── mcp/doc.go
│   ├── db/doc.go
│   ├── embed/doc.go
│   ├── search/doc.go
│   ├── claude/doc.go
│   └── index/doc.go
├── .gitignore
├── go.mod
└── go.sum
```

### 3. File Contents

#### `cmd/kb-server/main.go`

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("kb-server: not implemented")
	os.Exit(1)
}
```

#### `cmd/kb-mcp/main.go`

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("kb-mcp: not implemented")
	os.Exit(1)
}
```

#### `internal/server/doc.go`

```go
// Package server implements the kb-server daemon.
//
// It listens on a Unix socket (default: ${XDG_RUNTIME_DIR:-/tmp}/kb.sock)
// and handles knowledge queries from kb-mcp. The server manages the query
// lifecycle: receiving requests, coordinating with the search and claude
// packages, and returning synthesized responses.
//
// The server also manages background indexing tasks and file watching
// for live updates when vault contents change.
package server
```

#### `internal/mcp/doc.go`

```go
// Package mcp implements the MCP (Model Context Protocol) server.
//
// It exposes two tools via stdio transport:
//
//   - list_knowledge_bases(): returns available knowledge bases
//   - ask_knowledge_base(question, effort?, knowledge_bases?): queries knowledge bases
//
// The MCP server acts as a bridge between AI agents and kb-server,
// translating MCP tool calls into Unix socket requests.
//
// Tool parameters:
//
//	ask_knowledge_base:
//	  - question: free-form natural language query (required)
//	  - effort: "low" | "medium" | "high" (default: medium)
//	  - knowledge_bases: optional list of knowledge base names to search
package mcp
```

#### `internal/db/doc.go`

```go
// Package db provides SQLite database operations for kb.
//
// Each vault has its own SQLite database stored at:
//
//	${XDG_DATA_HOME:-~/.local/share}/kb/<vault-id>.sqlite
//
// Where vault-id is "<basename>-<hash8>" (first 8 chars of SHA256 of
// the absolute path) for human readability and collision resistance.
//
// The database stores vault metadata, note content, and embeddings
// using sqlite-vec for vector similarity search.
package db
```

#### `internal/embed/doc.go`

```go
// Package embed provides text embedding via the OpenAI API.
//
// It wraps the OpenAI embeddings endpoint (text-embedding-3-small)
// to generate 1536-dimensional vectors for semantic search.
//
// The package handles batching, rate limiting, and retries for
// reliable embedding generation.
package embed
```

#### `internal/search/doc.go`

```go
// Package search implements semantic search over vault notes.
//
// It combines the embed package (for query embedding) with the db
// package (for vector similarity via sqlite-vec) to find notes
// semantically related to a query.
//
// Search returns ranked results with relevance scores and excerpts,
// optionally filtered by vault.
package search
```

#### `internal/claude/doc.go`

```go
// Package claude provides a Go wrapper for headless Claude Code.
//
// It invokes Claude Code CLI in non-interactive mode to synthesize
// answers from search results. The package handles:
//   - Custom system prompts for knowledge retrieval
//   - Working directory control (vault root)
//   - JSON output parsing
//   - Effort level configuration
package claude
```

#### `internal/index/doc.go`

```go
// Package index manages vault indexing and synchronization.
//
// It handles:
//   - Initial vault scanning and embedding
//   - Startup sync (detecting changes while server was down)
//   - Live updates via fsevents file watching
//   - Change detection using mtime comparison
//
// The indexer runs as a background goroutine within kb-server.
package index
```

#### `.gitignore`

```gitignore
# Binaries
/kb-server
/kb-mcp
*.exe

# Build artifacts
/dist/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Test artifacts
*.test
coverage.out
coverage.html

# Local config (if any)
.env
.env.local
```

## Acceptance Criteria

- [ ] `go mod tidy` succeeds
- [ ] `go build ./...` succeeds with no errors
- [ ] Running `go run ./cmd/kb-server` prints "kb-server: not implemented" and exits with code 1
- [ ] Running `go run ./cmd/kb-mcp` prints "kb-mcp: not implemented" and exits with code 1
- [ ] All internal packages have `doc.go` files describing their purpose

## Notes

- No external dependencies yet; `go.sum` may be empty or absent
