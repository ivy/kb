# 002: Indexing Pipeline

Goal: implement the complete path from "GitLab repo URL" to "content-addressed chunks with provenance in PostgreSQL." After this plan, `kb reindex <url>` fetches a repo, chunks its Markdown, and lands data in the database. Embedding is deferred to plan 003.

## Package Structure

The boilerplate put chunker, source adapter, and handler together in `internal/index/`. That violates SRP — these are three distinct responsibilities with different reasons to change:

- **Chunker** changes when chunking strategy changes (new content types, token limits, heading parsing)
- **Source adapter** changes when source types change (GitHub support, local directories, API changes)
- **Handler** changes when orchestration logic changes (diff strategy, what gets enqueued)

Split into three packages:

```
internal/
├── chunk/                    # content → structured chunks
│   ├── markdown.go           # ChunkMarkdown() — structure-based Markdown chunking
│   ├── chunk.go              # Chunk type, shared types
│   ├── chunk_suite_test.go
│   └── markdown_test.go      # Markdown chunking specs
│
├── source/                   # source URL → file tree (adapters per source type)
│   ├── gitlab.go             # FetchGitLab() — GitLab archive API
│   ├── source.go             # Result type, Fetcher function type
│   ├── source_suite_test.go
│   └── gitlab_test.go        # GitLab source adapter specs
│
├── index/                    # orchestration: source → chunk → diff → persist
│   ├── handler.go            # ReindexHandler — wires source + chunk + store
│   ├── index_suite_test.go
│   ├── handler_test.go       # Reindex pipeline specs
│   └── fake_test.go          # FakeStore, FakeQueue for handler tests
│
├── store/                    # (existing — add migrations + implement methods)
│   ├── migrations/
│   │   ├── 001_initial.up.sql
│   │   └── 001_initial.down.sql
│   └── ...
```

The old `internal/index/chunker.go` and `internal/index/fetcher.go` move to their new packages. `testdata/` fixtures move with the chunker (`internal/chunk/testdata/`) and source adapter (`internal/source/testdata/`).

**Why this matters now:** The architecture doc already anticipates tree-sitter code chunking as a future content type. With `internal/chunk/`, adding `code.go` alongside `markdown.go` is natural. With `internal/source/`, adding `directory.go` or `github.go` alongside `gitlab.go` is the same pattern. With everything in `index/`, both would be a refactor.

## Scope

**In scope:**
- Markdown chunker (`internal/chunk/`) — structure-based, BLAKE3 hashing, cl100k_base token counting
- GitLab source adapter (`internal/source/`) — tarball download + extraction
- Reindex handler (`internal/index/`) — orchestrates source → chunk → diff → persist
- PGStore: `NewPGStore`, `Migrate`, and the subset of `Store` methods the handler needs
- SQL migrations for chunks, provenance, sources tables
- Ginkgo specs for all of the above

**Out of scope (later plans):**
- Embedding pipeline (plan 003 — depends on chunks existing)
- River queue wiring (plan 003 or 004 — handler is callable directly for now)
- Webhook endpoint (plan 004 — needs queue)
- Agent, server, sessions (later plans)
- pgvector extension / embeddings table (plan 003)

## Dependencies to Add

```
go get github.com/zeebo/blake3          # BLAKE3 hashing
go get github.com/pkoukk/tiktoken-go    # OpenAI cl100k_base tokenizer
go get github.com/golang-migrate/migrate/v4  # schema migrations
go get github.com/testcontainers/testcontainers-go  # PostgreSQL test containers
```

## Implementation Order

### Step 1: Chunker (`internal/chunk/`)

The most self-contained piece. Pure functions, no I/O, testable against fixtures.

#### Types (`internal/chunk/chunk.go`)

```go
// Chunk represents a semantically meaningful piece of a document.
type Chunk struct {
    Content          string   // the chunk text
    ContentHash      []byte   // BLAKE3 hash of Content
    TokenCount       int      // exact cl100k_base token count
    HeadingHierarchy []string // e.g., ["Architecture", "Data Model", "Chunks"]
    Position         int      // 0-indexed position within the file
}
```

#### Function (`internal/chunk/markdown.go`)

```go
// ChunkMarkdown splits a Markdown document into structure-based chunks.
// Uses cl100k_base tokenizer for accurate token counting.
// Chunks are split on heading boundaries; sections exceeding 4096 tokens
// are split on paragraph boundaries as a fallback.
func ChunkMarkdown(content []byte) ([]Chunk, error)
```

`maxTokens` is not a parameter — it's fixed at 4096 (tied to the embedding model). If we change embedding models, we change the constant. No need for runtime configurability here.

#### Chunking parameters

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Max tokens | 4096 | Half of `text-embedding-3-large`'s 8191 limit — better granularity for retrieval |
| Encoding | `cl100k_base` | OpenAI's tokenizer for `text-embedding-3-large` — exact token counts, no surprises at embedding time |
| Overlap | 0 | Two reasons. **Hashing**: overlap couples adjacent chunks — changing chunk N alters the overlap in chunk N+1, changing its BLAKE3 hash, triggering unnecessary re-embedding. **Agent architecture**: overlap exists to help dumb retrievers that return top-k chunks with no follow-up. KB's agent has `read` and `list` tools — it can fetch surrounding chunks or the full document when a single chunk isn't self-contained. Heading hierarchy metadata tells the agent where in the document structure a chunk sits, which is more useful than 150 tokens of duplicated text. |

Token counting uses `tiktoken-go` with the `cl100k_base` encoding — the same tokenizer the embedding model uses. This ensures chunks never exceed the model's token limit and chunk boundaries are deterministic regardless of content encoding quirks.

The v0 prototype (see `archive/v0-prototype/docs/plans/003-storage/001-chunking-strategy.md`) used the same tokenizer and max token value. The key difference: v0 had 150-token overlap, which we drop here because it conflicts with the content-addressable dedup model.

#### Chunking algorithm

1. Strip YAML frontmatter (between `---` fences at document start)
2. Split on heading boundaries (`# `, `## `, `### `, etc.)
3. Each section becomes a chunk, inheriting the heading hierarchy from its ancestors
4. Sections exceeding 4096 tokens are split on paragraph boundaries (`\n\n`), each sub-chunk retaining the same heading hierarchy
5. Compute BLAKE3 hash of each chunk's content
6. Assign position indices (0, 1, 2, ...) in document order

#### Specs (`internal/chunk/markdown_test.go`)

Specs describe *what the chunker guarantees*, not how it's implemented. The determinism context is first because content-addressable hashing depends on it — if chunking isn't deterministic, the entire diff/dedup model breaks.

```go
Describe("Markdown chunking", func() {
    Describe("determinism", func() {
        // This is the load-bearing property. Content-addressable hashing means
        // the same input MUST produce the same chunks with the same hashes,
        // every time. If this breaks, unchanged content gets re-embedded.

        It("produces identical chunks and hashes for the same input across calls", func() { ... })
        It("produces identical chunks regardless of surrounding file content", func() {
            // Chunk boundaries are structural (headings), not positional.
            // Appending a new section must not change chunks for existing sections.
        })
    })

    Describe("document structure", func() {
        Context("when the document has a single section", func() {
            It("produces one chunk containing the section content", func() { ... })
            It("records the heading as the hierarchy root", func() { ... })
        })

        Context("when the document has multiple sections", func() {
            It("produces one chunk per heading-delimited section", func() { ... })
            It("assigns position indices in document order", func() { ... })
            It("tracks each section's heading hierarchy independently", func() { ... })
        })

        Context("when headings are nested (H1 > H2 > H3)", func() {
            It("builds the full ancestor path for each chunk", func() { ... })
        })
    })

    Describe("content hashing", func() {
        It("computes a BLAKE3 hash of the chunk content", func() { ... })
        It("produces different hashes for different content", func() { ... })
        It("produces the same hash for identical content in different files", func() {
            // This is what makes cross-source dedup work.
        })
    })

    Describe("token counting", func() {
        It("reports the exact cl100k_base token count for each chunk", func() { ... })
        It("never produces a chunk exceeding 4096 tokens", func() { ... })
    })

    Describe("oversized sections", func() {
        Context("when a section exceeds the 4096 token limit", func() {
            It("splits on paragraph boundaries", func() { ... })
            It("preserves the heading hierarchy on every sub-chunk", func() { ... })
            It("keeps every resulting chunk within the token limit", func() { ... })
        })
    })

    Describe("edge cases", func() {
        It("strips YAML frontmatter before chunking", func() { ... })
        It("returns zero chunks for an empty document", func() { ... })
    })
})
```

Move existing `internal/index/testdata/` fixtures to `internal/chunk/testdata/`. Add a `nested-headings.md` fixture for the hierarchy specs.

### Step 2: Source Adapter (`internal/source/`)

Retrieves files from a content source. GitLab is the first adapter; the package is structured so adding `github.go` or `directory.go` later follows the same pattern.

#### Types (`internal/source/source.go`)

```go
// Result contains the extracted files and metadata from a source.
type Result struct {
    Ref   string            // commit SHA or ref that was fetched
    Files map[string][]byte // relative path → file content (Markdown files only)
}

// Fetcher retrieves files from a source. Abstracted as a function type so
// the reindex handler can be tested without I/O.
type Fetcher func(ctx context.Context, sourceURL, token string) (*Result, error)
```

#### Function (`internal/source/gitlab.go`)

```go
// FetchGitLab downloads and extracts Markdown files from a GitLab repo archive.
// Uses the repository archive API (/api/v4/projects/:id/repository/archive.tar.gz).
// Satisfies the Fetcher function type.
func FetchGitLab(ctx context.Context, sourceURL, token string) (*Result, error)
```

#### Implementation

1. Parse `sourceURL` to derive the GitLab API project path (URL-encoded)
2. `GET /api/v4/projects/:id/repository/archive.tar.gz` with `PRIVATE-TOKEN` header
3. Stream the response through `gzip.NewReader` → `tar.NewReader`
4. Walk tar entries, collect `.md` files (skip directories, non-Markdown, files in `.git/`)
5. Strip the archive's top-level directory prefix (GitLab tarballs have a `project-ref-sha/` root)
6. Extract the commit ref from the response headers or archive directory name
7. Return `Result` with relative paths and content

#### Specs (`internal/source/gitlab_test.go`)

All specs use `httptest.Server` returning canned responses — no real GitLab calls.

```go
Describe("GitLab source adapter", func() {
    Describe("archive extraction", func() {
        It("returns only Markdown files from the archive", func() { ... })
        It("uses relative paths with the archive root prefix stripped", func() { ... })
        It("captures the commit ref from the archive", func() { ... })
    })

    Describe("authentication", func() {
        It("sends the configured token via the PRIVATE-TOKEN header", func() { ... })
    })

    Describe("error handling", func() {
        Context("when the GitLab API returns a non-200 status", func() {
            It("returns an error containing the status code and URL", func() { ... })
        })

        Context("when the archive is malformed", func() {
            It("returns an error without panicking", func() { ... })
        })
    })
})
```

Move `internal/index/testdata/archives/` to `internal/source/testdata/`. Rebuild `sample.tar.gz` to contain Markdown files with known content so specs can assert on exact file paths and content.

### Step 3: Store — Migrations + Indexing Methods

Implement `NewPGStore`, `Migrate`, and the subset of `Store` methods the reindex handler calls. Leave query-time methods (`LookupByKeyword`, `GetDocument`) and vector methods for later plans.

#### SQL Migrations (`internal/store/migrations/001_initial.up.sql`)

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE chunks (
    content_hash BYTEA PRIMARY KEY,
    content      TEXT NOT NULL
);

CREATE TABLE sources (
    url             TEXT PRIMARY KEY,
    kind            TEXT NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}',
    last_indexed_ref TEXT,
    last_indexed_at  TIMESTAMPTZ
);

CREATE TABLE provenance (
    content_hash BYTEA NOT NULL REFERENCES chunks(content_hash),
    source_url   TEXT  NOT NULL REFERENCES sources(url),
    file_path    TEXT  NOT NULL,
    position     INT   NOT NULL DEFAULT 0,
    metadata     JSONB NOT NULL DEFAULT '{}',
    indexed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    ref          TEXT,
    PRIMARY KEY (content_hash, source_url, file_path)
);

CREATE INDEX idx_provenance_source ON provenance(source_url);
```

Down migration:

```sql
DROP TABLE IF EXISTS provenance;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS chunks;
```

Note: the `embeddings` table and `pgvector` extension are deferred to plan 003. This keeps migrations focused on what's needed now.

#### Provenance struct update

The current `store.Provenance` struct is missing fields from the architecture doc. Update to match:

```go
type Provenance struct {
    SourceURL   string
    FilePath    string
    ContentHash []byte
    Position    int
    Metadata    json.RawMessage // heading hierarchy, etc.
    Ref         string          // commit SHA at index time
}
```

The `Title` field in the current struct should be removed — heading hierarchy belongs in `Metadata` as JSONB, not a flat string.

#### PGStore methods to implement

| Method | Used by reindex handler | Notes |
|--------|------------------------|-------|
| `NewPGStore` | yes (composition root) | `pgxpool.New(ctx, cfg.URL)` |
| `Migrate` | yes (`kb setup`, `kb serve`) | golang-migrate with embedded SQL |
| `UpsertChunk` | yes | `INSERT ... ON CONFLICT DO NOTHING` (content-addressable, immutable) |
| `UpsertProvenance` | yes | `INSERT ... ON CONFLICT DO UPDATE` (same hash+source+path = update metadata/ref/indexed_at) |
| `DeleteProvenance` | yes | `DELETE FROM provenance WHERE source_url = $1 AND content_hash = $2` |
| `DeleteOrphanedChunks` | yes | `DELETE FROM chunks WHERE content_hash NOT IN (SELECT content_hash FROM provenance)` |
| `AddSource` | not yet (CLI) | implement now — needed to have sources in DB before reindex |
| `ListSources` | not yet (CLI) | implement now — simple, useful for testing |
| `UpdateSourceIndexed` | yes | `UPDATE sources SET last_indexed_ref = $2, last_indexed_at = now() WHERE url = $1` |
| `RemoveSource` | not yet (CLI) | defer |
| `LookupByKeyword` | no (query time) | defer |
| `GetDocument` | no (query time) | defer |

#### Specs (`internal/store/` — requires PostgreSQL)

All store specs get `Label("db")` so `make test-unit` skips them.

```go
// store_suite_test.go — testcontainers-go setup
// Starts a pgvector/pg17 container, runs migrations, provides pool to specs.
// Each Describe gets a clean database (truncate between specs).

Describe("PGStore", Label("db"), func() {
    Describe("content-addressable chunk storage", func() {
        It("stores a chunk retrievable by its content hash", func() { ... })
        It("silently deduplicates when the same content hash is upserted twice", func() { ... })

        Context("when a chunk has no remaining provenance", func() {
            It("is removed by orphan cleanup", func() { ... })
        })

        Context("when a chunk is still referenced by at least one provenance record", func() {
            It("survives orphan cleanup", func() { ... })
        })
    })

    Describe("provenance tracking", func() {
        It("links a chunk to its source file and position", func() { ... })
        It("carries metadata (heading hierarchy) and the indexed ref", func() { ... })

        Context("when the same chunk appears at the same path in the same source", func() {
            It("updates metadata and indexed_at rather than creating a duplicate", func() { ... })
        })

        Context("when provenance is deleted for a source", func() {
            It("removes only provenance for that source, leaving other sources intact", func() { ... })
        })

        It("lists all provenance records for a given source", func() { ... })
    })

    Describe("source management", func() {
        It("persists a source with its URL, kind, and config", func() { ... })
        It("lists all registered sources", func() { ... })

        Context("after a successful reindex", func() {
            It("records the indexed ref and timestamp on the source", func() { ... })
        })
    })
})
```

### Step 4: Reindex Handler (`internal/index/handler.go`)

Wires source adapter + chunker + store together. This is where the pipeline orchestration lives — it depends on `chunk.Chunk`, `source.Fetcher`, `store.Store`, and `queue.Queue` but contains no chunking logic, no HTTP calls, and no SQL.

#### Implementation

```go
func ReindexHandler(s store.Store, q queue.Queue, fetch source.Fetcher, cfg config.GitLab) queue.HandlerFunc {
    return func(ctx context.Context, payload []byte) error {
        // 1. Decode payload → ReindexJob
        // 2. Fetch: fetch(ctx, job.SourceURL, cfg.Token) → source.Result
        // 3. Chunk: for each file in result.Files, chunk.ChunkMarkdown(content)
        // 4. Collect all current chunk hashes for this source
        // 5. Diff: compare fetched chunks against existing provenance
        // 6. Upsert new/changed chunks and provenance
        // 7. Delete provenance for chunks no longer present
        // 8. Clean up orphaned chunks
        // 9. Enqueue embed jobs for new/changed chunks (via q.Enqueue)
        // 10. Update source last_indexed_ref
    }
}
```

The handler needs a way to query existing provenance for a source to compute the diff. This requires one more `Store` method not in the current interface:

```go
// ListProvenance returns all provenance records for a source.
// Used by the reindex handler to diff against fetched content.
ListProvenance(ctx context.Context, sourceURL string) ([]Provenance, error)
```

Add this to the `Store` interface. It's a read used during indexing, not query time.

#### Specs (`internal/index/handler_test.go`)

Uses `FakeStore`, `FakeQueue`, and a fake `source.Fetcher` — no database, no HTTP, no external dependencies. These specs describe the orchestration guarantees of the pipeline.

```go
Describe("Reindex pipeline", func() {
    Describe("first-time indexing of a source", func() {
        It("persists every chunk from every Markdown file", func() { ... })
        It("creates provenance linking each chunk to its source file and position", func() { ... })
        It("enqueues an embed job for every new chunk", func() { ... })
        It("records the fetched ref on the source", func() { ... })
    })

    Describe("re-indexing unchanged content", func() {
        // The critical efficiency property: if nothing changed in the repo,
        // no embedding work is enqueued. This depends on chunker determinism.
        It("enqueues zero embed jobs", func() { ... })
        It("refreshes provenance timestamps without creating new records", func() { ... })
    })

    Describe("re-indexing after content changes", func() {
        Context("when a file has been modified", func() {
            It("enqueues embed jobs only for chunks whose content hash changed", func() { ... })
        })

        Context("when a file has been removed", func() {
            It("deletes provenance for the removed file's chunks", func() { ... })
            It("cleans up chunks that no longer have any provenance", func() { ... })
        })

        Context("when a file has been renamed without content changes", func() {
            // Content-addressable: same content = same hash = no re-embedding.
            It("creates new provenance at the new path without re-embedding", func() { ... })
        })
    })
})
```

### Step 5: Update Fakes and Wiring

- Update `FakeStore` in `internal/store/fake_test.go` to support `ListProvenance`
- Create `internal/index/fake_test.go` with copies of `FakeStore` and `FakeQueue` (Go test packages can't import test files from other packages)
- Update `internal/cli/serve.go` to import `source` and pass `source.FetchGitLab` to `ReindexHandler`
- Update `internal/cli/setup.go` to actually call `Migrate`
- Remove the old `internal/index/chunker.go` and `internal/index/fetcher.go` stubs
- Move `internal/index/testdata/` to `internal/chunk/testdata/` and `internal/index/testdata/archives/` to `internal/source/testdata/`

## Type Changes Summary

Changes to existing types from the boilerplate:

| Type | Change | Reason |
|------|--------|--------|
| `store.Provenance` | Add `Position int`, `Metadata json.RawMessage`, `Ref string`; remove `Title` | Match architecture doc data model |
| `store.Store` | Add `ListProvenance(ctx, sourceURL) ([]Provenance, error)` | Reindex handler needs to diff against existing state |
| `index.Chunk` → `chunk.Chunk` (new package) | Replace bare `[][]byte` return | Chunker returns structured data with hashes and heading hierarchy |
| `index.Fetch` → `source.Result` (new package) | Replace bare `[]byte` return | Source adapter returns file tree + commit ref |
| `index.ReindexHandler` | Now depends on `source.Fetcher`, `chunk.ChunkMarkdown` | Clean dependency boundaries between packages |

## New Dependencies

| Dependency | Used by | Purpose |
|------------|---------|---------|
| `github.com/zeebo/blake3` | chunk/ | Content-addressable hashing |
| `github.com/pkoukk/tiktoken-go` | chunk/ | cl100k_base tokenizer for accurate token counting |
| `github.com/golang-migrate/migrate/v4` | store/ | Schema migrations |
| `github.com/testcontainers/testcontainers-go` | store/ tests | PostgreSQL test container |

## Implementation Sequence

```
1. chunk/           ← pure logic, no I/O, fastest to validate
   ↓
2. source/          ← HTTP only, testable with httptest
   ↓ (parallel with 3)
3. store/ migrations + PGStore methods  ← needs testcontainers setup
   ↓
4. index/ handler   ← wires chunk + source + store together
   ↓
5. Wiring updates   ← serve.go, setup.go, fakes, remove old stubs
```

Steps 2 and 3 can proceed in parallel once the chunker establishes the `Chunk` type.

## What This Enables

After this plan:
- `kb setup` creates the database schema
- `kb sources add <url>` registers a source
- The reindex handler can be called directly (or via `kb reindex <url>` once queue is wired) to fetch, chunk, and persist a GitLab repo's Markdown content
- Chunks are content-addressed, provenance tracks source locations, orphans get cleaned up
- The store has real data for the embedding pipeline (plan 003) and agent (later) to consume

## What This Doesn't Cover

- Embedding pipeline (plan 003)
- River queue wiring and worker lifecycle (plan 003 or 004)
- Webhook endpoint for push-triggered reindex (plan 004)
- Query-time store methods (`LookupByKeyword`, `GetDocument`)
- Agent, server, sessions
