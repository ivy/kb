# Phase 1: Walking Skeleton 🦴

## Goal

End-to-end pipeline with stubs. See it work (with placeholders).

## Philosophy

**Walking skeleton first.** Get the full end-to-end pipeline wired up with stubs immediately. See logs flowing, get placeholder responses. *Then* fill in the real implementation.

This approach:
- Validates integration points early
- Keeps motivation high (it "works" from the start)
- Makes debugging easier (you know the plumbing works)

## Epics

| Epic | Component | Purpose |
|------|-----------|---------|
| 001 | Project Scaffold | Directory structure, go.mod, compiles |
| 002 | Stub Implementation | kb-mcp + kb-server stubs, wired end-to-end |

## Architecture

```
┌─────────────┐      stdio       ┌─────────────┐     Unix socket    ┌─────────────┐
│   Agent     │◄───────────────►│   kb-mcp    │◄──────────────────►│  kb-server  │
│(Claude Code)│   JSON-RPC 2.0  │             │       gRPC         │   (daemon)  │
└─────────────┘                 └─────────────┘                    └─────────────┘
```

### kb-mcp
- MCP server using stdio transport (official Go SDK)
- Exposes tools: `list_knowledge_bases`, `ask_knowledge_base`
- Connects to kb-server via gRPC over Unix socket

### kb-server
- Long-running daemon
- gRPC server on `${XDG_RUNTIME_DIR:-/tmp}/kb.sock`
- Protocol Buffers for message serialization

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` |
| IPC | gRPC over Unix socket |
| Serialization | Protocol Buffers |
| MCP Transport | stdio |

## Success Milestone

When Phase 1 is complete:

```
# kb-server logs:
15:04:05 server.go:28: listening on /tmp/kb.sock
15:04:07 handler.go:32: ask_knowledge_base: question="Who handles security?" effort=medium knowledge_bases=[]

# Claude Code:
> Who handles security?
[Calls ask_knowledge_base]
[STUB] Would search for: Who handles security? (effort=medium)
```

🎉 End-to-end skeleton working!

## Project Context

This is Phase 1 of the **kb** (Knowledge Base) project - a semantic search service for Obsidian vaults.

Full project spec: [[Journal/2026-02 - Weekend Project - AI Information Tools]]

## File Structure Target

```
kb/
├── buf.gen.yaml
├── buf.yaml
├── cmd/
│   ├── kb-mcp/main.go
│   └── kb-server/main.go
├── gen/
│   └── kb/v1/
│       ├── kb.pb.go
│       └── kb_grpc.pb.go
├── internal/
│   ├── mcp/
│   │   ├── doc.go
│   │   ├── server.go
│   │   └── tools.go
│   ├── server/
│   │   ├── doc.go
│   │   └── server.go
│   ├── db/
│   ├── embed/
│   ├── search/
│   ├── claude/
│   └── index/
├── proto/
│   └── kb/v1/kb.proto
├── go.mod
└── go.sum
```
