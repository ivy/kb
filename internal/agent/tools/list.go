package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/ivy/kb/internal/store"
)

// ListTool browses what's indexed in the knowledge base.
type ListTool struct {
	store store.Store
}

// NewListTool creates a new list tool.
func NewListTool(s store.Store) *ListTool {
	return &ListTool{store: s}
}

func (t *ListTool) Name() string { return "list" }

func (t *ListTool) Description() string {
	return "Browse what's indexed. Input: empty string to list all indexed sources, or a source URL to list files within that source. Use this to understand coverage before concluding something doesn't exist."
}

func (t *ListTool) Call(ctx context.Context, input string) (string, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return t.listSources(ctx)
	}
	return t.listFiles(ctx, input)
}

func (t *ListTool) listSources(ctx context.Context) (string, error) {
	sources, err := t.store.ListSources(ctx)
	if err != nil {
		return "", fmt.Errorf("listing sources: %w", err)
	}

	if len(sources) == 0 {
		return "No sources indexed.", nil
	}

	var sb strings.Builder
	sb.WriteString("Sources:\n")
	for _, s := range sources {
		indexed := "never"
		if s.LastIndexed != nil {
			indexed = s.LastIndexed.Format("2006-01-02")
		}
		fmt.Fprintf(&sb, "- %s (%s, indexed %s)\n", s.URL, s.Kind, indexed)
	}
	return sb.String(), nil
}

func (t *ListTool) listFiles(ctx context.Context, sourceURL string) (string, error) {
	files, err := t.store.ListFiles(ctx, sourceURL)
	if err != nil {
		return "", fmt.Errorf("listing files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Sprintf("No files indexed for %s.", sourceURL), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Files in %s:\n", sourceURL)
	for _, f := range files {
		fmt.Fprintf(&sb, "- %s\n", f)
	}
	return sb.String(), nil
}
