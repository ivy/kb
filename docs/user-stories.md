# MVP User Stories

Vocabulary:

- **Caller**: Any agent that queries KB for context (Claude Code session, CLAUDE.md agent, Obsidian agent, etc.)
- **KB Agent**: The knowledge agent that receives queries, retrieves, synthesizes, and converses
- **Admin**: Any agent or human that manages KB — configuring sources, triggering indexing, observing state

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

### C5. Choose a model

As a caller, I want to optionally specify which LLM the KB agent uses for synthesis so I can trade off cost, speed, and quality based on my needs.

## Admin Stories

### A1. Manage sources

As an admin, I want to add, list, and remove sources so I control what knowledge KB indexes.

### A2. Trigger indexing

As an admin, I want to trigger reindexing of a source (or all sources) so KB stays current when I know something changed.

### A3. Check system health

As an admin, I want to see KB's status — what's indexed, what jobs are running, whether webhooks are configured — so I can diagnose issues.

### A4. Set up integrations

As an admin, I want to configure webhooks and prerequisites so KB stays current automatically without manual reindexing.

## Deferred

- **Access control / zone scoping** — not needed for single-human homestead MVP
- **Upstream corrections** — MVP is read-only; corrections flow is a future concern
- **Fine-grained index control** — repo-level config is sufficient for now; path/file-type filtering can come later
- **Non-GitLab sources** — all current sources (code repos, Obsidian vault, OpenClaw agents) are already Git repos pushed to GitLab
