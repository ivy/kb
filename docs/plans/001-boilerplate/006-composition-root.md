# 006: Composition Root

**Dependencies**: 001-config, 002-interfaces, 003-cli-skeleton, 004-server-handlers, 005-test-scaffolding
**Blocks**: nothing — this is the capstone

Wires everything together in `cli/serve.go`. This is the last slice because it touches every package. Once this lands, `kb serve` boots (and immediately fails with "not implemented" from the placeholder impls, which is correct).

## Deliverables

### internal/cli/serve.go (replace stub from 003)

Full wiring as specified in the parent plan's composition root section:

```go
func runServe(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()
    cfg := configFromContext(ctx)

    // 1. Store
    pgStore, err := store.NewPGStore(ctx, cfg.Database)
    // 2. Migrations
    pgStore.Migrate(ctx)
    // 3. Queue (two-phase init for ReindexHandler → Queue cycle)
    q, err := queue.NewRiverQueue(pgStore.Pool(), cfg.Workers, handlers)
    // 4. Sessions
    sessions := session.NewLRUManager(cfg.Sessions)
    // 5. Agent
    ag := agent.New(pgStore, pgStore, cfg.Agent)
    // 6. Server
    srv := server.New(ag, sessions, q, cfg.Tailscale)
    return srv.Serve(ctx)
}
```

### Two-phase init for ReindexHandler

The parent plan notes: ReindexHandler needs Queue to enqueue embed jobs, but Queue needs handlers at construction. Resolve with two-phase init:

1. Create Queue with handler map
2. Inject Queue into ReindexHandler after construction

Or use closure capture. Pick whichever is cleaner at implementation time.

### internal/cli/setup.go (replace stub from 003)

Wire migrations:

```go
func runSetup(cmd *cobra.Command, args []string) error {
    cfg := configFromContext(cmd.Context())
    pgStore, _ := store.NewPGStore(cmd.Context(), cfg.Database)
    return pgStore.Migrate(cmd.Context())
}
```

### internal/cli/status.go, sources.go, reindex.go (replace stubs from 003)

Wire to store and queue interfaces. Still return "not implemented" from the underlying impls, but the CLI plumbing is real:

- `kb sources list` → calls `store.ListSources()`
- `kb reindex <url>` → calls `queue.Enqueue()` with a reindex job
- `kb status` → calls `store.ListSources()` + `queue.Stats()`

### Graceful shutdown

`srv.Serve(ctx)` should handle `SIGINT`/`SIGTERM`:

1. Stop accepting new connections
2. `queue.Stop(ctx)` — drain in-flight jobs
3. Clean exit

## Acceptance

- `kb serve` attempts to boot (connects to store, runs migrations, starts queue and server)
- Fails with "not implemented" from placeholder impls — this is the expected behavior
- `kb setup` runs migrations (placeholder)
- `kb sources list`, `kb reindex`, `kb status` call through to interfaces
- Signal handling works (Ctrl-C clean exit)
- No circular imports in the full dependency graph

## Notes

- This slice is intentionally last. It's the integration point — if any interface is wrong, this is where you find out.
- The parent plan's dependency graph should hold: config/, store/, queue/, session/, agent/, index/, embed/, server/ have no circular dependencies.
- After this slice, all 8 parallel implementation streams from the parent plan can proceed independently.
