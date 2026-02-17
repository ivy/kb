package index

import (
	"context"
	"errors"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/store"
)

// ReindexJob represents a reindexing request.
type ReindexJob struct {
	SourceURL string `json:"source_url"`
}

// ReindexHandler returns a queue.HandlerFunc that fetches, chunks, and diffs a source.
func ReindexHandler(s store.Store, q queue.Queue, cfg config.GitLab) queue.HandlerFunc {
	return func(ctx context.Context, payload []byte) error {
		return errors.New("not implemented")
	}
}
