package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ivy/kb/internal/embed"
	"github.com/ivy/kb/internal/store"
)

// SearchTool performs semantic vector search across indexed knowledge.
type SearchTool struct {
	embedder *embed.Client
	vectors  store.VectorStore
}

// NewSearchTool creates a new search tool.
func NewSearchTool(embedder *embed.Client, vectors store.VectorStore) *SearchTool {
	return &SearchTool{embedder: embedder, vectors: vectors}
}

func (t *SearchTool) Name() string { return "search" }

func (t *SearchTool) Description() string {
	return "Semantic vector search across all indexed knowledge. Input: natural language query. Returns ranked results with provenance metadata (source URL, file path, heading hierarchy, indexed date, ref). No chunk content — use read to fetch content."
}

func (t *SearchTool) Call(ctx context.Context, input string) (string, error) {
	vec, err := t.embedder.Embed(ctx, input)
	if err != nil {
		return "", fmt.Errorf("embedding query: %w", err)
	}

	results, err := t.vectors.SearchSimilar(ctx, vec, 10)
	if err != nil {
		return "", fmt.Errorf("searching: %w", err)
	}

	if len(results) == 0 {
		return "No results found.", nil
	}

	return formatResults(results), nil
}

// formatResults formats search results as text with provenance metadata only (no content).
func formatResults(results []store.SearchResult) string {
	var sb strings.Builder
	for i, r := range results {
		heading := extractHeading(r.Metadata)
		fmt.Fprintf(&sb, "%d. %s — %s (indexed %s, ref %s)\n",
			i+1, r.SourceURL, r.FilePath,
			r.IndexedAt.Format("2006-01-02"), r.Ref)
		if heading != "" {
			fmt.Fprintf(&sb, "   Heading: %s\n", heading)
		}
	}
	return sb.String()
}

// extractHeading pulls the heading hierarchy from chunk metadata.
func extractHeading(metadata json.RawMessage) string {
	if len(metadata) == 0 {
		return ""
	}
	var m struct {
		Headings []string `json:"headings"`
	}
	if err := json.Unmarshal(metadata, &m); err != nil || len(m.Headings) == 0 {
		return ""
	}
	parts := make([]string, len(m.Headings))
	for i, h := range m.Headings {
		parts[i] = strings.TrimLeft(h, "# ")
	}
	return strings.Join(parts, " > ")
}
