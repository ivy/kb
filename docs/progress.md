# KB Implementation Progress

Living status of what's built, what's stubbed, and what's next. Updated as work lands.

Last updated: 2026-02-16

## How KB Works (Quick Orientation)

KB is a knowledge service for a personal homestead. It indexes GitLab repos into searchable chunks, then serves synthesized answers to calling agents via a knowledge agent. Single Go binary, single PostgreSQL database (pgvector + River job queue).

```
GitLab repo → webhook/CLI trigger → River reindex job → fetch tarball → chunk markdown → store chunks + provenance → River embed jobs → OpenAI embeddings API → pgvector

Agent query → POST /v1/ask → MRKL agent (search/lookup/read/list tools) → synthesized Markdown answer with provenance footnotes
```

Key directories:

| Path | Purpose |
|------|---------|
| `cmd/kb/` | CLI entrypoint (cobra) |
| `internal/agent/` | Knowledge agent (MRKL-style, query synthesis) |
| `internal/chunk/` | Markdown chunker (heading hierarchy, token limits) |
| `internal/cli/` | Command definitions (serve, setup, sources, reindex, status) |
| `internal/config/` | Viper-based config loading (YAML + `KB_*` env vars) |
| `internal/embed/` | Embedding handler (OpenAI API) |
| `internal/index/` | Reindex handler (fetch → chunk → persist pipeline) |
| `internal/queue/` | River-based async job queue |
| `internal/server/` | HTTP server (configurable listen addr, Tailscale-aware auth stub) |
| `internal/session/` | LRU session manager |
| `internal/source/` | Source adapters (GitLab archive fetcher) |
| `internal/store/` | PostgreSQL + pgvector persistence |
| `docs/architecture/` | Per-subsystem design docs (agent, auth, indexing, query, admin) |

## Subsystem Status

### Indexing Pipeline — Done

The full path from "GitLab repo URL" to "vectors in pgvector" works end-to-end.

| Component | Package | Status |
|-----------|---------|--------|
| GitLab archive fetcher | `internal/source/` | Done — downloads tarball via GitLab API, unpacks, walks markdown files |
| Markdown chunker | `internal/chunk/` | Done — heading-hierarchy-aware, content-addressable (BLAKE3 hash) |
| Reindex handler | `internal/index/` | Done — orchestrates fetch → chunk → diff → persist → enqueue embeds |
| Embed handler | `internal/embed/` | Done — calls OpenAI `/v1/embeddings`, upserts vector into pgvector |
| River job queue | `internal/queue/` | Done — GenericJobArgs bridge pattern, two named queues (reindex, embed) |
| Database migrations | `internal/store/migrations/` | Done — 001 (chunks/provenance/sources), 002 (embeddings + pgvector) |
| River migrations | `internal/store/pgstore.go` | Done — runs via `rivermigrate` inside `PGStore.Migrate()` |

**How the queue bridge works:** River uses typed `JobArgs` + `Worker[T]` generics. KB bridges this with a single `GenericJobArgs` type carrying `job_kind` + `payload` bytes. A `genericWorker` dispatches to the correct `HandlerFunc` by kind. `InsertOpts()` routes jobs to named queues matching their kind, giving each job type independent concurrency control.

**Worker defaults:** reindex=2, embed=4 (configurable via `config.Workers`).

### Store (PostgreSQL + pgvector) — Mostly Done

| Method | Status | Notes |
|--------|--------|-------|
| `UpsertChunk` | Done | Content-addressable by BLAKE3 hash |
| `DeleteOrphanedChunks` | Done | Removes chunks with no provenance references |
| `UpsertProvenance` | Done | Links chunks to source files |
| `DeleteProvenance` | Done | |
| `ListProvenance` | Done | By source URL |
| `AddSource` | Done | |
| `ListSources` | Done | |
| `RemoveSource` | Done | |
| `UpdateSourceIndexed` | Done | Sets last_indexed_ref and last_indexed_at |
| `GetChunkContent` | Done | Lookup by content hash |
| `UpsertEmbedding` | Done | pgvector upsert with `(content_hash, model)` composite key |
| `SearchSimilar` | Done | Cosine distance (`<=>`), joins chunks + provenance |
| `DeleteEmbeddings` | Done | By content hash |
| `LookupByKeyword` | **Stub** | Returns "not implemented" — needed for agent `lookup` tool |
| `GetDocument` | **Stub** | Returns "not implemented" — needed for agent `read` tool |

### Admin CLI — Mostly Done

| Command | Status | Notes |
|---------|--------|-------|
| `kb serve` | Done | Configurable listen address, defaults to `127.0.0.1:8080` |
| `kb setup` | Done | Checks PostgreSQL, pgvector, runs migrations |
| `kb sources add <url>` | Done | Registers source, infers kind from host |
| `kb sources list` | Done | Shows sources with last-indexed status |
| `kb sources remove <url>` | Done | Removes source + provenance + orphaned chunks |
| `kb reindex <url>` | Done | Enqueues reindex job for one source |
| `kb reindex` (all sources) | **Not done** | Currently requires URL arg (`cobra.ExactArgs(1)`) |
| `kb status` | Done | Shows source count + queue stats |

### HTTP Server — Routes Exist, Handlers Stubbed

| Endpoint | Status | Notes |
|----------|--------|-------|
| `POST /v1/ask` | **Stub** | Returns "ask endpoint not implemented" |
| `POST /v1/sessions/:id/ask` | **Stub** | Returns "session ask not implemented" |
| `POST /webhook` | **Stub** | Returns "webhook not implemented" |
| `GET /llms.txt` | Done | Static API discovery text |
| Auth middleware | **Stub** | Three-mode structure exists, WhoIs/capability check is TODO |
| Listen address | Done | Configurable via `config.Server.Listen`, `Addr()` for test discovery |

### Knowledge Agent — Not Started

The core value proposition. Architecture: MRKL-style agent with four tools.

| Component | Status | Blocking on |
|-----------|--------|-------------|
| `Ask()` method | **Stub** — returns "not implemented" | Everything below |
| MRKL agent loop | Not started | LLM client choice |
| `search` tool (vector similarity) | Not started | `SearchSimilar` exists in store |
| `lookup` tool (keyword/identifier) | Not started | `LookupByKeyword` stub in store |
| `read` tool (progressive doc fetch) | Not started | `GetDocument` stub in store |
| `list` tool (browse indexed content) | Not started | `ListSources` exists, file listing within source doesn't |
| System prompt | Not started | — |
| Provenance-annotated responses | Not started | — |

See `docs/architecture/agent.md` for the full agent design: tools, retrieval strategy, system prompt, response format.

### Sessions — Stub

| Component | Status |
|-----------|--------|
| `Manager` interface | Done — Create, Get, Append |
| `LRUManager` struct | **Stub** — all methods return "not implemented" |
| JSONL filesystem backing | Not started |
| LRU eviction | Not started |

See `docs/architecture/query.md` for session flow: `/v1/ask` creates session, `/v1/sessions/:id/ask` continues.

### Auth (Tailscale) — Stub, Deferred

| Component | Status |
|-----------|--------|
| tsnet listener | Returns error "not implemented" |
| WhoIs + capability check | TODO in `authMiddleware` |
| Three-mode switching (no-ts, ts-no-authz, ts-authz) | Mode selection logic exists |

Explicitly deferred. Not blocking MVP usefulness. See `docs/architecture/auth.md`.

### Webhook — Stub, Deferred

`handleWebhook` needs to parse GitLab system hook payload, check against configured sources, and enqueue reindex. The `kb reindex` CLI is the manual fallback.

## Config Reference

Viper loads from YAML file (`--config` flag) with `KB_*` env var overrides.

```yaml
database:
  url: postgres://kb:kb@localhost:5432/kb?sslmode=disable
server:
  listen: "127.0.0.1:8080"    # default; use 0.0.0.0:8080 in containers
tailscale:
  enabled: false               # tsnet disabled = plain TCP, no auth
  hostname: kb
  authz: false
  ask_cap: "homestead.local/cap/kb/ask"
  admin_cap: "homestead.local/cap/kb/admin"
gitlab:
  token: ""                    # GitLab PAT with read_api scope
embedding:
  provider: ""
  model: ""
  api_key: ""                  # OpenAI API key for embeddings
agent:
  default_model: ""
workers:
  reindex: 2                   # River queue concurrency
  embed: 4
sessions:
  dir: /tmp/kb-sessions
  max: 10
log:
  level: debug
  format: text
```

## Test Structure

Tests use **Ginkgo** + **Gomega**. Three tiers:

| Tier | Command | Label filter | What it tests |
|------|---------|--------------|---------------|
| Unit | `make test-unit` | `!db && !integration` | Pure logic, fakes, httptest |
| DB | `make test-db` | `db` | PostgreSQL via testcontainers |
| Integration | `make test-integration` | `integration` | Full pipeline with testcontainers |

Testcontainers require podman socket: `DOCKER_HOST=unix:///run/user/1000/podman/podman.sock` and `TESTCONTAINERS_RYUK_DISABLED=true`.

## What's Next

The critical path to "one real agent interaction noticeably better":

1. **Agent implementation** — MRKL loop, four tools, system prompt, LLM client
2. **Store methods** — `LookupByKeyword` and `GetDocument` (agent tools need these)
3. **`/v1/ask` handler** — wire agent to HTTP, text-in / markdown-out
4. **Sessions** — LRU + JSONL, wire into `/v1/ask` and `/v1/sessions/:id/ask`

After that: webhook handler, `kb reindex` without args, e2e integration tests for the full serve pipeline, auth/tsnet.
