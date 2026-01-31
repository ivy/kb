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
