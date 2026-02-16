# MVP User Stories

Vocabulary:

- **Caller**: Any agent that queries KB for context (Claude Code session, CLAUDE.md agent, Obsidian agent, etc.)
- **KB Agent**: The knowledge agent that receives queries, retrieves, synthesizes, and converses
- **Human**: The person who configures what gets indexed and reviews behavior

## Caller Stories

### C1. Ask a question with context

As a caller, I want to send the KB agent a question along with my working context (project, task, what I'm trying to do) so I get an answer relevant to my situation, not a generic one.

### C2. Get an answer I can act on

As a caller, I want to receive a synthesized answer — not a list of chunks — so I can act on it without doing my own retrieval and assembly.

The KB agent should communicate what it knows, what it doesn't, and how fresh its sources are. When sources conflict or are stale, that's part of the answer. When nothing relevant exists, the KB agent says so rather than fabricating.

### C3. Know where the answer came from

As a caller, I want every answer to include source repos, file paths, and last-indexed timestamps so I can judge freshness and trace back to the source if something seems wrong.

### C4. Have a conversation

As a caller, I want to ask follow-up questions within the same session so I can refine my understanding without re-explaining my context each time.

## Human Stories

### H1. Trigger indexing

As the human, I want to trigger indexing of new or updated repos so KB stays current as my repos change.

## Deferred

- **Access control / zone scoping** — not needed for single-human homestead MVP
- **Upstream corrections** — MVP is read-only; corrections flow is a future concern
- **Fine-grained index control** — repo-level config is sufficient for now; path/file-type filtering can come later
- **Index observability dashboard** — provenance on answers (C3) covers freshness discovery per-query; dedicated status view can wait
- **Non-GitLab sources** — all current sources (code repos, Obsidian vault, OpenClaw agents) are already Git repos pushed to GitLab
