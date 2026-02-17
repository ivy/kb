package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ivy/kb/internal/config"
)

// PGStore implements Store and VectorStore against PostgreSQL + pgvector.
type PGStore struct {
	pool *pgxpool.Pool
}

// NewPGStore creates a new PostgreSQL-backed store.
func NewPGStore(ctx context.Context, cfg config.Database) (*PGStore, error) {
	return nil, errors.New("not implemented")
}

// Pool returns the underlying pgx pool for infrastructure that needs direct access (e.g., River queue).
func (s *PGStore) Pool() *pgxpool.Pool {
	return s.pool
}

// Migrate runs schema migrations (golang-migrate for KB tables, rivermigrate for River tables).
func (s *PGStore) Migrate(ctx context.Context) error {
	return errors.New("not implemented")
}

// Store interface implementation

func (s *PGStore) UpsertChunk(ctx context.Context, contentHash []byte, content string) error {
	return errors.New("not implemented")
}

func (s *PGStore) DeleteOrphanedChunks(ctx context.Context) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *PGStore) UpsertProvenance(ctx context.Context, p Provenance) error {
	return errors.New("not implemented")
}

func (s *PGStore) DeleteProvenance(ctx context.Context, sourceURL string, contentHash []byte) error {
	return errors.New("not implemented")
}

func (s *PGStore) LookupByKeyword(ctx context.Context, keyword string, limit int) ([]SearchResult, error) {
	return nil, errors.New("not implemented")
}

func (s *PGStore) GetDocument(ctx context.Context, sourceURL, filePath string) ([]ProvenanceRow, error) {
	return nil, errors.New("not implemented")
}

func (s *PGStore) AddSource(ctx context.Context, url, kind string, cfg json.RawMessage) error {
	return errors.New("not implemented")
}

func (s *PGStore) ListSources(ctx context.Context) ([]Source, error) {
	return nil, errors.New("not implemented")
}

func (s *PGStore) RemoveSource(ctx context.Context, url string) error {
	return errors.New("not implemented")
}

func (s *PGStore) UpdateSourceIndexed(ctx context.Context, url, ref string) error {
	return errors.New("not implemented")
}

// VectorStore interface implementation

func (s *PGStore) UpsertEmbedding(ctx context.Context, contentHash []byte, model string, vector []float32) error {
	return errors.New("not implemented")
}

func (s *PGStore) SearchSimilar(ctx context.Context, vector []float32, limit int) ([]SearchResult, error) {
	return nil, errors.New("not implemented")
}

func (s *PGStore) DeleteEmbeddings(ctx context.Context, contentHash []byte) error {
	return errors.New("not implemented")
}

var (
	_ Store       = (*PGStore)(nil)
	_ VectorStore = (*PGStore)(nil)
)
