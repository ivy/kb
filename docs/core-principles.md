# Core Principles

Decision filters for KB. Every principle has teeth — it should be possible to violate it, and doing so should feel wrong.

## 1. Built for Agents

Agents are the primary consumers of KB. Every interface decision optimizes for what LLMs produce and consume naturally: plain text, not structured formats that require precise syntax. Markdown with footnotes, not JSON with nested objects. Curl-able endpoints, not generated clients.

This isn't about dumbing things down. It's about recognizing that the caller's strengths are language comprehension and generation — not serialization and parsing. An interface that plays to those strengths gets used correctly more often.

**Says no to**: JSON request bodies where plain text suffices. Response formats that require parsing to extract meaning. Protocols that need code generation or specialized clients. Any interface that's easier for a programmer to implement than for an agent to call.

## 2. KB Indexes, Repos Own

Every piece of knowledge traces to a source file in a Git repo. KB is the search layer, not the storage layer. When knowledge is wrong, corrections go to the source — not the index.

This is the foundation everything else rests on. It determines the data model (chunks reference source files), the correction model (propose changes upstream), and what KB is allowed to forget (anything it can re-index).

**Says no to**: Knowledge that exists only in KB with no traceable origin. Corrections that patch the index without fixing the source. KB becoming a second source of truth that competes with the repos it indexes.

## 3. Answers Over Chunks

The KB agent understands context and returns synthesized answers, not ranked lists of text fragments. Raw vector search is a building block inside the agent, not the interface other agents consume.

An agent asking "what's the deployment pattern for this service?" needs an answer it can act on — with caveats about staleness, conflicts between sources, and gaps in coverage. A list of the five nearest chunks doesn't do that.

**Says no to**: Exposing raw search results as the primary interface. Treating retrieval as the product. Returning results without synthesis or context about reliability.

## 4. Provenance Is Load-Bearing

Every result carries a source repo, file path, and last-indexed timestamp. Provenance isn't decorative metadata — it's how you catch staleness, trace errors, and route corrections to the right place.

When a calling agent discovers that KB's answer contradicts reality, provenance is what makes that actionable: here's where the bad information lives, here's how old it is, here's where the fix goes.

**Says no to**: Chunks without source attribution. Results with no freshness indicator. "Trust me, this is current."

## 5. Index What Exists

The repos have documentation now. Some of it is stale. Index it anyway. Stale docs that are searchable and flagged as stale are more useful than perfect docs that aren't indexed.

Don't block the service on a perfect ingestion pipeline or a documentation cleanup sprint. The value is in making existing knowledge findable, not in curating it first.

**Says no to**: "We need to reorganize the docs first." Waiting for every repo to have clean documentation. Building elaborate intake pipelines before search works at all.

## 6. Usefulness Is the Metric

If KB costs more time than it saves, something is wrong. The first version makes one real agent interaction noticeably better — not perfect, just measurably better than an agent starting cold.

This is the principle that kills features. Every addition has to clear the bar: does this reduce the context overhead that KB exists to solve? If the answer is "not yet, but eventually" — it waits.

**Says no to**: Spending more time building KB than doing the work KB is supposed to improve. Features that sound good but don't reduce context overhead today. Optimizing for completeness when usefulness is what matters.

## 7. Corrections Flow Upstream

When an agent discovers incorrect or outdated knowledge, corrections route back to the source. KB tracks what was wrong, where the fix belongs, and when it was proposed. Humans review at their own pace.

Sessions enable this naturally — a builder queries KB, learns something is outdated mid-conversation, and the correction gets captured in context rather than lost when the session ends.

**Says no to**: Silent index mutations with no audit trail. Corrections that dead-end inside KB without reaching the source. Knowledge that changes and nobody knows why.
