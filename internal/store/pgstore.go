package store

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/ivy/kb/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// PGStore implements Store and VectorStore against PostgreSQL + pgvector.
type PGStore struct {
	pool *pgxpool.Pool
}

// NewPGStore creates a new PostgreSQL-backed store.
func NewPGStore(ctx context.Context, cfg config.Database) (*PGStore, error) {
	pool, err := pgxpool.New(ctx, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("creating pgx pool: %w", err)
	}
	return &PGStore{pool: pool}, nil
}

// Pool returns the underlying pgx pool for infrastructure that needs direct access (e.g., River queue).
func (s *PGStore) Pool() *pgxpool.Pool {
	return s.pool
}

// Migrate runs schema migrations using golang-migrate with embedded SQL files.
func (s *PGStore) Migrate(ctx context.Context) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	db := stdlib.OpenDBFromPool(s.pool)
	defer func() { _ = db.Close() }()

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("creating migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("running migrations: %w", err)
	}

	// Run River schema migrations (idempotent, tracks its own state).
	rm, err := rivermigrate.New(riverpgxv5.New(s.pool), nil)
	if err != nil {
		return fmt.Errorf("creating river migrator: %w", err)
	}
	if _, err := rm.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("running river migrations: %w", err)
	}

	return nil
}

// Store interface implementation

func (s *PGStore) UpsertChunk(ctx context.Context, contentHash []byte, content string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO chunks (content_hash, content) VALUES ($1, $2) ON CONFLICT (content_hash) DO NOTHING`,
		contentHash, content)
	return err
}

func (s *PGStore) DeleteOrphanedChunks(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM chunks WHERE content_hash NOT IN (SELECT content_hash FROM provenance)`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *PGStore) UpsertProvenance(ctx context.Context, p Provenance) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO provenance (content_hash, source_url, file_path, position, metadata, ref)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (content_hash, source_url, file_path)
		 DO UPDATE SET position = $4, metadata = $5, indexed_at = now(), ref = $6`,
		p.ContentHash, p.SourceURL, p.FilePath, p.Position, p.Metadata, p.Ref)
	return err
}

func (s *PGStore) DeleteProvenance(ctx context.Context, sourceURL string, contentHash []byte) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM provenance WHERE source_url = $1 AND content_hash = $2`,
		sourceURL, contentHash)
	return err
}

func (s *PGStore) ListProvenance(ctx context.Context, sourceURL string) ([]Provenance, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT content_hash, source_url, file_path, position, metadata, ref
		 FROM provenance WHERE source_url = $1`, sourceURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Provenance
	for rows.Next() {
		var p Provenance
		if err := rows.Scan(&p.ContentHash, &p.SourceURL, &p.FilePath, &p.Position, &p.Metadata, &p.Ref); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *PGStore) LookupByKeyword(ctx context.Context, keyword string, limit int) ([]SearchResult, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.content_hash, c.content,
		        ts_rank(to_tsvector('english', c.content), plainto_tsquery('english', $1)) AS score,
		        p.source_url, p.file_path, p.metadata, p.indexed_at, p.ref
		 FROM chunks c
		 JOIN provenance p ON p.content_hash = c.content_hash
		 WHERE to_tsvector('english', c.content) @@ plainto_tsquery('english', $1)
		 ORDER BY score DESC
		 LIMIT $2`,
		keyword, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ContentHash, &r.Content, &r.Score, &r.SourceURL, &r.FilePath, &r.Metadata, &r.IndexedAt, &r.Ref); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *PGStore) GetDocument(ctx context.Context, sourceURL, filePath string) ([]ProvenanceRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT p.content_hash, c.content
		 FROM provenance p
		 JOIN chunks c ON c.content_hash = p.content_hash
		 WHERE p.source_url = $1 AND p.file_path = $2
		 ORDER BY p.position`,
		sourceURL, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ProvenanceRow
	for rows.Next() {
		var r ProvenanceRow
		if err := rows.Scan(&r.ContentHash, &r.Content); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *PGStore) AddSource(ctx context.Context, url, kind string, cfg json.RawMessage) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sources (url, kind, config) VALUES ($1, $2, $3)`,
		url, kind, cfg)
	return err
}

func (s *PGStore) ListSources(ctx context.Context) ([]Source, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT url, kind, config, last_indexed_ref, last_indexed_at FROM sources`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Source
	for rows.Next() {
		var src Source
		var lastRef *string
		if err := rows.Scan(&src.URL, &src.Kind, &src.Config, &lastRef, &src.LastIndexed); err != nil {
			return nil, err
		}
		if lastRef != nil {
			src.LastRef = *lastRef
		}
		result = append(result, src)
	}
	return result, rows.Err()
}

func (s *PGStore) RemoveSource(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (s *PGStore) UpdateSourceIndexed(ctx context.Context, url, ref string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sources SET last_indexed_ref = $2, last_indexed_at = now() WHERE url = $1`,
		url, ref)
	return err
}

func (s *PGStore) GetChunkContent(ctx context.Context, contentHash []byte) (string, error) {
	var content string
	err := s.pool.QueryRow(ctx,
		`SELECT content FROM chunks WHERE content_hash = $1`, contentHash).Scan(&content)
	if err != nil {
		return "", fmt.Errorf("getting chunk content: %w", err)
	}
	return content, nil
}

// VectorStore interface implementation

func (s *PGStore) UpsertEmbedding(ctx context.Context, contentHash []byte, model string, vector []float32) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO embeddings (content_hash, model, embedding)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (content_hash, model) DO UPDATE SET embedding = $3`,
		contentHash, model, pgvectorSlice(vector))
	return err
}

func (s *PGStore) SearchSimilar(ctx context.Context, vector []float32, limit int) ([]SearchResult, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT e.content_hash, c.content, (e.embedding <=> $1) AS score,
		        p.source_url, p.file_path, p.metadata, p.indexed_at, p.ref
		 FROM embeddings e
		 JOIN chunks c ON c.content_hash = e.content_hash
		 JOIN provenance p ON p.content_hash = e.content_hash
		 ORDER BY e.embedding <=> $1
		 LIMIT $2`,
		pgvectorSlice(vector), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ContentHash, &r.Content, &r.Score, &r.SourceURL, &r.FilePath, &r.Metadata, &r.IndexedAt, &r.Ref); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *PGStore) DeleteEmbeddings(ctx context.Context, contentHash []byte) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM embeddings WHERE content_hash = $1`, contentHash)
	return err
}

func (s *PGStore) ListFiles(ctx context.Context, sourceURL string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT file_path FROM provenance
		 WHERE source_url = $1
		 ORDER BY file_path`,
		sourceURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// pgvectorSlice formats a float32 slice as a pgvector-compatible string.
func pgvectorSlice(v []float32) string {
	buf := make([]byte, 0, len(v)*12+2)
	buf = append(buf, '[')
	for i, f := range v {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, fmt.Sprintf("%g", f)...)
	}
	buf = append(buf, ']')
	return string(buf)
}

var (
	_ Store       = (*PGStore)(nil)
	_ VectorStore = (*PGStore)(nil)
)
