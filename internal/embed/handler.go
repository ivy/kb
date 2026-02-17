package embed

import (
	"context"
	"errors"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/store"
)

// EmbedJob represents an embedding request.
type EmbedJob struct {
	ContentHash []byte `json:"content_hash"`
	Content     string `json:"content"`
}

// EmbedHandler returns a queue.HandlerFunc that embeds a chunk.
func EmbedHandler(v store.VectorStore, cfg config.Embedding) queue.HandlerFunc {
	return func(ctx context.Context, payload []byte) error {
		return errors.New("not implemented")
	}
}
