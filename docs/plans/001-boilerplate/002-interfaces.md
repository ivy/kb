# 002: Interfaces & Placeholder Implementations

**Dependencies**: 000-module-init
**Blocks**: 003-cli-skeleton, 004-server-handlers, 005-test-scaffolding, 006-composition-root
**Parallel with**: 001-config

The seams that enable parallel implementation. Each interface is defined in the package that owns the concept. Placeholder implementations return `errors.New("not implemented")` so the project compiles end-to-end.

## Deliverables

### internal/store/store.go — Store interface

```go
type Store interface {
    UpsertChunk(ctx context.Context, contentHash []byte, content string) error
    DeleteOrphanedChunks(ctx context.Context) (int64, error)
    UpsertProvenance(ctx context.Context, p Provenance) error
    DeleteProvenance(ctx context.Context, sourceURL string, contentHash []byte) error
    LookupByKeyword(ctx context.Context, keyword string, limit int) ([]SearchResult, error)
    GetDocument(ctx context.Context, sourceURL, filePath string) ([]ProvenanceRow, error)
    AddSource(ctx context.Context, url, kind string, cfg json.RawMessage) error
    ListSources(ctx context.Context) ([]Source, error)
    RemoveSource(ctx context.Context, url string) error
    UpdateSourceIndexed(ctx context.Context, url, ref string) error
}
```

Plus supporting types: `Provenance`, `SearchResult`, `ProvenanceRow`, `Source`.

### internal/store/vector.go — VectorStore interface

```go
type VectorStore interface {
    UpsertEmbedding(ctx context.Context, contentHash []byte, model string, vector []float32) error
    SearchSimilar(ctx context.Context, vector []float32, limit int) ([]SearchResult, error)
    DeleteEmbeddings(ctx context.Context, contentHash []byte) error
}
```

### internal/store/pgstore.go — PGStore placeholder

```go
type PGStore struct { pool *pgxpool.Pool }

func NewPGStore(ctx context.Context, cfg config.Database) (*PGStore, error)
func (s *PGStore) Pool() *pgxpool.Pool
func (s *PGStore) Migrate(ctx context.Context) error
```

Implements both `Store` and `VectorStore`. All methods return `errors.New("not implemented")`.

`Migrate()` must run **two** migration systems:

1. **golang-migrate** for KB tables (chunks, embeddings, provenance, sources, pgvector extension). SQL files in `store/migrations/`, embedded via `go:embed`. Numbered up/down pairs.
2. **rivermigrate** for River's job tables. River owns its schema — we don't copy it into our migrations, avoiding breakage on River version bumps.

### internal/queue/queue.go — Queue interface

```go
type Queue interface {
    Enqueue(ctx context.Context, job Job) error
    Stats(ctx context.Context) (Stats, error)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type Job struct { Kind string; Payload []byte }
type Stats struct { Pending, Running, Failed int }
type HandlerFunc func(ctx context.Context, payload []byte) error
```

### internal/queue/river.go — River placeholder

```go
func NewRiverQueue(pool *pgxpool.Pool, cfg config.Workers, handlers map[string]HandlerFunc) (Queue, error)
```

Returns `errors.New("not implemented")`.

### internal/session/session.go — Manager interface

```go
type Manager interface {
    Create(ctx context.Context) (Session, error)
    Get(ctx context.Context, id string) (Session, error)
    Append(ctx context.Context, id string, turn Turn) error
}
```

Plus types: `Session`, `Turn`.

### internal/session/lru.go + jsonl.go — placeholders

`NewLRUManager(cfg config.Sessions) Manager` — returns stub.

### internal/agent/agent.go — Agent interface

```go
type Agent interface {
    Ask(ctx context.Context, question string, history []session.Turn, model string) (string, error)
}

type MRKLAgent struct { ... }
func New(s store.Store, v store.VectorStore, cfg config.Agent) Agent
```

Returns `errors.New("not implemented")`.

### internal/index/handler.go — ReindexHandler signature

```go
type ReindexJob struct { SourceURL string `json:"source_url"` }
func ReindexHandler(s store.Store, q queue.Queue, cfg config.GitLab) queue.HandlerFunc
```

### internal/index/fetcher.go + chunker.go — signatures only

Placeholder function signatures for `Fetch()` and `Chunk()`. Stubs return `errors.New("not implemented")`.

### internal/embed/handler.go — EmbedHandler signature

```go
type EmbedJob struct { ContentHash []byte `json:"content_hash"`; Content string `json:"content"` }
func EmbedHandler(v store.VectorStore, cfg config.Embedding) queue.HandlerFunc
```

## Dependencies to add

- `github.com/jackc/pgx/v5` (for pgxpool types in PGStore)

## Acceptance

- `go build ./...` succeeds with all interfaces defined and placeholders compiling
- Each interface is in the package that owns the concept
- No circular imports between packages
- Placeholder impls satisfy their interfaces (compiler-checked)

## Notes

- Config types from 001 are referenced but not blocking: use `config.Database`, etc. These can be empty structs initially if 001 hasn't landed yet, or both slices can land in the same commit
- The parent plan emphasizes: `Pool()` is on the concrete `PGStore` type, NOT on the `Store` interface
- `SearchResult` is shared between `Store` and `VectorStore` — define it once in `store/store.go`
