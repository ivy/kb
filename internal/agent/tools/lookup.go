package tools

import (
	"context"
	"fmt"

	"github.com/ivy/kb/internal/store"
)

// LookupTool performs exact keyword/identifier search.
type LookupTool struct {
	store store.Store
}

// NewLookupTool creates a new lookup tool.
func NewLookupTool(s store.Store) *LookupTool {
	return &LookupTool{store: s}
}

func (t *LookupTool) Name() string { return "lookup" }

func (t *LookupTool) Description() string {
	return "Exact keyword/identifier search across all indexed knowledge. Input: keyword or identifier string (e.g. incident IDs, service names, config keys). Returns matching results with provenance metadata. Use for precise terms that semantic search handles poorly."
}

func (t *LookupTool) Call(ctx context.Context, input string) (string, error) {
	results, err := t.store.LookupByKeyword(ctx, input, 10)
	if err != nil {
		return "", fmt.Errorf("keyword lookup: %w", err)
	}

	if len(results) == 0 {
		return "No results found.", nil
	}

	return formatResults(results), nil
}
