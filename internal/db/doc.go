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
