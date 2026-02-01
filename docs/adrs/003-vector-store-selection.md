---
status: proposed
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

* sqlite-vec (SQLite extension via CGo)
* libsql (Turso's SQLite fork with native vector support)
* Pure Go approximate nearest neighbor (e.g., hnswlib-go, custom)
* External vector DB (Qdrant, ChromaDB, Pinecone)

## Decision Outcome

TBD — evaluating options before committing.

## Pros and Cons of the Options

### sqlite-vec

SQLite extension by Alex Garcia providing vector search via virtual tables.

* Good, because embeds directly in SQLite (one file per vault)
* Good, because familiar SQL query patterns
* Good, because maintained and purpose-built
* Neutral, because brute-force search (no ANN index) — fine at this scale
* Bad, because requires CGo
* Bad, because Go bindings (`sqlite-vec-go-bindings`) are relatively young

### libsql

Turso's fork of SQLite with native vector search support.

* Good, because vector support is a first-class feature
* Good, because offers a Go SDK
* Good, because maintains SQLite compatibility
* Neutral, because additional capabilities (embedded replicas) we may not need
* Bad, because forked from SQLite — less battle-tested than upstream
* Bad, because unclear CGo situation for embedded use

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

This decision blocks Phase 2 (Storage) implementation. Will evaluate with hands-on prototyping before committing.

Key questions to resolve:
- How do sqlite-vec Go bindings work in practice? Build complexity?
- Does libsql's embedded mode work cleanly from Go?
- What's the actual query latency at 1500 notes for each option?
