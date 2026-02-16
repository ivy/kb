# Admin Architecture

How KB is configured and managed. Read [indexing.md](indexing.md) for the pipeline, [query.md](query.md) for the caller interface, this doc for administration.

## Interface

CLI subcommands on the same Go binary that runs the server. Agents run shell commands natively — no HTTP client, no JSON, no auth tokens. The CLI connects directly to PostgreSQL.

```
kb serve          → start the HTTP server (caller-facing)
kb setup          → check prerequisites, configure webhooks
kb sources add    → register a source for indexing
kb sources list   → show configured sources and indexing status
kb sources remove → remove a source and its indexed content
kb reindex        → trigger reindexing of a source
kb status         → show overall system health
```

Deployed as a scratch container image with the Go binary as ENTRYPOINT. Agents pass `--help` for details.

## Commands

### `kb setup`

Checks prerequisites and configures integrations. Designed to be run by an agent or human after initial deployment.

```
$ kb setup
✓ PostgreSQL connection
✓ pgvector extension
✓ River migrations
✗ GitLab system hook not configured
  → Configure for https://gitlab.example.com? (needs admin API token)
```

The server runs without webhooks — `kb reindex` works as a manual trigger. Webhooks are an automation, not a hard dependency.

### `kb sources add <url>`

Registers a source for indexing. KB infers the source kind from the host.

```
$ kb sources add https://gitlab.example.com/grove/atlas
Added gitlab source: https://gitlab.example.com/grove/atlas
  branch: main (default)
  To index now: kb reindex https://gitlab.example.com/grove/atlas
```

Optional flags for source-specific config (branch, paths) as needed.

### `kb sources list`

Shows all configured sources with indexing status.

```
$ kb sources list
SOURCE                                          KIND     LAST INDEXED         REF
https://gitlab.example.com/grove/atlas          gitlab   2026-02-16 09:32     abc1234
https://gitlab.example.com/grove/policy         gitlab   2026-02-15 14:01     def5678
https://gitlab.example.com/hearth/obsidian      gitlab   never                —
```

### `kb sources remove <url>`

Removes a source and cleans up its indexed content (provenance, orphaned chunks).

```
$ kb sources remove https://gitlab.example.com/grove/atlas
Removed source: https://gitlab.example.com/grove/atlas
  Cleaned up 142 provenance records
  Removed 89 orphaned chunks
```

### `kb reindex <url>`

Triggers a reindex job for a source. Same pipeline as webhook-triggered reindexing.

```
$ kb reindex https://gitlab.example.com/grove/atlas
Enqueued reindex job for https://gitlab.example.com/grove/atlas
  Job ID: 47
```

Without a URL, reindexes all configured sources.

```
$ kb reindex
Enqueued reindex jobs for 3 sources
  Job IDs: 47, 48, 49
```

### `kb status`

System health at a glance.

```
$ kb status
Server:     running (pid 1234)
Database:   connected (42 chunks, 3 sources)
Webhook:    configured (https://gitlab.example.com)
Jobs:       2 pending, 0 running, 0 failed
```

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| CLI over HTTP admin API | Subcommands on the server binary | Principle #1. Agents have Bash tools. Shell commands are the most natural admin interface. |
| Direct database access | CLI connects to PostgreSQL, not through the server | No admin HTTP endpoints to build or secure for MVP. One fewer attack surface. |
| Webhook as optional automation | Server works without webhooks, `kb reindex` is the manual fallback | Graceful degradation. Setup can happen incrementally. |
| `kb setup` for prerequisites | Interactive check-and-configure flow | An agent runs it, reads the output, acts on it. Self-documenting. |
| Real URLs as source identity | `kb sources add https://...` | Agents paste what they see. No invented URI scheme to learn. |
