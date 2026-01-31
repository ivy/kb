// Package embed provides text embedding via the OpenAI API.
//
// It wraps the OpenAI embeddings endpoint (text-embedding-3-small)
// to generate 1536-dimensional vectors for semantic search.
//
// The package handles batching, rate limiting, and retries for
// reliable embedding generation.
package embed
