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

Markdown with Wikipedia-style footnotes. Each footnote carries a **relevant quote** from the source so callers can evaluate claims without following links, catch hallucination, and see what was true at index time.

```
Atlas uses canary deployments via Nomad job specs. The canary
count is set to 1, with a manual promotion step after health
checks pass[^1]. The CI pipeline triggers deployment through
a Nomad job dispatch[^2].

If you have follow-up questions, continue at:
POST /v1/sessions/a1b2c3

[^1]: "The canary count is set to 1 in the Nomad job spec, with a manual
  promotion step triggered after health checks pass for 5 minutes."
  §Canary Config
  https://gitlab.example.com/grove/atlas/raw/abc123/docs/deploy.md
[^2]: "deploy: stage triggers nomad job dispatch with canary strategy"
  §deploy
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

See [agent.md](agent.md) for the agent's tools, retrieval strategy, system prompt, and iteration notes.

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
| Wikipedia-style footnotes | Quote + source path + URL | Callers evaluate claims inline without following links. Quotes catch hallucination and preserve what was true at index time. |
| `/ask` over `/query` | Conversational framing | Primes callers to write natural questions, not keyword searches. |
| Server-side sessions | LRU + JSONL filesystem store | Stateless callers, simple follow-ups. Go interface allows swapping storage later. |
| No streaming | Synchronous request/response | Agents consume complete messages. Streaming benefits humans, not callers. |
| Default model with override | Header-based model selection | Sane default for most queries. Callers can opt into stronger/cheaper models when needed. |
