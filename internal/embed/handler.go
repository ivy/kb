package embed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/store"
)

// EmbedJob represents an embedding request.
type EmbedJob struct {
	ContentHash []byte `json:"content_hash"`
}

// embeddingRequest is the JSON body sent to the OpenAI embeddings API.
type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// embeddingResponse is the JSON response from the OpenAI embeddings API.
type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// EmbedHandler returns a queue.HandlerFunc that embeds a chunk.
// It looks up chunk content, calls the embedding API, and upserts the vector.
func EmbedHandler(s store.VectorStore, cs store.Store, cfg config.Embedding) queue.HandlerFunc {
	return EmbedHandlerWithClient(s, cs, cfg, http.DefaultClient, "https://api.openai.com")
}

// EmbedHandlerWithClient is like EmbedHandler but accepts a custom HTTP client and base URL for testing.
func EmbedHandlerWithClient(vs store.VectorStore, cs store.Store, cfg config.Embedding, client *http.Client, baseURL string) queue.HandlerFunc {
	embedClient := NewClientWithHTTP(cfg, client, baseURL)

	return func(ctx context.Context, payload []byte) error {
		var job EmbedJob
		if err := json.Unmarshal(payload, &job); err != nil {
			return fmt.Errorf("decoding embed job: %w", err)
		}

		content, err := cs.GetChunkContent(ctx, job.ContentHash)
		if err != nil {
			return fmt.Errorf("getting chunk content: %w", err)
		}

		vector, err := embedClient.Embed(ctx, content)
		if err != nil {
			return fmt.Errorf("calling embedding API: %w", err)
		}

		if err := vs.UpsertEmbedding(ctx, job.ContentHash, cfg.Model, vector); err != nil {
			return fmt.Errorf("upserting embedding: %w", err)
		}

		return nil
	}
}
