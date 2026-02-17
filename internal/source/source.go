package source

import "context"

// Result contains the extracted files and metadata from a source.
type Result struct {
	Ref   string            // commit SHA or ref that was fetched
	Files map[string][]byte // relative path → file content (Markdown files only)
}

// Fetcher retrieves files from a source. Abstracted as a function type so
// the reindex handler can be tested without I/O.
type Fetcher func(ctx context.Context, sourceURL, token string) (*Result, error)
