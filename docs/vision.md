# Vision

## The Problem

Context is the bottleneck. Not compute, not tools, not the agents themselves.

Every agent interaction across every project starts cold. The same context gets re-explained, the same mistakes get repeated, the same stale docs get treated as truth. When an agent needs to understand how we do things — deployment patterns, architectural decisions, incident learnings — it either gets lucky with file traversal or the human becomes the bottleneck, copy-pasting between sessions.

This isn't theoretical. It shows up constantly:

- An agent deployed a 6-year-old container image because it didn't know about the supply chain policy (INC-002)
- Stale planning docs listed "deploy GitLab" as pending when GitLab had been running for days — and a fresh agent repeated the error to me with full confidence
- Every new conversation burns time rebuilding context that already exists somewhere

The atlas vision's hierarchy of context sins:

1. **Incorrect information** — actively harmful; agents act on false beliefs
2. **Missing information** — costly, but the agent might know to ask
3. **Too much noise** — inefficient but manageable

KB exists to fix #1 and #2. If agents had accurate, relevant context on demand, most of these failures wouldn't happen.

## What KB Does

KB is the brain of the homestead — a centralized knowledge service that gives agents access to the right information, scoped to what they're allowed to see.

It runs as a wellspring platform service. It indexes knowledge from GitLab repos — project documentation, architecture decisions, incident reports, operational runbooks. It tracks canonical URLs, source repos, and when each piece of knowledge was last indexed.

But KB isn't a search engine. Queries go through a knowledge agent that understands the context of the question, cross-references sources, and returns a synthesized answer — not a ranked list of chunks. It's a conversation between agents, not a dumb query store.

When knowledge is wrong, corrections route back to the source. The knowledge agent decides where the fix belongs — a PR on a project repo, an update to operational docs, an edit in the Obsidian vault — creating an audit trail that humans can review, approve, or revert.

## The Access Model

Agents only see what their zone is allowed to access. A grove agent querying KB gets results from repos grove can read in GitLab — project code, shared architecture docs, operational runbooks. It never sees hearth data — personal journal, financial records, private reflections.

Access control derives from GitLab's permission model. The same system that governs code access governs knowledge access. One source of permissions, not two.

## The End State

Any agent, in any zone, asks "what do I need to know about X?" and gets an accurate, relevant, zone-appropriate answer. No copy-paste. No cold starts. No stale docs leading agents astray.

When something changes — a service gets deployed, an incident teaches a lesson, an architecture decision gets made — the knowledge updates. When the knowledge is wrong, corrections propagate back to the source. The system gets smarter over time, not staler.

## What KB Is Not

- **Not a document store.** Git repos are the source of truth. KB indexes and serves them. Corrections route upstream as PRs.
- **Not a search engine.** Raw vector search is a building block, not the interface. Agents talk to a knowledge agent that understands context and intent.
- **Not a local tool.** The original KB prototype was an MCP server for Obsidian vaults on a laptop. The constraints changed when the homestead became the platform. This is a wellspring service, not a desktop app.
- **Not a product.** Personal tool for one homestead. Will never be packaged or supported.

## How I Stay Honest

- If I'm spending more time on KB than on the work KB is supposed to improve, something is wrong.
- The first version should make one real agent interaction noticeably better. Not perfect — just measurably better than copy-paste.
- Don't build the perfect knowledge system. Build one that's more useful than not having one.
- If the knowledge agent becomes a bottleneck (too slow, too expensive, too wrong), simplify. A fast dumb search that works beats a slow smart agent that doesn't.
