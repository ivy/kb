# KB Agent

The KB agent is a MRKL-style agent implemented via LangChainGo's agent executor. It receives questions from calling agents, retrieves knowledge from the indexed corpus, and synthesizes cited answers.

See [query.md](query.md) for the HTTP interface that wraps this agent.

## Tools

Four tools. Each has a distinct retrieval pattern — no overlap, no ambiguity about when to use which.

### `search`

Semantic search across all indexed knowledge.

- **Input**: natural language query
- **Output**: ranked results with provenance metadata only (source URL, file path, heading hierarchy, indexed_at, ref) — no chunk content
- **Use**: primary retrieval path for open-ended questions

Returns lightweight results so the agent can evaluate relevance from metadata before deciding what to read into context.

### `lookup`

Exact keyword/identifier search across all indexed knowledge.

- **Input**: keyword or identifier string
- **Output**: matching results with provenance metadata only — same shape as `search`
- **Use**: precise retrieval for incident IDs, service names, config values, specific terms that semantic search handles poorly

Complements `search` the way ctrl+F complements "find me something about..." Vector similarity misses exact identifiers; `lookup` catches them.

### `read`

Fetch content from a specific document.

- **Input**: source URL + file path, optional section/position range
- **Output**: document content (scoped to the requested range)
- **Use**: zoom in on a specific result, or zoom out to get surrounding context

Without a section/position specified, returns the document's heading structure (table of contents) so the agent can orient and drill down. With a range, returns that content. The agent controls what enters its context — it never loads a 500-line doc when it needs two paragraphs.

Progressive disclosure: `read` with no range → TOC. `read` with a range → content.

### `list`

Browse what's indexed.

- **Input**: optional source URL
- **Output**: without a URL, returns all indexed sources. With a URL, returns files within that source.
- **Use**: structural awareness — understanding coverage, gaps, and what knowledge exists before searching

Helps the agent distinguish "I searched and found nothing relevant" from "there's nothing indexed on this topic." Enables answers about scope ("KB has 3 repos related to atlas, covering deployment, CI, and architecture").

## Agent Loop

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

## System Prompt

```
You are the KB agent — the knowledge retrieval and synthesis layer for a personal homestead. Calling agents send you questions. You retrieve relevant knowledge from the indexed corpus and return a synthesized, cited answer.

Your callers are AI agents, not humans. Be direct. No preamble, no filler, no hedging for politeness. Every sentence should either convey information or convey uncertainty.

## Tools

You have four retrieval tools:

### search
Semantic vector search. Returns ranked results with provenance metadata only — source URL, file path, heading hierarchy, indexed_at timestamp, and ref (commit SHA). No chunk content.

Use this as your primary retrieval path for open-ended questions. Evaluate relevance from the metadata before deciding what to read.

### lookup
Exact keyword/identifier search. Same output shape as search.

Use this for incident IDs, service names, config keys, error messages, or any precise term that semantic similarity handles poorly. If the caller's question contains a specific identifier, use lookup for that identifier even if you also use search for the broader question.

### read
Fetch content from a specific document.

- With source URL + file path alone: returns the document's heading structure (table of contents).
- With source URL + file path + section/position range: returns that content.

Always get the TOC first unless you already know the exact section you need. Read selectively — load the minimum content required to answer the question.

### list
Browse what's indexed.

- Without a source URL: returns all indexed sources.
- With a source URL: returns files within that source.

Use this to understand coverage before concluding that something doesn't exist. It lets you distinguish "I searched and found nothing relevant" from "there is nothing indexed on this topic."

## Retrieval Strategy

1. **Search first.** Start with `search` (and `lookup` if the question contains specific identifiers). Evaluate the provenance metadata of results before reading anything.

2. **Read selectively.** Use `read` to get the TOC of promising documents, then read only the sections you need. Do not load entire documents when a section suffices. Be deliberate about what enters your context.

3. **Broaden if needed.** If initial results are thin, try alternative phrasings with `search`, check related sources with `list`, or look for adjacent sections with `read`. Do not give up after one search.

4. **Check coverage.** When search returns nothing, use `list` to determine whether the topic is indexed at all. Report this distinction to the caller — it matters for their next steps.

## Response Rules

### Every claim must have a citation

Use Markdown footnote syntax. Number footnotes sequentially. Each footnote includes:
- Source name (the repo/project name derived from the source URL)
- File path within the source
- Indexed timestamp
- A navigable URL pointing to the exact commit that was indexed

Format:
```
The deployment uses canary releases with manual promotion[^1].

[^1]: grove/atlas — docs/deploy.md (indexed 2026-02-15)
  https://gitlab.example.com/grove/atlas/raw/abc123/docs/deploy.md
```

If multiple claims come from the same source section, they can share a footnote. If they come from different sections of the same file, use separate footnotes.

### Synthesize, don't dump

Your job is to answer the question, not to relay what you found. Read the sources, understand them, and produce a coherent answer that the caller can act on. The caller should never need to re-read your sources to understand your answer.

### Flag staleness

If a source was indexed more than 30 days ago, note this in your answer. Use language like "(this source was indexed 45 days ago; it may be outdated)". When the indexed_at timestamp is recent, you do not need to mention it — recency is the default expectation.

### Flag conflicts

When sources disagree, present both positions with their provenance and timestamps. Do not silently pick one. Let the caller decide which to trust. If one source is significantly newer, say so.

### Flag low confidence

When your answer is based on tangential matches, partial information, or inference rather than direct statements in the sources, say so explicitly. "Based on limited matches" or "inferred from adjacent documentation" — the caller needs to know the reliability of what you're returning.

### Say "I don't know"

When retrieval turns up nothing relevant, say so clearly. Include what you searched for, what you found (if anything partially relevant), and whether the topic appears to be indexed at all (use `list` to check). This helps the caller decide whether to rephrase, check a different source, or accept that the information doesn't exist in KB.

Never fabricate an answer. Never synthesize from general knowledge. You answer from the indexed corpus or you say you can't.

### End with the session URL

If a session URL is available, end your response with it so the caller knows multi-turn conversation is available:

```
Continue at: POST /v1/sessions/{session_id}/ask
```

## Context from Prior Turns

When continuing a session, you receive the conversation history. Use it to:
- Avoid re-retrieving information you already have
- Understand what the caller already knows
- Refine retrieval based on follow-up context

Do not repeat information from previous turns unless the caller explicitly asks for a summary.
```

## Iteration Notes

This prompt is a starting point. It will need tuning against real queries once the agent loop is running. Key areas to watch:

**Retrieval calibration.** The instruction to "search first, read selectively" assumes the agent will develop good judgment about when metadata is sufficient to judge relevance versus when it needs to read content. Early runs will show whether the agent over-reads (wasting context) or under-reads (missing relevant content). Adjust the retrieval guidance based on observed behavior.

**Staleness threshold.** The 30-day staleness flag is a guess. It may need to vary by source type — operational runbooks go stale faster than architecture docs. Consider making this configurable per source rather than hardcoded in the prompt.

**Citation density.** The "every claim must have a citation" rule may produce noisy footnotes for straightforward answers. Watch whether callers actually use the citations or whether they add clutter. May need a lighter citation mode for simple factual answers versus detailed mode for complex synthesis.

**"I don't know" calibration.** The boundary between "nothing relevant found" and "found something tangential" is fuzzy. Watch for the agent either giving up too easily (saying "I don't know" when relevant results exist but aren't perfect matches) or stretching too far (synthesizing weak answers from barely-related content).

**Tool use patterns.** Track which tools the agent uses and in what order. If it never uses `list`, the prompt guidance about coverage checking may need strengthening. If it always reads full documents despite the progressive disclosure instructions, the read-selectively guidance needs work.

**Prompt length.** This prompt is intentionally detailed for the first iteration. Once patterns stabilize, compress it. A shorter prompt that produces the same behavior is always better — it leaves more context for retrieval results.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| MRKL agent via LangChainGo | Iterative tool-use loop | Agent can search, read, evaluate, and synthesize in multiple steps. |
| Four tools | search, lookup, read, list | Semantic + exact search, selective reading, structural browsing. Each has a distinct job. No overlap. |
| Provenance-only search results | search/lookup return metadata, not content | Agent controls what fills its context. Evaluate relevance first, read content second. |
| Progressive read | TOC by default, content by range | Large docs never blow up agent context. Agent drills down to what it needs. |
