---
status: accepted
date: 2026-02-01
---

# Vector Store Selection for Note Embeddings

## Context and Problem Statement

kb needs a vector store to persist and query 1536-dimensional OpenAI embeddings for ~1500+ Obsidian vault notes. The store must support cosine similarity search and fit the project's single-binary, no-external-deps deployment model.

Which embedded vector storage approach should we use?

## Decision Drivers

* Single-binary deployment (no external services)
* Reasonable query performance at 1500-5000 notes
* Minimal operational complexity
* CGo vs pure Go trade-offs
* Future scalability if vault grows significantly

## Considered Options

* sqlite-vec via ncruces/go-sqlite3 (WASM, no CGo)
* sqlite-vec via mattn/go-sqlite3 (SQLite extension via CGo)
* libsql/Turso Database (Rust rewrite with native vector support)
* Pure Go approximate nearest neighbor (e.g., hnswlib-go, custom)
* External vector DB (Qdrant, ChromaDB, Pinecone)

## Decision Outcome

Chosen option: **sqlite-vec via `ncruces/go-sqlite3` (WASM)**, because it satisfies the single-binary deployment requirement with zero CGo complexity while remaining swappable if performance or tooling concerns arise later.

### Consequences

* Good, because pure Go builds — no C toolchain, no cross-compilation complexity, no slow CGo rebuild times
* Good, because sqlite-vec provides proven, production-ready vector search with official WASM bindings
* Good, because SQLite embeds cleanly (one file per vault, familiar SQL patterns)
* Good, because the storage interface keeps the implementation swappable if WASM overhead or tooling issues emerge
* Neutral, because WASM has ~1.5-2x overhead vs CGo on some workloads — negligible at 1500 notes with brute-force scan
* Neutral, because `ncruces/go-sqlite3` is a smaller ecosystem than `mattn/go-sqlite3` (fewer users, less community support)
* Bad, because brute-force vector search won't scale to 100k+ notes (would need ANN index like libsql's DiskANN)

## Pros and Cons of the Options

### sqlite-vec via ncruces/go-sqlite3 (WASM) — CHOSEN

SQLite extension by Alex Garcia with WASM bindings for `ncruces/go-sqlite3`.

* Good, because pure Go builds — no CGo, no C toolchain required
* Good, because embeds directly in SQLite (one file per vault)
* Good, because familiar SQL query patterns
* Good, because sqlite-vec is maintained, purpose-built, and production-ready
* Good, because trivial cross-compilation (`GOOS=linux go build`)
* Neutral, because brute-force search (no ANN index) — fine at 1500-5000 notes
* Neutral, because WASM has measurable overhead vs CGo (~1.5-2x on insert-heavy workloads, competitive on reads)
* Neutral, because `ncruces/go-sqlite3` has smaller community than `mattn/go-sqlite3`

### sqlite-vec via mattn/go-sqlite3 (CGo)

SQLite extension via traditional CGo bindings.

* Good, because fastest raw performance (no WASM overhead)
* Good, because `mattn/go-sqlite3` is the de facto standard Go SQLite driver (10k+ stars)
* Good, because embeds directly in SQLite (one file per vault)
* Good, because sqlite-vec is production-ready and well-documented
* Bad, because requires CGo — slow builds, cross-compilation complexity, C toolchain dependency
* Bad, because macOS shows deprecation warnings on `sqlite3_auto_extension` (cosmetic but annoying)

### libsql/Turso Database (Rust rewrite)

Turso's Rust rewrite of SQLite with native DiskANN-based vector search.

* Good, because vector support is a first-class feature (DiskANN index, not brute-force)
* Good, because `turso-go` driver uses `purego` to avoid CGo while calling Rust library
* Good, because positions well for future scale (ANN index helps at 100k+ rows)
* Good, because Turso is actively developed and shipping features fast
* Bad, because Rust rewrite is not production-ready (early adopter risk)
* Bad, because Go bindings are very new (updated Jan 6, 2026 — one month old)
* Bad, because less battle-tested than 20+ years of SQLite C code
* Bad, because risk of API churn, immature tooling, or integration bugs during Phase 2

### Pure Go approximate nearest neighbor

Use a Go library for vector indexing, store raw vectors in SQLite BLOBs.

* Good, because no CGo dependency
* Good, because full control over indexing strategy
* Bad, because two storage systems to coordinate
* Bad, because more code to maintain
* Bad, because less mature than SQLite-based options at this scale

### External vector DB

Hosted or self-hosted vector database (Qdrant, ChromaDB, Pinecone).

* Good, because purpose-built for vector search at scale
* Good, because advanced features (filtering, hybrid search)
* Bad, because violates single-binary deployment model
* Bad, because operational complexity (another service to run)
* Bad, because overkill for 1500-5000 notes

## More Information

This decision unblocks Phase 2 (Storage) implementation.

Key trade-offs accepted:
- **CGo complexity avoided**: Pure Go builds with no C toolchain dependency. Cross-compilation is trivial (`GOOS=linux go build`).
- **WASM overhead accepted**: ~1.5-2x slower than CGo on insert-heavy workloads per [benchmarks](https://github.com/cvilsmeier/go-sqlite-bench), but negligible at 1500-5000 notes with brute-force vector scan.
- **Swappable later**: Storage interface (`internal/storage`) keeps implementation swappable. Can migrate to CGo (for perf) or libsql/Turso (for ANN index) if needs change.
- **No ANN index**: Brute-force vector search won't scale to 100k+ notes. If vault grows significantly, would need to migrate to libsql's DiskANN or external vector DB.

Implementation next steps:
1. Add `github.com/ncruces/go-sqlite3` and `github.com/asg017/sqlite-vec-go-bindings/ncruces` to dependencies
2. Implement `internal/storage` interface with SQLite backend
3. Define schema for notes table with vector column (1536-dim BLOB)
4. Test cosine similarity queries at target scale (1500-5000 notes)
