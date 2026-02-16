# Phase 2: Storage

**Goal:** Build minimal viable vector search: markdown files → chunks → embeddings → ranked search results.

## The Big Picture

```
Markdown file → Chunks → Embeddings → Vector DB → Formatted Results
     ↓            ↓          ↓            ↓             ↓
  people/       "# Notes   [0.23,      SQLite +    Chunk excerpts
  alice.md      > Alice     0.45,      sqlite-vec  with scores
                handles     ...]                   (0.0-1.0)
                security"
```

**End state:** `curl Query "Who handles security?"` → get back formatted list of matching chunks with file paths and similarity scores.

## Key Simplifications

This is a **prototype**, not production software:

1. **No note storage** - files stay on disk, DB only stores chunks with `file_path` metadata
2. **Raw search results** - Query returns formatted chunk list, not AI-synthesized answers (that's Phase 4)
3. **Single hardcoded vault** - multi-vault support is Phase 3
4. **Manual seeding** - run a script once to populate DB, no automatic sync
5. **Minimal abstractions** - direct implementations, no interfaces until needed

## Simplified Schema

```sql
-- Vault metadata (one row for now)
CREATE TABLE vault (
    id INTEGER PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL
);

-- Chunks (no note table - files stay on disk)
CREATE TABLE chunk (
    id INTEGER PRIMARY KEY,
    vault_id INTEGER NOT NULL REFERENCES vault(id),
    file_path TEXT NOT NULL,        -- e.g., "people/alice.md"
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,           -- Heading context + chunk text
    token_count INTEGER NOT NULL,
    UNIQUE(vault_id, file_path, chunk_index)
);

-- Vector embeddings (rowid maps to chunk.id)
CREATE VIRTUAL TABLE chunk_vec USING vec0(
    embedding float[3072]
);
```

## Build Steps

### 001: Chunking + Test Vault

**Deliverables:**
- `internal/chunk` package - heading-aware markdown splitter
- `testdata/vault/` - 5 test markdown files
- Unit tests

**What it does:** 4096 token chunks with 150 token overlap, heading paths prepended

**Time:** 1-2 hours

---

### 002: Database + Manual Seed

**Deliverables:**
- `internal/db` package - SQLite + sqlite-vec setup
- `cmd/seed-testdata/main.go` - manual seeding script
- Simple schema creation (no migrations framework)

**What it does:**
- Create vault + chunk + chunk_vec tables
- Insert chunked test vault (with zero embeddings initially)
- Run manually: `go run cmd/seed-testdata/main.go`

**Time:** 2-3 hours

---

### 003: Embeddings

**Deliverables:**
- `internal/embed` package - OpenAI API client
- Single-embedding support (batching can wait)

**What it does:** `Embed(text) → []float32` using text-embedding-3-large

**Time:** 1 hour

---

### 004: Search + Query RPC

**Deliverables:**
- `internal/search` package - query embedding + KNN + formatting
- Wire `internal/server.Query()` to return real results
- Update seed script to embed chunks (run once with API key)

**What it does:**
```
Query "Who handles security?"
→ Embed query
→ KNN search
→ Format results:

Found 2 relevant chunks:

1. people/alice.md (chunk 0, score: 0.89)
   "Alice handles security and infrastructure..."

2. meetings/2026-01.md (chunk 1, score: 0.76)
   "Discussed security concerns with Alice..."

Citations:
- people/alice.md
- meetings/2026-01.md
```

**Time:** 2-3 hours

---

### 005: Basic Polish (Optional)

**Deliverables:**
- Error handling (don't crash on missing files)
- Query logging

**Skip:** Automatic seeding, idempotency, file watching, performance tuning (Phase 3)

**Time:** 30 minutes

## Validation

After completion:

```bash
# One-time: seed the test vault
OPENAI_API_KEY=xxx go run cmd/seed-testdata/main.go

# Start server
kb-server

# Query
curl --unix-socket /tmp/kb.sock \
  http://localhost/kb.v1.KnowledgeBaseService/Query \
  -H 'Content-Type: application/json' \
  -d '{"question": "Who handles security?"}'
```

Get back formatted chunk results. **That's Phase 2 complete.**

## What This Doesn't Do

- Automatic seeding or change detection (Phase 3)
- Multi-vault support (Phase 3)
- LLM-synthesized answers (Phase 4)
- Production error handling, monitoring, optimization
- Smart excerpts or result ranking beyond cosine similarity

Keep it simple. Prove vector search works. Build production features later.

## References

- [ADR-003: Vector Store Selection](../../adrs/003-vector-store-selection.md) - why sqlite-vec
- [Embedding Benchmark](../../research/embedding-benchmark.md) - cost/token analysis
