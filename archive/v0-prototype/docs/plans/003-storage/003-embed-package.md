# 003: Embeddings

## Goal

OpenAI API client for generating 3072-dimensional embeddings.

## Model

| Property | Value |
|----------|-------|
| Model | `text-embedding-3-large` |
| Dimensions | 3072 |
| Max input tokens | 8191 |
| Encoding | cl100k_base |

## Implementation

Create `internal/embed/client.go`:

```go
package embed

import (
    "context"

    "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"
)

type Client struct {
    client *openai.Client
}

func New(apiKey string) *Client {
    return &Client{
        client: openai.NewClient(option.WithAPIKey(apiKey)),
    }
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
    resp, err := c.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
        Model: openai.F(openai.EmbeddingModelTextEmbedding3Large),
        Input: openai.F[openai.EmbeddingNewParamsInputUnion](
            openai.EmbeddingNewParamsInputArrayOfStrings{text},
        ),
        Dimensions: openai.Int(3072),
    })
    if err != nil {
        return nil, err
    }

    // Convert []float64 to []float32
    embedding64 := resp.Data[0].Embedding
    embedding32 := make([]float32, len(embedding64))
    for i, v := range embedding64 {
        embedding32[i] = float32(v)
    }

    return embedding32, nil
}
```

**Dependencies:** `github.com/openai/openai-go`

## Validation

- [ ] Can embed a test string
- [ ] Returns 3072-dimensional vector
- [ ] Handles API errors (returns error, doesn't panic)
- [ ] Integration test with real API (gated by env var)

## Testing

```go
func TestEmbed(t *testing.T) {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        t.Skip("OPENAI_API_KEY not set")
    }

    client := New(apiKey)
    vec, err := client.Embed(context.Background(), "test text")

    if err != nil {
        t.Fatal(err)
    }

    if len(vec) != 3072 {
        t.Errorf("expected 3072 dims, got %d", len(vec))
    }
}
```

## Notes

- **No batching yet** - single embeddings only (batching in Phase 3 if needed)
- **No retry logic** - fail fast on API errors
- **No rate limiting** - trust OpenAI SDK defaults
- Keep it simple - just wrap the API

## Time Estimate

1 hour
