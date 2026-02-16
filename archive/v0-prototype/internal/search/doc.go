// Package search implements semantic search over vault notes.
//
// It combines the embed package (for query embedding) with the db
// package (for vector similarity via sqlite-vec) to find notes
// semantically related to a query.
//
// Search returns ranked results with relevance scores and excerpts,
// optionally filtered by vault.
package search
