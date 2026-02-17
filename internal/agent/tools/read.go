package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ivy/kb/internal/store"
)

// ReadTool fetches content from a specific document with progressive disclosure.
type ReadTool struct {
	store store.Store
}

// NewReadTool creates a new read tool.
func NewReadTool(s store.Store) *ReadTool {
	return &ReadTool{store: s}
}

func (t *ReadTool) Name() string { return "read" }

func (t *ReadTool) Description() string {
	return "Fetch content from a specific document. Input: 'source_url file_path' for table of contents, or 'source_url file_path N' for chunk at position N, or 'source_url file_path N-M' for chunks at positions N through M. Always get the TOC first to understand structure before reading specific sections."
}

func (t *ReadTool) Call(ctx context.Context, input string) (string, error) {
	sourceURL, filePath, start, end, err := parseReadInput(input)
	if err != nil {
		return "", err
	}

	rows, err := t.store.GetDocument(ctx, sourceURL, filePath)
	if err != nil {
		return "", fmt.Errorf("getting document: %w", err)
	}

	if len(rows) == 0 {
		return fmt.Sprintf("Document not found: %s %s", sourceURL, filePath), nil
	}

	if start < 0 {
		return buildTOC(sourceURL, filePath, rows), nil
	}

	return buildContent(rows, start, end), nil
}

func parseReadInput(input string) (sourceURL, filePath string, start, end int, err error) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return "", "", -1, -1, fmt.Errorf("read requires at least source_url and file_path, got: %q", input)
	}

	sourceURL = parts[0]
	filePath = parts[1]
	start = -1
	end = -1

	if len(parts) >= 3 {
		rangeStr := parts[2]
		if dashIdx := strings.Index(rangeStr, "-"); dashIdx >= 0 {
			start, err = strconv.Atoi(rangeStr[:dashIdx])
			if err != nil {
				return "", "", -1, -1, fmt.Errorf("invalid range start: %w", err)
			}
			end, err = strconv.Atoi(rangeStr[dashIdx+1:])
			if err != nil {
				return "", "", -1, -1, fmt.Errorf("invalid range end: %w", err)
			}
		} else {
			start, err = strconv.Atoi(rangeStr)
			if err != nil {
				return "", "", -1, -1, fmt.Errorf("invalid position: %w", err)
			}
			end = start
		}
	}

	return sourceURL, filePath, start, end, nil
}

func buildTOC(sourceURL, filePath string, rows []store.ProvenanceRow) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Table of contents for %s — %s (%d chunks):\n\n", sourceURL, filePath, len(rows))

	for i, row := range rows {
		heading := extractFirstHeading(row.Content)
		if heading != "" {
			fmt.Fprintf(&sb, "  %d. %s\n", i, heading)
		} else {
			preview := row.Content
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			fmt.Fprintf(&sb, "  %d. %s\n", i, preview)
		}
	}

	return sb.String()
}

// extractFirstHeading finds the first markdown heading in content.
func extractFirstHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimLeft(trimmed, "# ")
		}
	}
	return ""
}

func buildContent(rows []store.ProvenanceRow, start, end int) string {
	var sb strings.Builder
	for i, row := range rows {
		if i >= start && i <= end {
			if sb.Len() > 0 {
				sb.WriteString("\n---\n\n")
			}
			sb.WriteString(row.Content)
		}
	}
	if sb.Len() == 0 {
		return fmt.Sprintf("No chunks found at positions %d-%d (document has %d chunks, positions 0-%d).", start, end, len(rows), len(rows)-1)
	}
	return sb.String()
}
