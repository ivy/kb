# AGENTS.md

This file provides guidance to coding agents when working with code in this repository.

## Project

kb is a semantic search service for Obsidian vaults, exposing a personal knowledge base to AI agents via MCP.

## Architecture

```
Agent ←stdio/JSON-RPC→ kb-mcp ←HTTP-over-Unix-socket/Connect→ kb-server
```

- **kb-server** (`cmd/kb-server`): daemon listening on `${XDG_RUNTIME_DIR}/kb.sock` (fallback `/tmp/kb.sock`), implements `KnowledgeBaseService` via Connect RPC
- **kb-mcp** (`cmd/kb-mcp`): MCP server bridging stdio JSON-RPC to Connect RPC calls over Unix socket
- **proto/kb/v1/kb.proto**: service definition with `List` and `Query` RPCs
- **gen/**: committed generated protobuf/Connect code (allows `go build` without buf)

## Build & Development Commands

```sh
go build ./...                 # build all
go test ./...                  # run all tests
go test ./internal/server/     # run tests for one package
go generate ./...              # regenerate protobuf code (requires buf)
buf lint                       # lint proto definitions
```

Tool versions managed by **mise** (`mise.toml`): Go 1.25.1, buf, protoc-gen-go, protoc-gen-connect-go.

## Key Design Decisions

- **Connect RPC over Unix sockets** (ADR-001): same server speaks Connect, gRPC, and gRPC-Web; JSON-over-HTTP/1.1 for debuggability
- **Unix socket directory security** (ADR-002): mandatory ownership check, mode `0700`, no symlinks; server refuses to start if invariants fail

## Proto Code Generation

Generated code is committed. After changing `proto/kb/v1/kb.proto`:

```sh
go generate ./...
```

Verify nothing drifted: `go generate ./... && git diff --exit-code`

## Debugging

Manual testing with curl:

```sh
curl --unix-socket /tmp/kb.sock http://localhost/kb.v1.KnowledgeBaseService/List \
  -H 'Content-Type: application/json' -d '{}'
```

Or with buf curl for gRPC testing. See `docs/how-to/debug-pipeline.md`.

## Conventions

- ADRs live in `docs/adrs/` and follow the template in `000-adr-template.md`
- Epic specs live in `docs/epics/`
- Internal packages each have a `doc.go` describing purpose and planned scope
- MCP tool input types defined in `internal/mcp/tools.go` with JSON schema struct tags
