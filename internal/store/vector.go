package store

import "context"

// VectorStore provides embedding and similarity search operations.
type VectorStore interface {
	UpsertEmbedding(ctx context.Context, contentHash []byte, model string, vector []float32) error
	SearchSimilar(ctx context.Context, vector []float32, limit int) ([]SearchResult, error)
	DeleteEmbeddings(ctx context.Context, contentHash []byte) error
}
