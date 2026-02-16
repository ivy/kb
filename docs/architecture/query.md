# Query Architecture

How callers interact with KB. Read [indexing.md](indexing.md) for how knowledge gets in, this doc for how it comes back out.

## Interface

Plain HTTP. Text in, Markdown out. No JSON for the query path (Principle #1 — Built for Agents).

```
POST /v1/ask              → new session, returns answer + session URL
POST /v1/sessions/:id/ask  → continue session, returns answer
GET  /llms.txt             → API discovery for agents
```

### /llms.txt

Served at the root. Plain text description of KB's API — what endpoints exist, what they expect, what they return. An agent reads this and knows how to talk to KB. No OpenAPI spec, no schema to parse.

### Request Format

The query body is free-form text. The caller writes naturally — question, context, whatever is relevant. Guidance on effective queries lives in `/llms.txt`.

```
POST /v1/ask

What's the deployment pattern for atlas?

I'm updating the CI pipeline for atlas and need to understand
the current deployment approach — canary, blue-green, or rolling.
```

### Response Format

Markdown with footnotes. Provenance as navigable URLs.

```
Atlas uses canary deployments via Nomad job specs. The canary
count is set to 1, with a manual promotion step after health
checks pass[^1]. The CI pipeline triggers deployment through
a Nomad job dispatch[^2].

If you have follow-up questions, continue at:
POST /v1/sessions/a1b2c3

[^1]: grove/atlas — docs/deploy.md (indexed 2026-02-15)
  https://gitlab.example.com/grove/atlas/raw/abc123/docs/deploy.md
[^2]: grove/atlas — .gitlab-ci.yml (indexed 2026-02-15)
  https://gitlab.example.com/grove/atlas/raw/abc123/.gitlab-ci.yml
```

### Model Selection

The caller can optionally specify a model via header:

```
POST /v1/ask
X-Model: claude-sonnet-4-5-20250929

What's the deployment pattern for atlas?
```

Default is a sensible auto choice (e.g., GPT-4.1 or Claude Sonnet). The KB agent's task — retrieval, provenance evaluation, synthesis — doesn't require frontier reasoning. Fast and smart enough beats slow and brilliant.

## KB Agent

Implemented as a MRKL agent executor via LangChainGo. The agent receives the caller's prompt and iterates with its tools until it arrives at a synthesized answer.

### Tools

Four tools. Each has a distinct retrieval pattern — no overlap, no ambiguity about when to use which.

#### `search`

Semantic search across all indexed knowledge.

- **Input**: natural language query
- **Output**: ranked results with provenance metadata only (source URL, file path, heading hierarchy, indexed_at, ref) — no chunk content
- **Use**: primary retrieval path for open-ended questions

Returns lightweight results so the agent can evaluate relevance from metadata before deciding what to read into context.

#### `lookup`

Exact keyword/identifier search across all indexed knowledge.

- **Input**: keyword or identifier string
- **Output**: matching results with provenance metadata only — same shape as `search`
- **Use**: precise retrieval for incident IDs, service names, config values, specific terms that semantic search handles poorly

Complements `search` the way ctrl+F complements "find me something about..." Vector similarity misses exact identifiers; `lookup` catches them.

#### `read`

Fetch content from a specific document.

- **Input**: source URL + file path, optional section/position range
- **Output**: document content (scoped to the requested range)
- **Use**: zoom in on a specific result, or zoom out to get surrounding context

Without a section/position specified, returns the document's heading structure (table of contents) so the agent can orient and drill down. With a range, returns that content. The agent controls what enters its context — it never loads a 500-line doc when it needs two paragraphs.

Progressive disclosure: `read` with no range → TOC. `read` with a range → content.

#### `list`

Browse what's indexed.

- **Input**: optional source URL
- **Output**: without a URI, returns all indexed sources. With a URI, returns files within that source.
- **Use**: structural awareness — understanding coverage, gaps, and what knowledge exists before searching

Helps the agent distinguish "I searched and found nothing relevant" from "there's nothing indexed on this topic." Enables answers about scope ("KB has 3 repos related to atlas, covering deployment, CI, and architecture").

### Agent Loop

The agent decides its own retrieval strategy. A typical flow:

```
search "deployment pattern"
  → 8 results with provenance metadata, no content

read grove/atlas docs/deploy.md
  → TOC: 4 sections (Overview, Canary Config, Promotion, Rollback)

read grove/atlas docs/deploy.md sections 1-2
  → actual content for the most relevant sections

read grove/policy docs/supply-chain.md section 3
  → supporting context from another source

→ synthesize answer with footnoted provenance
```

The agent is responsible for:
- **Retrieval strategy**: choosing which tools to use and in what order
- **Context management**: reading only what it needs, not everything available
- **Provenance judgment**: understanding that a chunk indexed yesterday is more trustworthy than one indexed six months ago
- **Conflict detection**: flagging when sources disagree
- **Honesty**: saying "I don't know" when retrieval turns up nothing relevant

## Sessions

Server-side session state enables multi-turn conversations (C4).

### Storage

- LRU cache with configurable max size
- Backed by filesystem store (JSONL)
- Implemented behind a Go interface for future replacement

### Flow

1. `POST /v1/ask` creates a new session, returns the answer and a session URL
2. `POST /v1/sessions/:id/ask` continues the conversation with prior turns as context
3. Sessions expire from the LRU when capacity is reached — the agent handles missing sessions gracefully ("I don't have context from a previous conversation, but here's what I can tell you...")

Session history is prepended to the agent's context on each turn, giving it the full conversation to reason over. The caller doesn't manage state — it just hits the session URL.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Text in / Markdown out | Plain HTTP, no JSON for queries | Principle #1. Agents produce and consume text naturally. |
| Footnote-style provenance | Markdown footnotes with navigable URLs | Agents read footnotes naturally. URLs point to the exact indexed commit. |
| MRKL agent via LangChainGo | Iterative tool-use loop | Agent can search, read, evaluate, and synthesize in multiple steps. |
| Four tools | search, lookup, read, list | Semantic + exact search, selective reading, structural browsing. Each has a distinct job. No overlap. |
| Provenance-only search results | search/lookup return metadata, not content | Agent controls what fills its context. Evaluate relevance first, read content second. |
| Progressive read | TOC by default, content by range | Large docs never blow up agent context. Agent drills down to what it needs. |
| Server-side sessions | LRU + JSONL filesystem store | Stateless callers, simple follow-ups. Go interface allows swapping storage later. |
| No streaming | Synchronous request/response | Agents consume complete messages. Streaming benefits humans, not callers. |
| Default model with override | Header-based model selection | Sane default for most queries. Callers can opt into stronger/cheaper models when needed. |
