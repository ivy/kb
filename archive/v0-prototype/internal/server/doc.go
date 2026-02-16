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
