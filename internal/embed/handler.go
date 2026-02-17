package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	return func(ctx context.Context, payload []byte) error {
		var job EmbedJob
		if err := json.Unmarshal(payload, &job); err != nil {
			return fmt.Errorf("decoding embed job: %w", err)
		}

		// Look up chunk content.
		content, err := cs.GetChunkContent(ctx, job.ContentHash)
		if err != nil {
			return fmt.Errorf("getting chunk content: %w", err)
		}

		// Call embedding API.
		vector, err := callEmbeddingAPI(ctx, client, baseURL, cfg, content)
		if err != nil {
			return fmt.Errorf("calling embedding API: %w", err)
		}

		// Upsert the embedding.
		if err := vs.UpsertEmbedding(ctx, job.ContentHash, cfg.Model, vector); err != nil {
			return fmt.Errorf("upserting embedding: %w", err)
		}

		return nil
	}
}

func callEmbeddingAPI(ctx context.Context, client *http.Client, baseURL string, cfg config.Embedding, content string) ([]float32, error) {
	reqBody, err := json.Marshal(embeddingRequest{
		Model: cfg.Model,
		Input: content,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned status %d: %s", resp.StatusCode, body)
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("embedding API returned no data")
	}

	return result.Data[0].Embedding, nil
}
