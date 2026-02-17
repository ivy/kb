package store

import (
	"context"
	"encoding/json"
	"time"
)

// Store provides relational metadata operations (chunks, provenance, sources).
type Store interface {
	UpsertChunk(ctx context.Context, contentHash []byte, content string) error
	DeleteOrphanedChunks(ctx context.Context) (int64, error)
	UpsertProvenance(ctx context.Context, p Provenance) error
	DeleteProvenance(ctx context.Context, sourceURL string, contentHash []byte) error
	ListProvenance(ctx context.Context, sourceURL string) ([]Provenance, error)
	LookupByKeyword(ctx context.Context, keyword string, limit int) ([]SearchResult, error)
	GetDocument(ctx context.Context, sourceURL, filePath string) ([]ProvenanceRow, error)
	AddSource(ctx context.Context, url, kind string, cfg json.RawMessage) error
	ListSources(ctx context.Context) ([]Source, error)
	RemoveSource(ctx context.Context, url string) error
	UpdateSourceIndexed(ctx context.Context, url, ref string) error
	GetChunkContent(ctx context.Context, contentHash []byte) (string, error)
}

// Provenance links a chunk to its source location.
type Provenance struct {
	SourceURL   string
	FilePath    string
	ContentHash []byte
	Position    int
	Metadata    json.RawMessage // heading hierarchy, etc.
	Ref         string          // commit SHA at index time
}

// SearchResult is returned by both keyword and vector similarity searches.
type SearchResult struct {
	ContentHash []byte
	Content     string
	Score       float64
	SourceURL   string
	FilePath    string
}

// ProvenanceRow represents a stored provenance record.
type ProvenanceRow struct {
	ContentHash []byte
	Content     string
}

// Source represents a registered content source.
type Source struct {
	URL         string
	Kind        string
	Config      json.RawMessage
	LastRef     string
	LastIndexed *time.Time
}
