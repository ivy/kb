# 002: embed Package

**Time estimate:** ~45 minutes

## Goal

OpenAI embeddings client for generating 1536-dimensional vectors.

## Interface

```go
// Embed single text string
func Embed(ctx context.Context, text string) ([]float32, error)

// Embed batch of texts
func EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
```

## Implementation

- **Model:** `text-embedding-3-small`
- **Dimensions:** 1536
- **API:** OpenAI Embeddings API via `github.com/openai/openai-go` (or net/http directly)

### Features

- Rate limiting (respect OpenAI limits)
- Retries with exponential backoff
- Batch support for efficiency
- Cost tracking (optional, helpful for monitoring)

### Configuration

```go
type Config struct {
    APIKey     string  // From env var OPENAI_API_KEY
    Model      string  // Default: "text-embedding-3-small"
    MaxRetries int     // Default: 3
}
```

## Validation

- [ ] Can embed a test string
- [ ] Returns 1536-dimensional vectors
- [ ] Batch embedding works
- [ ] Handles API errors gracefully
- [ ] Respects rate limits

## Testing Strategy

- **Unit tests:** Mock HTTP responses for error handling
- **Integration tests:** Real API calls with test key (optional, can skip for speed)
- Interface design allows easy mocking in search package tests

## Dependencies

- `github.com/openai/openai-go` (or use `net/http` directly for minimal deps)

## Notes

- Model choice is swappable - interface allows future changes
- Consider caching embeddings by content hash (future optimization)
- Cost tracking: ~$0.02 per 1M tokens for text-embedding-3-small
