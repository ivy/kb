# MVP Architecture

How KB indexes knowledge and serves it to callers. Read [vision.md](../vision.md) for why, [user-stories.md](../user-stories.md) for what, this doc for how.

## System Overview

```
GitLab push → system hook → KB webhook endpoint → River job queue
                                                        │
                                              ┌─────────┴─────────┐
                                              │                   │
                                         repo reindex        embedding
                                           workers            workers
                                              │                   │
                                              └─────────┬─────────┘
                                                        │
                                                   PostgreSQL
                                                (pgvector + River)
                                                        │
                                                   KB Agent
                                                  (query time)
                                                        │
                                                     Caller
```

Single Go binary. Single PostgreSQL database. No external queue, no Redis, no second datastore.

## Indexing Pipeline

### Trigger

A GitLab [system hook](https://docs.gitlab.com/administration/system_hooks/) fires on every push across the instance. KB receives the webhook, checks the source against its configured list, and either enqueues a reindex job or ignores the event.

Initial backfill for a newly added source is the same pipeline — the first push after configuration triggers a full index.

### Repo Reindex Job

A River job that:

1. Fetches the current HEAD ref of the default branch from GitLab
2. Downloads and unpacks a tarball archive (no git clone, no history)
3. Walks the tree for Markdown files
4. Chunks each file deterministically based on document structure (headings, sections — no sliding windows)
5. Computes content hashes for each chunk
6. Diffs against existing chunks for this source
7. Enqueues embedding jobs for new/changed chunks
8. Removes provenance for chunks no longer present
9. Cleans up orphaned chunks (no remaining provenance references)

Concurrency is controllable — a sane default (e.g., 2-4 repo workers) prevents overwhelming GitLab or the database during bulk reindex.

### Embedding Job

A separate River job type that:

1. Takes a chunk content hash and text
2. Calls the OpenAI embedding API
3. Upserts an embedding row (content_hash, model, vector) into pgvector

Separate job type = separate concurrency control. Embedding workers are rate-limited by the OpenAI API, not by KB's CPU. River manages retries and backpressure.

## Data Model

### Chunks

Content-addressable. The content hash is the identity.

| Column | Purpose |
|--------|---------|
| `content_hash` | BLAKE3 of chunk text — primary identity |
| `content` | The chunk text |

If two files in different sources contain identical content, it's one chunk. Deduplication is free.

### Embeddings

Decoupled from chunks to support model changes and multiple models.

| Column | Purpose |
|--------|---------|
| `content_hash` | FK to chunks |
| `model` | Model identifier (e.g., `text-embedding-3-large`) |
| `embedding` | pgvector vector |

Composite key on `(content_hash, model)`. MVP uses `text-embedding-3-large` only. Swapping models means deleting old embedding rows and re-enqueuing jobs — chunks and provenance are untouched.

### Provenance

Maps chunks to their source locations. Many provenance records can point to one chunk.

| Column | Purpose |
|--------|---------|
| `content_hash` | FK to chunks |
| `source_url` | Source URL (FK to sources) |
| `file_path` | Path within the source |
| `position` | Chunk's position within the file (ordering) |
| `metadata` | JSONB — chunker-specific context (e.g., `{"heading_hierarchy": ["Architecture", "Data Model"]}` for Markdown, `{"symbol": "func NewClient", "package": "kb"}` for code) |
| `indexed_at` | When this provenance was last confirmed |
| `ref` | Version identifier at time of indexing (commit SHA for git sources) |

This separation means:
- **File renames are free.** Same content hash, new provenance record.
- **File moves are free.** Same content hash, updated file path.
- **Reorgs are free.** Only actual content changes trigger embedding work.
- **Staleness is detectable.** `indexed_at` and `ref` tell you exactly how fresh each piece of knowledge is.

### Sources

Tracks what's configured for indexing. Each source is identified by its real URL.

| Column | Purpose |
|--------|---------|
| `url` | Source URL (e.g., `https://gitlab.example.com/grove/atlas`) — primary identity |
| `kind` | Source type, inferred from host or explicit (`gitlab`, `github`, `directory`) |
| `config` | Source-specific configuration (branch, paths, etc.) |
| `last_indexed_ref` | Last successfully indexed version identifier |
| `last_indexed_at` | Timestamp of last successful index |

MVP implements the `gitlab` kind only. Adding a new source type means adding a fetcher, not changing the schema. The URL is already globally unique — no invented URI scheme needed.

## Query Time

Not fully designed yet — this section describes the retrieval path that the KB agent will use to fulfill caller stories C1-C4.

1. Caller sends a question with working context
2. KB agent formulates a vector search query
3. pgvector returns relevant chunks with similarity scores
4. KB agent fetches provenance for each chunk (source, path, freshness)
5. KB agent may fetch surrounding chunks or full documents for additional context
6. KB agent synthesizes an answer with provenance citations

The "zoom out" capability — fetching neighbors or the full file — uses provenance (repo, file_path, position) to reconstruct broader context when a chunk alone isn't sufficient.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Single datastore | PostgreSQL | pgvector for embeddings, River for jobs, chunks and provenance in one place. One backup, one connection. |
| Content-addressable chunks | BLAKE3 hash as identity | Only embed what actually changed. Dedup across sources for free. |
| Embeddings decoupled from chunks | Separate table, composite key (hash, model) | Swap or add embedding models without touching chunks or provenance. |
| Real URLs as source identity | `https://gitlab.example.com/grove/atlas` | Globally unique, no invented URI scheme. Kind inferred from host. |
| Tarball over git clone | GitLab archive API | No git history, no `.git` directory, lighter memory footprint. |
| Structure-based chunking | Markdown headings/sections | Deterministic boundaries that don't shift when neighboring content changes. Heading hierarchy preserved for context. |
| River for all async work | Two job types (reindex, embed) | One queue system, one set of concurrency knobs. Retries and backpressure built in. |
| System hook over per-repo config | GitLab instance-wide webhook | Zero per-repo setup. KB decides what to index based on its own config. |

## Deferred

- **Code indexing** — tree-sitter based chunking for source code, after Markdown proves out
- **Correction flow** — routing fixes upstream to source repos (Principle #6, parked for MVP)
- **Access control** — zone-scoped results based on GitLab permissions
- **Conversational sessions** — multi-turn state management for C4 (KB agent design TBD)
- **Embedding model selection** — using OpenAI for now, may revisit
