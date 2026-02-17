# 004: MRKL Knowledge Agent

## Context

The indexing pipeline is complete — GitLab repos get fetched, chunked, embedded, and stored in pgvector. But KB can't answer a single question yet. `POST /v1/ask` returns "not implemented." The agent is the core value proposition: callers ask questions in plain text, the agent retrieves knowledge and returns synthesized Markdown answers with provenance footnotes.

The architecture (`docs/architecture/agent.md`) specifies a MRKL-style agent with four retrieval tools: `search`, `lookup`, `read`, `list`. LangChainGo provides the agent executor. This plan implements the full agent and the HTTP handlers.

## Scope

**In scope:**
- MRKL agent implementation via LangChainGo (`internal/agent/`)
- Four agent tools: search, lookup, read, list
- Store methods needed by tools: `LookupByKeyword`, `GetDocument`, `ListFiles`
- Agent config expansion (LLM provider, API key, model)
- `POST /v1/ask` handler (text in → markdown out)
- `POST /v1/sessions/:id/ask` handler (stateless for now — accepts the route, ignores history)
- Tests for all of the above

**Out of scope:**
- Session manager implementation (LRU + JSONL) — deferred, stubs remain
- Tailscale auth (deferred)
- Webhook handler (deferred, `kb reindex` is the manual fallback)
- `kb reindex` without URL arg (minor CLI improvement)
- Correction flow (deferred per architecture doc)
- E2E integration tests for serve pipeline (separate follow-up)

**Planning ahead for sessions:** The `Agent.Ask()` signature already accepts `[]session.Turn` for history. The `session.Manager` interface is already defined. The HTTP handlers accept the session route and will wire in session state when the manager is implemented. No abstractions need to change — sessions slot in later by implementing `LRUManager` and adding the create/get/append calls in the handlers.

## Dependencies

LangChainGo v0.1.14 is already in go.mod. It provides:
- `github.com/tmc/langchaingo/agents` — `OneShotAgent` + `Executor`
- `github.com/tmc/langchaingo/tools` — `Tool` interface (`Name()`, `Description()`, `Call()`)
- `github.com/tmc/langchaingo/llms/openai` — OpenAI-compatible LLM client
- `github.com/tmc/langchaingo/llms/anthropic` — Anthropic LLM client
- `github.com/tmc/langchaingo/chains` — `chains.Run()` / `chains.Call()` to execute

Both OpenAI and Anthropic providers implement `llms.Model`. The agent executor works identically with either — swap the LLM constructor and it just works.

## LangChainGo Agent Mechanics

How the executor loop works (important for understanding the implementation):

1. `Executor.Call()` loops up to `MaxIterations` (default 5)
2. Each iteration calls `agent.Plan(ctx, intermediateSteps, inputs)` which sends the prompt + conversation so far to the LLM
3. The LLM returns either actions (`Action: search`, `Action Input: deployment pattern`) or a final answer (`Final Answer: ...`)
4. If actions: executor calls `tool.Call(ctx, input)`, appends the observation, loops
5. If final answer: executor returns the result
6. If max iterations reached: returns `ErrNotFinished`

The `Tool` interface is three methods:
```go
type Tool interface {
    Name() string
    Description() string
    Call(ctx context.Context, input string) (string, error)
}
```

Tools receive a single string input and return a single string output. The agent parses the tool name and input from the LLM's text output. Tool descriptions go into the system prompt — they're how the LLM knows what each tool does and when to use it.

## Implementation Order

### Step 1: Expand agent config

The `config.Agent` struct currently has only `DefaultModel`. The agent needs to know which LLM provider to use and needs an API key.

```go
type Agent struct {
    Provider     string  `mapstructure:"provider"`      // "openai" or "anthropic"
    APIKey       string  `mapstructure:"api_key"`        // LLM API key
    DefaultModel string  `mapstructure:"default_model"`  // set automatically per provider if empty
    MaxIter      int     `mapstructure:"max_iter"`        // max agent loop iterations, default 15
    Thinking     string  `mapstructure:"thinking"`        // "none", "low", "medium", "high", "auto"
    Temperature  float64 `mapstructure:"temperature"`    // sampling temperature, default 0.3
    MaxTokens    int     `mapstructure:"max_tokens"`      // max response tokens, default 16384
}
```

**Default models per provider** (applied when `default_model` is empty):

| Provider | Default Model |
|----------|---------------|
| `openai` | `gpt-5.2` |
| `anthropic` | `claude-sonnet-4-5-20250929` |

**Thinking modes:** LangChainGo abstracts both providers behind `llms.WithThinkingMode()` — maps to Anthropic's extended thinking and OpenAI's `reasoning_effort`. The executor doesn't pass `CallOption` through to the LLM, so thinking mode is baked into the LLM at construction time via config. Default is `none` — the agent's task (retrieval + synthesis) doesn't require deep reasoning, and thinking tokens cost extra latency.

| Mode | Anthropic | OpenAI | Use case |
|------|-----------|--------|----------|
| `none` | No thinking | No reasoning_effort | Default — fast, cheap |
| `low` | ~20% budget | `low` | Light reasoning |
| `medium` | ~50% budget | `medium` | Moderate reasoning |
| `high` | ~80% budget | `high` | Heavy reasoning |
| `auto` | Model decides | n/a | Let model choose |

**Why default 15 iterations:** The progressive disclosure pattern (TOC then content) doubles the read cost per document. A cross-project question like "What's the security model?" needs 3-4 documents × 2 reads each (6-8 iterations), plus 2-3 searches/lookups, plus a coverage `list` call — that's 10-12 tool calls before the final answer. At 10 iterations, the agent runs out of budget on exactly the complex cross-project queries KB exists to answer well. 15 gives room for broadening searches and checking additional sources. Simple questions still finish in 4-5 iterations — the model self-terminates when it has enough.

**Temperature default 0.3:** The agent has two modes per LLM call: tool selection (structured `Action: ...\nAction Input: ...` output, ~100 tokens, needs reliable parsing) and final synthesis (Markdown prose + footnotes, constrained by source material). Both favor low temperature. 0.0 risks parser loops if the model produces identical malformed output twice — a small amount of randomness breaks that. 0.3 is the sweet spot between determinism and resilience.

**MaxTokens default 16384:** The cap applies per LLM call in the agent loop — during tool-use steps the model outputs ~100 tokens, so it only matters for the final answer. Real answers from this portfolio include verbatim code blocks (Tailscale ACL policy JSON, TOML config files, GraphQL queries, systemd unit templates), incident postmortem tables (system profiles, timelines), and multi-step deployment procedures with CLI commands. Complex cross-project answers run 4000-5000 tokens. With thinking enabled, the budget may come from this pool. 16384 is generous — most answers land well under it, but the headroom means the agent never truncates a thorough answer. You only pay for tokens generated.

**Not exposed in config (intentionally):** `top_p`, `top_k`, `frequency_penalty`, `presence_penalty`, `seed`, `stop_words`. These are either redundant with temperature, solve problems we don't have yet, or are managed internally by the agent executor. Prompt caching is enabled unconditionally for Anthropic as an implementation detail.

Config YAML:
```yaml
agent:
  provider: openai
  api_key: ""               # or KB_AGENT_API_KEY env var
  # default_model: gpt-5.2  # auto-set per provider if omitted
  max_iter: 15
  thinking: none
  temperature: 0.3
  max_tokens: 16384
```

**Files:** `internal/config/config.go`

### Step 2: Store methods for agent tools

Three methods need implementing. One needs a new migration.

#### `LookupByKeyword` — full-text search

PostgreSQL full-text search against chunk content. Uses `to_tsvector` / `plainto_tsquery`.

```sql
SELECT c.content_hash, c.content,
       ts_rank(to_tsvector('english', c.content), plainto_tsquery('english', $1)) AS score,
       p.source_url, p.file_path
FROM chunks c
JOIN provenance p ON p.content_hash = c.content_hash
WHERE to_tsvector('english', c.content) @@ plainto_tsquery('english', $1)
ORDER BY score DESC
LIMIT $2
```

Needs a GIN index on `chunks.content` for performance:

```sql
-- 003_fulltext_index.up.sql
CREATE INDEX idx_chunks_content_fts ON chunks
    USING GIN (to_tsvector('english', content));
```

#### `GetDocument` — fetch chunks for a source file

Returns all chunks for a given source URL + file path, ordered by position.

```sql
SELECT p.content_hash, c.content
FROM provenance p
JOIN chunks c ON c.content_hash = p.content_hash
WHERE p.source_url = $1 AND p.file_path = $2
ORDER BY p.position
```

Already matches the `ProvenanceRow` return type. No schema changes needed.

#### `ListFiles` — list files within a source

New method. The agent's `list` tool needs to enumerate files within a source, not just sources.

```go
ListFiles(ctx context.Context, sourceURL string) ([]string, error)
```

```sql
SELECT DISTINCT file_path FROM provenance
WHERE source_url = $1
ORDER BY file_path
```

Add to `Store` interface.

**Files:**
- `internal/store/migrations/003_fulltext_index.up.sql` — new
- `internal/store/migrations/003_fulltext_index.down.sql` — new
- `internal/store/store.go` — add `ListFiles` to `Store` interface
- `internal/store/pgstore.go` — implement `LookupByKeyword`, `GetDocument`, `ListFiles`

### Step 3: Embedding helper for search tool

The `search` tool needs to embed the query text before calling `SearchSimilar`. It needs access to the same OpenAI embeddings API used by the embed handler.

Extract the embedding API call into a reusable function:

```go
// internal/embed/client.go
type Client struct {
    httpClient *http.Client
    baseURL    string
    apiKey     string
    model      string
}

func NewClient(cfg config.Embedding) *Client { ... }
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) { ... }
```

The embed handler (`internal/embed/handler.go`) should use this client internally too — refactor to avoid duplication.

**Files:**
- `internal/embed/client.go` — new, extracted from handler
- `internal/embed/handler.go` — refactor to use `Client`

### Step 4: Agent tools

Four tools, each implementing `tools.Tool`. All live in `internal/agent/tools/`.

#### `search` — semantic vector search

```go
type SearchTool struct {
    embedder *embed.Client
    vectors  store.VectorStore
}

func (t *SearchTool) Name() string        { return "search" }
func (t *SearchTool) Description() string  { return "Semantic vector search..." }
func (t *SearchTool) Call(ctx context.Context, input string) (string, error) {
    // 1. Embed the query text
    vec, err := t.embedder.Embed(ctx, input)
    // 2. SearchSimilar(ctx, vec, 10)
    // 3. Format results as text with provenance metadata
}
```

Output format (text, not JSON — principle #1):
```
1. grove/atlas — docs/deploy.md (indexed 2026-02-15, ref abc1234)
   Heading: Architecture > Deployment
2. grove/policy — docs/supply-chain.md (indexed 2026-02-14, ref def5678)
   Heading: Policy > Container Images
```

**Design note:** The architecture says search/lookup return provenance metadata only, no content. The agent reads content via the `read` tool. But `SearchSimilar` currently returns content in `SearchResult`. Two options:

- **A. Strip content from search output.** The tool has the content but doesn't show it. The agent must use `read` to see content. Matches architecture exactly — forces deliberate context management.
- **B. Include a snippet.** Show first ~200 chars of content. Saves an agent iteration for obvious matches. Practical but drifts from architecture.

**Going with A** — the architecture is explicit about this. The agent learns to search → evaluate metadata → read selectively. We can relax later if the extra iterations hurt latency too much.

However, `SearchSimilar` needs to return provenance metadata (heading hierarchy, indexed_at, ref) that it currently doesn't. Need to expand the query to join provenance metadata:

```sql
SELECT e.content_hash, c.content, (e.embedding <=> $1) AS score,
       p.source_url, p.file_path, p.metadata, p.indexed_at, p.ref
FROM embeddings e
JOIN chunks c ON c.content_hash = e.content_hash
JOIN provenance p ON p.content_hash = e.content_hash
ORDER BY e.embedding <=> $1
LIMIT $2
```

And expand `SearchResult` to carry the extra fields:
```go
type SearchResult struct {
    ContentHash []byte
    Content     string
    Score       float64
    SourceURL   string
    FilePath    string
    Metadata    json.RawMessage  // heading hierarchy etc.
    IndexedAt   time.Time
    Ref         string
}
```

This affects `SearchSimilar` and `LookupByKeyword` — both return `[]SearchResult`.

#### `lookup` — keyword/identifier search

```go
type LookupTool struct {
    store store.Store
}

func (t *LookupTool) Name() string        { return "lookup" }
func (t *LookupTool) Description() string  { return "Exact keyword/identifier search..." }
func (t *LookupTool) Call(ctx context.Context, input string) (string, error) {
    // 1. LookupByKeyword(ctx, input, 10)
    // 2. Format results same as search (provenance metadata only)
}
```

Same output format as search. Same provenance metadata fields.

#### `read` — progressive document fetch

```go
type ReadTool struct {
    store store.Store
}

func (t *ReadTool) Name() string        { return "read" }
func (t *ReadTool) Description() string  { return "Fetch content from a specific document..." }
func (t *ReadTool) Call(ctx context.Context, input string) (string, error) {
    // Parse input: "source_url file_path [position or range]"
    // If no position: return TOC (heading structure from metadata)
    // If position/range: return chunk content at those positions
}
```

Input parsing: the agent writes free-form text. Parse it as space-separated tokens:
- `https://gitlab.example.com/grove/atlas docs/deploy.md` → TOC
- `https://gitlab.example.com/grove/atlas docs/deploy.md 1-3` → chunks at positions 1-3

TOC mode uses `GetDocument` to fetch all chunks, then extracts heading hierarchy from provenance metadata to build a table of contents. Content mode returns the chunk text at the requested positions.

#### `list` — browse indexed content

```go
type ListTool struct {
    store store.Store
}

func (t *ListTool) Name() string        { return "list" }
func (t *ListTool) Description() string  { return "Browse what's indexed..." }
func (t *ListTool) Call(ctx context.Context, input string) (string, error) {
    // If input is empty: ListSources()
    // If input is a URL: ListFiles(sourceURL)
}
```

Output format:
```
Sources:
- https://gitlab.example.com/grove/atlas (gitlab, indexed 2026-02-15)
- https://gitlab.example.com/grove/policy (gitlab, indexed 2026-02-14)
```

Or for files within a source:
```
Files in https://gitlab.example.com/grove/atlas:
- docs/deploy.md
- docs/architecture.md
- README.md
```

**Files:**
- `internal/agent/tools/search.go`
- `internal/agent/tools/lookup.go`
- `internal/agent/tools/read.go`
- `internal/agent/tools/list.go`

### Step 5: MRKL agent implementation

Replace the stub `Ask()` with a real LangChainGo agent executor.

```go
func (a *MRKLAgent) Ask(ctx context.Context, question string, history []session.Turn, model string) (string, error) {
    // 1. Create LLM client (openai or anthropic based on config)
    llm, err := a.newLLM(model)

    // 2. Build tools list
    agentTools := []tools.Tool{
        tools.NewSearchTool(a.embedder, a.vectors),
        tools.NewLookupTool(a.store),
        tools.NewReadTool(a.store),
        tools.NewListTool(a.store),
    }

    // 3. Create agent with system prompt
    agent := agents.NewOneShotAgent(llm, agentTools,
        agents.WithPromptPrefix(systemPrompt),
    )

    // 4. Create executor
    executor := agents.NewExecutor(agent,
        agents.WithMaxIterations(a.cfg.MaxIter),
    )

    // 5. Build input with conversation history
    input := a.buildInput(question, history)

    // 6. Run
    result, err := chains.Run(ctx, executor, input)
    return result, err
}
```

**LLM construction:** `newLLM(model)` creates either an OpenAI or Anthropic client based on `config.Agent.Provider`. The `model` parameter overrides `DefaultModel` when non-empty (supports the `X-Model` header from `query.md`). A new LLM is created per `Ask()` call — this is intentional because the executor doesn't pass `CallOption` through to the LLM, so per-request model overrides and thinking mode must be set at construction.

```go
func (a *MRKLAgent) newLLM(model string) (llms.Model, error) {
    if model == "" {
        model = a.cfg.DefaultModel
    }

    // Default model per provider
    if model == "" {
        switch a.cfg.Provider {
        case "openai":
            model = "gpt-5.2"
        case "anthropic":
            model = "claude-sonnet-4-5-20250929"
        }
    }

    // Map thinking config to CallOption
    thinkingOpt := thinkingModeOption(a.cfg.Thinking)

    switch a.cfg.Provider {
    case "openai":
        return openai.New(
            openai.WithToken(a.cfg.APIKey),
            openai.WithModel(model),
            openai.WithCallback(thinkingOpt),  // baked in
        )
    case "anthropic":
        return anthropic.New(
            anthropic.WithToken(a.cfg.APIKey),
            anthropic.WithModel(model),
        )
    default:
        return nil, fmt.Errorf("unsupported LLM provider: %q", a.cfg.Provider)
    }
}
```

Note: the exact mechanism for baking thinking mode into the LLM needs verification during implementation — `llms.WithThinkingMode()` is a `CallOption`, so it may need to be passed via a wrapper or at `GenerateContent` time rather than at construction. If the executor doesn't forward call options, we may need a thin wrapper around the LLM that injects the thinking option on every call.

**Conversation history:** For session continuations, prepend prior turns to the input so the agent has context. Format as a plain text conversation log:

```
Previous conversation:
User: What's the deployment pattern for atlas?
Agent: Atlas uses canary deployments via Nomad...

Current question:
What about rollback procedures?
```

**System prompt:** Embedded as a Go string constant. Taken directly from `docs/architecture/agent.md` — the full system prompt is already written there.

**Constructor change:** `MRKLAgent` needs an `embed.Client` for the search tool:

```go
type MRKLAgent struct {
    store    store.Store
    vectors  store.VectorStore
    embedder *embed.Client
    cfg      config.Agent
}

func New(s store.Store, v store.VectorStore, embedder *embed.Client, cfg config.Agent) Agent {
    return &MRKLAgent{store: s, vectors: v, embedder: embedder, cfg: cfg}
}
```

This changes the constructor signature — `serve.go` needs updating.

**Files:**
- `internal/agent/agent.go` — rewrite
- `internal/agent/prompt.go` — system prompt constant

### Step 6: HTTP handlers

#### `POST /v1/ask`

Text in, Markdown out. Stateless for now — no session creation.

```go
func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
    // 1. Read body as plain text
    body, _ := io.ReadAll(r.Body)
    question := strings.TrimSpace(string(body))

    // 2. Optional model override from X-Model header
    model := r.Header.Get("X-Model")

    // 3. Call agent (no history for stateless request)
    answer, err := s.agent.Ask(r.Context(), question, nil, model)

    // 4. Write response
    w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
    fmt.Fprint(w, answer)
}
```

#### `POST /v1/sessions/:id/ask`

Session continuation route exists but has no session state yet. Behaves identically to `/v1/ask` — the session ID is accepted but history is not loaded. When the session manager is implemented, this handler wires in `Get()` → pass history → `Append()`.

```go
func (s *Server) handleSessionAsk(w http.ResponseWriter, r *http.Request) {
    // sessionID := r.PathValue("id")  // accepted but unused until sessions land
    body, _ := io.ReadAll(r.Body)
    question := strings.TrimSpace(string(body))
    model := r.Header.Get("X-Model")

    // TODO: when session manager is implemented:
    //   sess, err := s.sessions.Get(r.Context(), sessionID)
    //   answer, err := s.agent.Ask(r.Context(), question, sess.Turns, model)

    answer, err := s.agent.Ask(r.Context(), question, nil, model)

    w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
    fmt.Fprint(w, answer)
}
```

**Files:**
- `internal/server/ask.go` — rewrite

### Step 7: Wiring in `serve.go`

Update the serve command to pass the embedding client to the agent constructor.

```go
// Add after creating pgStore:
embedClient := embed.NewClient(cfg.Embedding)

// Update agent creation:
ag := agent.New(pgStore, pgStore, embedClient, cfg.Agent)
```

The session manager is still constructed and passed to the server (keeps the interface wired), but it remains a stub. When sessions are implemented, only `lru.go` and the handlers change.

**Files:**
- `internal/cli/serve.go` — small edit

### Step 8: Tests

Tests follow BDD principles: `Describe` names the behavior/feature, `Context` sets up preconditions ("when...", "with..."), `It` states the expected outcome from the caller's perspective. Fakes for store/embedder/LLM keep tests fast and deterministic.

#### Agent tools (`internal/agent/tools/*_test.go`)

```
Describe("search", func() {
    // The agent's primary retrieval path. Takes a natural language query,
    // embeds it, and returns provenance metadata for the nearest matches.

    Context("with indexed content about deployment patterns", func() {
        It("returns source URLs, file paths, and heading hierarchy", func() { ... })
        It("omits chunk content from results", func() { ... })
        It("returns at most 10 results", func() { ... })
    })

    Context("when the query has no close matches", func() {
        It("returns an empty result with a 'no results' message", func() { ... })
    })

    Context("when the embedding API is unreachable", func() {
        It("returns an error describing the failure", func() { ... })
    })
})

Describe("lookup", func() {
    // Exact keyword/identifier search — complements semantic search for
    // precise terms like incident IDs, service names, config keys.

    Context("with a chunk containing the keyword 'INC-002'", func() {
        It("returns the matching chunk's provenance metadata", func() { ... })
        It("ranks exact matches above partial matches", func() { ... })
    })

    Context("when no chunks contain the keyword", func() {
        It("returns an empty result with a 'no results' message", func() { ... })
    })
})

Describe("read", func() {
    // Progressive document fetch. Without a position, returns a table of
    // contents built from heading hierarchy. With a position/range, returns
    // the chunk content at those positions.

    Context("with a 4-chunk document about deployment", func() {
        Context("when called without a position", func() {
            It("returns the heading hierarchy as a table of contents", func() { ... })
        })

        Context("when called with a single position", func() {
            It("returns the chunk content at that position", func() { ... })
        })

        Context("when called with a position range like '1-3'", func() {
            It("returns chunk content for positions 1 through 3", func() { ... })
        })
    })

    Context("when the source URL and file path don't match any provenance", func() {
        It("returns a 'document not found' message", func() { ... })
    })
})

Describe("list", func() {
    // Structural awareness — lets the agent understand what's indexed
    // before concluding something doesn't exist.

    Context("when called with no input", func() {
        It("returns all indexed sources with their kind and last-indexed date", func() { ... })
    })

    Context("when called with a source URL", func() {
        It("returns the file paths indexed from that source", func() { ... })
    })

    Context("when called with an unknown source URL", func() {
        It("returns an empty list", func() { ... })
    })
})
```

#### Agent (`internal/agent/agent_test.go`)

Test the agent's coordination of LLM and tools, not the LLM's reasoning. Uses a `fakeLLM` that returns predetermined tool calls and final answers.

```
Describe("asking the knowledge agent", func() {
    // The agent receives a question, uses tools to retrieve knowledge,
    // and returns a synthesized answer. These tests verify the orchestration.

    Context("with a question that matches indexed content", func() {
        // fakeLLM returns: Action: search → Action: read → Final Answer
        It("returns the synthesized answer from the agent loop", func() { ... })
    })

    Context("with conversation history from a prior session turn", func() {
        // fakeLLM receives history prepended to the input
        It("includes prior turns in the prompt sent to the LLM", func() { ... })
    })

    Context("with an X-Model override", func() {
        It("constructs the LLM with the overridden model", func() { ... })
    })

    Context("without a model override", func() {
        It("uses the default model for the configured provider", func() { ... })
    })

    Context("with an unsupported provider in config", func() {
        It("returns an error before calling any tools", func() { ... })
    })
})
```

#### HTTP handlers (`internal/server/server_test.go`)

```
Describe("POST /v1/ask", func() {
    // Text in, Markdown out. The handler reads the body as a question,
    // passes it to the agent, and returns the answer.

    Context("with a plain text question in the body", func() {
        It("returns the agent's answer as text/markdown", func() { ... })
    })

    Context("with an X-Model header", func() {
        It("forwards the model name to the agent", func() { ... })
    })

    Context("with an empty request body", func() {
        It("returns 400 Bad Request", func() { ... })
    })
})

Describe("POST /v1/sessions/:id/ask", func() {
    // Session route is wired but stateless until sessions are implemented.

    Context("with a plain text question", func() {
        It("returns the agent's answer (stateless until sessions land)", func() { ... })
    })
})
```

#### Store methods (add to `internal/store/store_test.go`)

```
Describe("keyword search", Label("db"), func() {
    // Full-text search via PostgreSQL tsvector/tsquery. Used by the
    // agent's lookup tool for exact keyword/identifier matches.

    Context("with chunks containing 'canary deployment' and 'blue-green deployment'", func() {
        Context("when searching for 'canary'", func() {
            It("returns the canary chunk ranked above the blue-green chunk", func() { ... })
            It("includes provenance metadata in each result", func() { ... })
        })
    })

    Context("when no chunks match the keyword", func() {
        It("returns an empty slice", func() { ... })
    })
})

Describe("document retrieval", Label("db"), func() {
    // Fetches all chunks for a source file, ordered by position.
    // Used by the agent's read tool for progressive document disclosure.

    Context("with a 3-chunk document at positions 0, 1, 2", func() {
        It("returns chunks ordered by position", func() { ... })
        It("includes the chunk content", func() { ... })
    })

    Context("when the source URL and file path have no provenance records", func() {
        It("returns an empty slice", func() { ... })
    })
})

Describe("file listing", Label("db"), func() {
    // Lists distinct file paths within a source. Used by the agent's
    // list tool to show what's indexed for a given repo.

    Context("with provenance records for 3 files in the same source", func() {
        It("returns the 3 distinct file paths sorted alphabetically", func() { ... })
    })

    Context("when the source has no provenance records", func() {
        It("returns an empty slice", func() { ... })
    })
})
```

## Key Files

| File | Action |
|------|--------|
| `internal/config/config.go` | **Edit** — expand Agent struct |
| `internal/store/store.go` | **Edit** — add `ListFiles`, expand `SearchResult` |
| `internal/store/pgstore.go` | **Edit** — implement `LookupByKeyword`, `GetDocument`, `ListFiles`; expand `SearchSimilar` |
| `internal/store/migrations/003_fulltext_index.up.sql` | **Create** |
| `internal/store/migrations/003_fulltext_index.down.sql` | **Create** |
| `internal/embed/client.go` | **Create** — extracted embedding API client |
| `internal/embed/handler.go` | **Edit** — refactor to use `Client` |
| `internal/agent/tools/search.go` | **Create** |
| `internal/agent/tools/lookup.go` | **Create** |
| `internal/agent/tools/read.go` | **Create** |
| `internal/agent/tools/list.go` | **Create** |
| `internal/agent/agent.go` | **Rewrite** — MRKL agent with LangChainGo |
| `internal/agent/prompt.go` | **Create** — system prompt constant |
| `internal/server/ask.go` | **Rewrite** — real handlers (stateless, session-ready) |
| `internal/cli/serve.go` | **Edit** — wire embedding client into agent |
| `internal/agent/tools/*_test.go` | **Create** |
| `internal/agent/agent_test.go` | **Create** |
| `internal/server/server_test.go` | **Edit** — add handler tests |
| `internal/store/store_test.go` | **Edit** — add store method tests |

## Config Changes

Add to `kb.local.yaml`:
```yaml
agent:
  provider: openai
  api_key: ""       # or KB_AGENT_API_KEY env var
  default_model: gpt-4.1
  max_iter: 15
```

Add to `env.sample`:
```
# LLM API key for the knowledge agent (query-time reasoning)
# Supports OpenAI and Anthropic — set provider in config
KB_AGENT_API_KEY=
```

## Open Questions

1. **LLM for agent vs. embeddings — same key?** The embedding API key (`KB_EMBEDDING_API_KEY`) is for OpenAI embeddings. The agent LLM key (`KB_AGENT_API_KEY`) could be OpenAI or Anthropic. They're separate config paths. If both are OpenAI, the user sets the same key twice. Acceptable for clarity — each has a distinct purpose.

2. **Search result deduplication.** A chunk with provenance from multiple files appears once per provenance row in `SearchSimilar`. The agent might see the same chunk content with different source URLs. Options: deduplicate in SQL (use `DISTINCT ON`), deduplicate in the tool, or let the agent handle it. Starting with SQL `DISTINCT ON (e.content_hash)` to keep results clean.

3. **Session URL in response (deferred).** The architecture says responses include a session continuation URL. The handler will append this after the agent returns — the agent doesn't know the session ID. Deferred until the session manager is implemented.

## Verification

1. `make lint` — clean
2. `make test-unit` — all pass
3. `make test-db` — store method tests pass
4. Manual smoke test:
   ```bash
   # With indexed content already in DB from previous e2e test:
   curl -X POST http://127.0.0.1:8080/v1/ask \
     -d "What deployment patterns are documented?"
   ```
   Expect: Markdown answer with provenance footnotes.
