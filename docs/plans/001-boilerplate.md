# 001: Go Boilerplate & Package Structure

Goal: define the app skeleton with interfaces, config, and placeholders so implementation can proceed in parallel across packages.

## Package Tree

```
kb/
├── cmd/kb/
│   └── main.go                    # entry point → cli.Execute()
│
├── internal/
│   ├── cli/                       # cobra commands
│   │   ├── root.go                # root command, global flags, config loading
│   │   ├── version.go             # kb version — prints build info
│   │   ├── serve.go               # kb serve — wires everything, starts server + workers
│   │   ├── setup.go               # kb setup — prerequisite checks, webhook config
│   │   ├── sources.go             # kb sources add|list|remove
│   │   ├── reindex.go             # kb reindex [url]
│   │   └── status.go              # kb status
│   │
│   ├── config/                    # centralized configuration
│   │   └── config.go              # Config struct with nested sub-structs, viper binding
│   │
│   ├── server/                    # HTTP server (tsnet + stdlib net/http)
│   │   ├── server.go              # listener setup (tsnet or plain TCP), mux
│   │   ├── ask.go                 # POST /v1/ask, POST /v1/sessions/:id/ask
│   │   ├── webhook.go             # POST /webhook (GitLab system hook)
│   │   ├── llmstxt.go             # GET /llms.txt
│   │   └── auth.go                # WhoIs + capability checking middleware
│   │
│   ├── agent/                     # KB agent (LangChainGo MRKL)
│   │   ├── agent.go               # agent executor setup, Run()
│   │   ├── prompt.md              # system prompt (//go:embed)
│   │   └── tools.go               # search, lookup, read, list tool definitions
│   │
│   ├── store/                     # data access (pluggable: PostgreSQL, SQLite, etc.)
│   │   ├── store.go               # Store interface (relational metadata)
│   │   ├── vector.go              # VectorStore interface (embeddings/search)
│   │   ├── pgstore.go             # PGStore: implements Store + VectorStore via pgx/pgvector
│   │   ├── chunks.go              # PGStore chunk operations
│   │   ├── provenance.go          # PGStore provenance operations + metadata queries
│   │   ├── sources.go             # PGStore source operations
│   │   ├── pgvector.go            # PGStore vector operations
│   │   ├── migrate.go             # golang-migrate runner (go:embed SQL files)
│   │   └── migrations/            # SQL migration files (golang-migrate)
│   │       ├── 001_initial.up.sql
│   │       └── 001_initial.down.sql
│   │
│   ├── queue/                     # job queue abstraction
│   │   ├── queue.go               # Queue interface + types
│   │   └── river.go               # River implementation (PostgreSQL deployments)
│   │
│   ├── index/                     # indexing pipeline
│   │   ├── handler.go             # reindex handler (pure business logic, no queue coupling)
│   │   ├── fetcher.go             # GitLab archive API → tarball → extracted tree
│   │   └── chunker.go             # Markdown structure-based chunking (BLAKE3)
│   │
│   ├── embed/                     # embedding service
│   │   └── handler.go             # embed handler (pure business logic, no queue coupling)
│   │
│   └── session/                   # session management
│       ├── session.go             # Manager interface + types
│       ├── lru.go                 # LRU cache (in-memory)
│       └── jsonl.go               # JSONL filesystem backing store
│
├── Makefile                       # build, test, lint targets with ldflags
├── go.mod
├── go.sum
└── mise.toml                      # tool versions (go, golangci-lint, ginkgo, etc.)
```

## Configuration

Centralized `config.Config` with nested sub-structs. Each package receives only its config slice — ISP at the package boundary, single source of truth for all knobs.

```go
// config/config.go
type Config struct {
    Database  Database  `mapstructure:"database"`
    Tailscale Tailscale `mapstructure:"tailscale"`
    GitLab    GitLab    `mapstructure:"gitlab"`
    Embedding Embedding `mapstructure:"embedding"`
    Agent     Agent     `mapstructure:"agent"`
    Sessions  Sessions  `mapstructure:"sessions"`
    Workers   Workers   `mapstructure:"workers"`
    Log       Log       `mapstructure:"log"`
}

type Database struct {
    URL string `mapstructure:"url"`
}

type Tailscale struct {
    Enabled  bool   `mapstructure:"enabled"`   // false → plain TCP listener, no auth
    Hostname string `mapstructure:"hostname"`
    StateDir string `mapstructure:"state_dir"`
    AuthZ    bool   `mapstructure:"authz"`     // false → tsnet listener but no capability checks
    AskCap   string `mapstructure:"ask_cap"`   // capability for query access (requires authz: true)
    AdminCap string `mapstructure:"admin_cap"` // capability for admin access (requires authz: true)
}

type GitLab struct {
    Token string `mapstructure:"token"`
}

type Embedding struct {
    Provider string `mapstructure:"provider"` // "openai"
    Model    string `mapstructure:"model"`    // "text-embedding-3-large"
    APIKey   string `mapstructure:"api_key"`
}

type Agent struct {
    DefaultModel string `mapstructure:"default_model"`
}

type Sessions struct {
    Dir string `mapstructure:"dir"`
    Max int    `mapstructure:"max"`
}

type Workers struct {
    Reindex int `mapstructure:"reindex"`
    Embed   int `mapstructure:"embed"`
}

type Log struct {
    Format string `mapstructure:"format"` // "journal" or "text"
    Level  string `mapstructure:"level"`  // "debug", "info", "warn", "error"
}

// Load reads config from file, env vars (KB_*), and returns a validated Config.
func Load() (*Config, error)
```

Config file example:

```yaml
# kb.yaml (or env vars KB_DATABASE_URL, KB_EMBEDDING_API_KEY, etc.)
database:
  url: postgres://localhost:5432/kb

tailscale:
  enabled: true                                  # false → plain TCP on :8080, no tsnet
  hostname: kb
  state_dir: /var/lib/kb/tailscale
  authz: true                                    # false → tsnet listener but no capability checks
  ask_cap: homestead.local/cap/kb/ask            # capability for query endpoints (requires authz)
  admin_cap: homestead.local/cap/kb/admin        # capability for admin endpoints (requires authz)

gitlab:
  token: ${GITLAB_TOKEN}

embedding:
  provider: openai
  model: text-embedding-3-large
  api_key: ${OPENAI_API_KEY}

agent:
  default_model: gpt-4.1

sessions:
  dir: /var/lib/kb/sessions
  max: 100

workers:
  reindex: 2
  embed: 4

log:
  format: journal
  level: info
```

**Why centralized with sub-structs:**

- One file to see all knobs (an agent adding a config key knows where to look)
- ISP at the boundary (`store.New(ctx, cfg.Database)` — store never sees embedding config)
- Validated once at startup, before wiring anything
- Adding knobs later (new provider, log level, rate limits) is a struct field, not a constructor signature change

## Build

### Version injection

Build metadata injected via `ldflags` into `internal/cli/version.go`:

```go
// internal/cli/version.go
var (
    version   = "dev"     // semantic version (from git tag or "dev")
    gitRef    = "unknown" // git commit SHA
    buildTag  = "unknown" // CI build tag or "local"
    buildDate = "unknown" // RFC 3339 timestamp
)
```

### Makefile

```makefile
MODULE   := github.com/ivy/kb
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_REF  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TAG ?= local
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -s -w \
  -X '$(MODULE)/internal/cli.version=$(VERSION)' \
  -X '$(MODULE)/internal/cli.gitRef=$(GIT_REF)' \
  -X '$(MODULE)/internal/cli.buildTag=$(BUILD_TAG)' \
  -X '$(MODULE)/internal/cli.buildDate=$(BUILD_DATE)'

.PHONY: build test test-unit lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/kb ./cmd/kb

test:
	ginkgo ./...

test-unit:
	ginkgo --label-filter='!db' ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
```

CI passes `BUILD_TAG` to stamp the build: `make build BUILD_TAG=pipeline-1234`. Local builds default to `local`.

### `kb version`

```
$ kb version
kb dev (abc1234) built local at 2026-02-16T09:32:00Z

$ kb version --json
{"version":"dev","git_ref":"abc1234","build_tag":"local","build_date":"2026-02-16T09:32:00Z"}
```

Plain text by default (Principle #1). `--json` for programmatic consumption when needed.

## Key Interfaces

These are the seams that enable parallel work and testability via Ginkgo/Gomega. Each is defined in the package that owns the concept; consumers depend on the interface, not the implementation.

### store.Store + store.VectorStore

Two interfaces that reflect two concerns with different swap profiles:

```go
// store/store.go — relational metadata (chunks, provenance, sources)
type Store interface {
    // Chunks
    UpsertChunk(ctx context.Context, contentHash []byte, content string) error
    DeleteOrphanedChunks(ctx context.Context) (int64, error)

    // Provenance
    UpsertProvenance(ctx context.Context, p Provenance) error
    DeleteProvenance(ctx context.Context, sourceURL string, contentHash []byte) error
    LookupByKeyword(ctx context.Context, keyword string, limit int) ([]SearchResult, error)
    GetDocument(ctx context.Context, sourceURL, filePath string) ([]ProvenanceRow, error)

    // Sources
    AddSource(ctx context.Context, url, kind string, cfg json.RawMessage) error
    ListSources(ctx context.Context) ([]Source, error)
    RemoveSource(ctx context.Context, url string) error
    UpdateSourceIndexed(ctx context.Context, url, ref string) error
}

// store/vector.go — vector operations (swappable: pgvector, sqlite-vec, ChromaDB, S3 Vectors)
type VectorStore interface {
    UpsertEmbedding(ctx context.Context, contentHash []byte, model string, vector []float32) error
    SearchSimilar(ctx context.Context, vector []float32, limit int) ([]SearchResult, error)
    DeleteEmbeddings(ctx context.Context, contentHash []byte) error
}
```

```go
// store/pgstore.go — implements both Store and VectorStore against PostgreSQL + pgvector.
type PGStore struct { pool *pgxpool.Pool }

func NewPGStore(ctx context.Context, cfg config.Database) (*PGStore, error)

// Pool returns the pgx pool for infrastructure that needs it (River queue).
// This is on the concrete type, not the interface — it's PostgreSQL-specific.
func (s *PGStore) Pool() *pgxpool.Pool

// Migrate runs KB schema migrations (golang-migrate, embedded SQL).
func (s *PGStore) Migrate(ctx context.Context) error
```

No `Pool()` on the interface — it's a PostgreSQL implementation detail. The composition root accesses it through the concrete `*PGStore` type when wiring River. A SQLite deployment wouldn't have a pgx pool or River.

**Why two interfaces:**
- Vector storage and relational metadata have different swap profiles
- **Swap just vectors**: PostgreSQL metadata + S3 Vectors/ChromaDB for embeddings (scale scenario)
- **Swap everything**: SQLite + sqlite-vec for small/embedded deployments (no PostgreSQL at all)
- `PGStore` implements both today — no behavioral change, no extra packages
- Consumers depend on the interface they actually use

**Who depends on what:**

| Consumer | `Store` | `VectorStore` |
|----------|---------|---------------|
| agent/ (search, lookup tools) | yes | yes |
| agent/ (read, list tools) | yes | — |
| index/ | yes | — |
| embed/ | — | yes |
| server/ | — | — (through agent) |
| cli/ | yes | — |

### session.Manager

```go
// session/session.go
type Manager interface {
    Create(ctx context.Context) (Session, error)
    Get(ctx context.Context, id string) (Session, error)
    Append(ctx context.Context, id string, turn Turn) error
}
```

Implemented by an LRU-fronted JSONL store. The interface exists because query.md explicitly calls for a Go interface to allow future replacement.

### agent.Agent

```go
// agent/agent.go
type Agent interface {
    Ask(ctx context.Context, question string, history []session.Turn, model string) (string, error)
}

type MRKLAgent struct {
    store   store.Store
    vectors store.VectorStore
    cfg     config.Agent
}

func New(s store.Store, v store.VectorStore, cfg config.Agent) Agent
```

Interface so the server can be tested without a real agent/LLM.

### queue.Queue

Abstracts the job queue so the queue backend can be swapped alongside the storage backend. River for PostgreSQL deployments; in-memory worker pool (or similar) for SQLite deployments.

```go
// queue/queue.go
type Queue interface {
    Enqueue(ctx context.Context, job Job) error
    Stats(ctx context.Context) (Stats, error)
    Start(ctx context.Context) error // blocks, runs workers until ctx cancels
    Stop(ctx context.Context) error
}

type Job struct {
    Kind    string // "reindex", "embed"
    Payload []byte // JSON-encoded args
}

type Stats struct {
    Pending int
    Running int
    Failed  int
}

// HandlerFunc processes a job. Queue implementations call the registered handler
// for each job kind.
type HandlerFunc func(ctx context.Context, payload []byte) error
```

```go
// queue/river.go — River implementation (PostgreSQL deployments)
func NewRiverQueue(pool *pgxpool.Pool, cfg config.Workers, handlers map[string]HandlerFunc) (Queue, error)
```

Concurrency config (`cfg.Workers.Reindex`, `cfg.Workers.Embed`) flows through to the implementation. River maps these to per-job-type concurrency limits. A future in-memory implementation would map them to goroutine pool sizes. The config is backend-agnostic; the implementation decides what it means.

### index / embed handlers

Handler functions — pure business logic, no queue coupling. They receive a payload, do their work, and return. The queue calls them.

```go
// index/handler.go
type ReindexJob struct {
    SourceURL string `json:"source_url"`
}

// ReindexHandler returns a queue.HandlerFunc that fetches, chunks, and diffs a source.
// It enqueues embed jobs for new/changed chunks via the provided Queue.
func ReindexHandler(s store.Store, q queue.Queue, cfg config.GitLab) queue.HandlerFunc

// embed/handler.go
type EmbedJob struct {
    ContentHash []byte `json:"content_hash"`
    Content     string `json:"content"`
}

// EmbedHandler returns a queue.HandlerFunc that embeds a chunk via the configured provider.
func EmbedHandler(v store.VectorStore, cfg config.Embedding) queue.HandlerFunc
```

`ReindexHandler` takes a `Queue` to enqueue embed jobs after chunking. `EmbedHandler` doesn't enqueue anything — it's a leaf.

## Dependency Graph

```
cmd/kb/main.go
  └── cli/
        ├── root.go ──→ config/
        │
        ├── serve.go ──→ server/ ──→ agent/ ──→ store/
        │                  │
        │                  └── auth (tsnet)
        │
        │              ┌── queue/ (interface)
        │              │     ├── river.go (PostgreSQL impl)
        │              │     │
        │              │     ├── index/ ──→ store/, queue/ (enqueues embed jobs)
        │              │     └── embed/ ──→ store/
        │              │
        │              └── session/
        │
        ├── sources.go ──→ store/
        ├── reindex.go ──→ queue/ (enqueue via Queue interface)
        ├── setup.go ────→ store/ (migrations, checks)
        └── status.go ───→ store/, queue/
```

Key property: **config/, store/, queue/, session/, agent/, index/, embed/, server/ have no circular dependencies.** Each can be implemented and tested independently once the interfaces exist.

## Composition Root

`cli/serve.go` wires everything together. Config sub-structs flow to constructors:

```go
func runServe(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    cfg := configFromContext(ctx) // loaded in root.go PersistentPreRun

    // 1. Store (PGStore implements both Store and VectorStore)
    pgStore, _ := store.NewPGStore(ctx, cfg.Database)

    // 2. Migrations
    pgStore.Migrate(ctx)

    // 3. Queue (River, backed by PGStore's pool)
    q, _ := queue.NewRiverQueue(pgStore.Pool(), cfg.Workers, map[string]queue.HandlerFunc{
        "reindex": index.ReindexHandler(pgStore, nil, cfg.GitLab), // q injected below
        "embed":   embed.EmbedHandler(pgStore, cfg.Embedding),
    })
    // ReindexHandler needs Queue to enqueue embed jobs — resolve the cycle:
    // option a: two-phase init (register handlers after queue construction)
    // option b: pass Queue via closure capture
    // The boilerplate will use two-phase init for clarity.

    // 4. Session manager
    sessions := session.NewLRUManager(cfg.Sessions)

    // 5. Agent
    ag := agent.New(pgStore, pgStore, cfg.Agent) // satisfies Store + VectorStore

    // 6. Server (tsnet)
    srv := server.New(ag, sessions, q, cfg.Tailscale)
    return srv.Serve(ctx)
}
```

## Logging

`log/slog` throughout. Handler selection at startup in `cli/root.go`:

```go
func setupLogging(cfg config.Log) {
    level := parseLevel(cfg.Level) // maps string → slog.Level
    var handler slog.Handler
    switch cfg.Format {
    case "journal":
        handler = journal.NewHandler(journal.WithLevel(level))
    default:
        handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
    }
    slog.SetDefault(slog.New(handler))
}
```

Packages use `slog.Default()`. No custom logger abstraction, no logger injection.

## Testing

Ginkgo + Gomega for BDD specs. Specs describe **behaviors**, not methods — organized around what the system does, not how it's implemented.

### Suite structure

Each package has a Ginkgo suite. Specs use `Describe`/`Context`/`It` to express behaviors at the appropriate scope:

```
internal/store/
  store_suite_test.go                  # suite bootstrap, PostgreSQL test container setup
  chunks_test.go                       # Describe("Chunk storage", ...)
  provenance_test.go                   # Describe("Provenance tracking", ...)
  sources_test.go                      # Describe("Source management", ...)
  pgvector_test.go                     # Describe("Vector similarity search", ...)

internal/queue/
  queue_suite_test.go                  # suite bootstrap
  river_test.go                        # Describe("River queue", ...) — requires PostgreSQL

internal/agent/
  agent_suite_test.go                  # suite bootstrap
  agent_test.go                        # Describe("KB agent", ...) — store + vector fakes

internal/index/
  index_suite_test.go                  # suite bootstrap
  chunker_test.go                      # Describe("Markdown chunking", ...)
  fetcher_test.go                      # Describe("Source fetching", ...) — HTTP test server
  handler_test.go                      # Describe("Reindex handler", ...) — store + queue fakes

internal/embed/
  embed_suite_test.go                  # suite bootstrap
  handler_test.go                      # Describe("Embed handler", ...) — vector store fake

internal/session/
  session_suite_test.go                # suite bootstrap
  lru_test.go                          # Describe("LRU session cache", ...)
  jsonl_test.go                        # Describe("JSONL persistence", ...) — temp directory

internal/server/
  server_suite_test.go                 # suite bootstrap
  ask_test.go                          # Describe("Query endpoint", ...) — agent + session fakes
  webhook_test.go                      # Describe("Webhook endpoint", ...) — queue fake
  auth_test.go                         # Describe("Authorization", ...) — tsnet fake/disabled mode

internal/config/
  config_suite_test.go                 # suite bootstrap
  config_test.go                       # Describe("Configuration loading", ...)
```

### Spec style

```go
var _ = Describe("Chunk storage", func() {
    Context("when upserting a chunk", func() {
        It("stores content addressable by BLAKE3 hash", func() { ... })
        It("deduplicates identical content across sources", func() { ... })
    })

    Context("when cleaning up orphans", func() {
        It("removes chunks with no remaining provenance", func() { ... })
        It("preserves chunks still referenced by other sources", func() { ... })
    })
})
```

Specs read as behavioral documentation. A failing spec tells you *what behavior broke*, not just *what function returned an error*.

### Test scopes

| Scope | Packages | External deps | Strategy |
|-------|----------|---------------|----------|
| **Behavioral (DB)** | store/, queue/ | PostgreSQL + pgvector | Test container via [testcontainers-go](https://golang.testcontainers.org/) or similar. Real database, real queries. |
| **Behavioral (HTTP)** | index/ (fetcher) | GitLab API | `httptest.Server` returning canned tarballs. |
| **Behavioral (FS)** | session/ | Filesystem | `GinkgoT().TempDir()` per spec. |
| **Behavioral (unit)** | agent/, index/ (chunker, handler), embed/, server/, config/ | None | Interface fakes for store, vector store, queue, agent, session. |

No mocking frameworks — hand-written fakes that implement the interfaces. They're small (3-12 methods each) and serve as living documentation of expected behavior.

### Fixtures

```
internal/index/testdata/
  simple.md                            # basic headings, single chunk
  multi-section.md                     # multiple H2s, multiple chunks
  oversized-section.md                 # single section exceeding token max
  frontmatter.md                       # YAML frontmatter to skip
  empty.md                             # empty file edge case

internal/index/testdata/archives/
  sample.tar.gz                        # minimal repo tarball for fetcher specs

internal/server/testdata/
  llms.txt                             # expected /llms.txt content
```

Fixtures live in `testdata/` within the package that uses them (Go tooling ignores `testdata/` directories by convention).

### Running specs

```makefile
test:
	ginkgo ./...

test-unit:
	ginkgo --label-filter='!db' ./...

test-verbose:
	ginkgo -v ./...
```

Specs requiring a database are labeled so they can be skipped for fast feedback during development: `Label("db")` on the relevant `Describe` blocks.

## Parallelization Plan

Once the boilerplate is committed (main.go, cli/ skeleton, config types, interfaces, Ginkgo suites), these work streams are independent:

| Stream | Packages | Depends on |
|--------|----------|------------|
| **Database** | store/ (pgstore, migrations) | config types |
| **Queue** | queue/ (river) | config types, store concrete type (for Pool) |
| **Indexing** | index/ (fetcher, chunker, handler) | store interface, queue interface, config types |
| **Embedding** | embed/ (handler) | vector store interface, config types |
| **Agent** | agent/ (prompt, tools, executor) | store + vector store interfaces, session types, config types |
| **Server** | server/ (tsnet, routes, auth) | agent interface, session interface, queue interface, config types |
| **Sessions** | session/ | config types |
| **CLI** | cli/ (sources, reindex, setup, status) | store interface, queue interface, config types |

The boilerplate commit delivers: module init, package directories, config types, all interface definitions (`Store`, `VectorStore`, `Queue`, `Manager`, `Agent`), placeholder implementations returning `errors.New("not implemented")`, the cobra command tree, and Ginkgo suite files.

## Migrations

Two migration systems for the PostgreSQL deployment, both run via `PGStore.Migrate()`:

- **golang-migrate** for KB tables (chunks, embeddings, provenance, sources, pgvector extension). SQL files in `store/migrations/`, embedded via `go:embed`. Numbered up/down pairs.
- **rivermigrate** for River's job tables. River owns its schema — we don't copy it into our migrations, avoiding breakage on River version bumps.

Both run at `kb serve` startup (migrate-on-boot, fine for single-instance) and during `kb setup`. `Migrate()` lives on the concrete `PGStore` type, not the `Store` interface — it's an infrastructure concern specific to the PostgreSQL deployment. A future SQLite store would have its own migration strategy.

## What This Plan Doesn't Cover

- SQL migration content (database stream)
- Chunking algorithm details (indexing stream)
- LangChainGo agent wiring specifics (agent stream)
- Container image / deployment (separate concern)
