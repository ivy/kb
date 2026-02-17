# 004: Server & Handler Stubs

**Dependencies**: 002-interfaces
**Blocks**: 006-composition-root
**Parallel with**: 001-config, 003-cli-skeleton, 005-test-scaffolding

HTTP server structure and route handlers as stubs. The server depends on interfaces (agent, session, queue) but not on config loading or CLI wiring — it receives its dependencies via constructor injection.

## Deliverables

### internal/server/server.go

- `Server` struct holding agent, session manager, queue, and tailscale config
- `New(ag agent.Agent, sess session.Manager, q queue.Queue, cfg config.Tailscale) *Server`
- `Serve(ctx context.Context) error` — sets up listener (tsnet or plain TCP based on config), wires mux, blocks until ctx cancels
- Route registration: `/v1/ask`, `/v1/sessions/:id/ask`, `/webhook`, `/llms.txt`

### internal/server/ask.go

- `POST /v1/ask` handler stub — accepts plain text body (principle #1: built for agents), returns placeholder response
- `POST /v1/sessions/:id/ask` handler stub — same but with session context

### internal/server/webhook.go

- `POST /webhook` handler stub — accepts GitLab system hook payload, enqueues reindex job via queue interface

### internal/server/llmstxt.go

- `GET /llms.txt` handler — can be fully implemented in this slice (static content describing KB's capabilities)

### internal/server/auth.go

- Auth middleware stub
- When tsnet+authz enabled: WhoIs lookup + capability check
- When tsnet without authz: tsnet listener, no capability checks
- When tsnet disabled: plain TCP, no auth
- Three modes per parent plan's Tailscale config

## Acceptance

- `server.New(...)` compiles with interface dependencies
- Route handlers are registered on the mux
- `/llms.txt` returns actual content
- Auth middleware compiles in all three modes (even if tsnet stubs are no-ops)
- No circular imports: server → agent, session, queue (never the reverse)

## Notes

- Plain text request/response for `/v1/ask` aligns with principle #1 (built for agents). Markdown with footnotes for provenance, not JSON.
- The server uses stdlib `net/http` — no framework
- tsnet integration may need a build tag or interface to avoid pulling in the full tailscale dependency for tests
