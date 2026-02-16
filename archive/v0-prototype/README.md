# kb

**Your second brain as a service.**

> [!WARNING]
> This project is a work in progress. APIs, architecture, and features may change significantly.

kb is a semantic search service for Obsidian vaults. It lets any AI agent query your personal knowledge base using natural language - no more fumbling with file paths or hoping the agent finds the right context.

## The Problem

You have a 1500+ note Obsidian vault full of meeting notes, project docs, and accumulated knowledge. When you ask an AI agent to help with something, it struggles to find relevant context:

- Wrong notes surface, or nothing surfaces at all
- You end up manually hunting for files and pasting them in
- Fresh conversations require hand-holding with file paths
- Agents waste tokens fumbling around your vault

**You become the bottleneck** - manually retrieving context instead of synthesizing and creating.

## The Solution

kb provides semantic search over your vault via MCP (Model Context Protocol). Any agent, anywhere, can tap into your knowledge:

```
> "I need to draft an epic for sandboxed execution. Pull relevant
>  context from my notes about security conversations and prior art."

[Agent calls ask_knowledge_base]

Based on your notes, here's what I found...
- Taylor discussed container isolation in your recent meeting
- The ExamplePlatform architecture doc covers sandboxing approaches
- You explored gVisor vs Firecracker in December

[Synthesized answer with citations]
```

## How It Works

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Agent     │────▶│   kb-mcp    │────▶│  kb-server  │
│ (any tool)  │ MCP │   (stdio)   │socket│  (daemon)   │
└─────────────┘     └─────────────┘     └─────────────┘
                                              │
                                              ▼
                                        ┌─────────────┐
                                        │   SQLite    │
                                        │ + sqlite-vec│
                                        └─────────────┘
```

1. **kb-server** runs as a daemon, indexing your vault(s) and keeping embeddings up to date
2. **kb-mcp** is an MCP server that any agent can connect to
3. Agents call `list_knowledge_bases` to discover available sources, then `ask_knowledge_base` with a natural language question
4. kb searches semantically, retrieves relevant notes, and returns a synthesized answer with citations

## Features

- **Semantic search** - Query by meaning, not keywords or filenames
- **Multi-vault support** - Index multiple Obsidian vaults
- **Live indexing** - Notes re-embed automatically as you edit
- **Effort levels** - Quick lookups or deep research, your choice
- **Works anywhere** - Any MCP-compatible agent can use it

## Components

| Component   | Description                                                  |
| ----------- | ------------------------------------------------------------ |
| `kb-server` | Background daemon that handles queries and manages indexing  |
| `kb-mcp`    | MCP server (stdio transport) exposing knowledge base tools   |

## Tech Stack

- **Go** - Single binary, fast startup
- **SQLite + sqlite-vec** - Embedded vector store, no external dependencies
- **OpenAI Embeddings** - `text-embedding-3-small` for semantic understanding
- **Unix sockets** - Clean IPC, works in devcontainers
