# KB

Knowledge service for a personal homestead. Indexes GitLab repos, serves synthesized answers to agents via a knowledge agent. Not a search engine — agents ask questions, get answers with provenance.

- See `docs/vision.md` for full context and goals
- See `docs/core-principles.md` for decision filters

## Local Instructions

@AGENTS.local.md

## Repo Map

```
cmd/kb/          CLI entrypoint (cobra)
internal/
  agent/         Knowledge agent (query synthesis)
  chunk/         Markdown chunker (heading hierarchy, token limits)
  cli/           Command definitions (serve, setup, sources, reindex, status, version)
  config/        Viper-based config loading
  embed/         Embedding handler
  index/         Reindex handler, coordinates source→chunk→embed pipeline
  queue/         River-based async job queue
  server/        HTTP server (Tailscale-aware)
  session/       LRU session manager
  source/        Source adapters (GitLab archive fetcher)
  store/         PostgreSQL + pgvector persistence
docs/
  architecture/  Per-subsystem design docs (agent, auth, indexing, query, admin)
  plans/         Implementation plans
```

## Commands

Tools are managed with `mise` (`mise.toml`). Run `mise install` to bootstrap.

| Task | Command |
|------|---------|
| Build | `make build` |
| Test (all) | `make test` |
| Test (no DB) | `make test-unit` |
| Lint | `make lint` |
| Verify before PR | `make lint && make test-unit` |

See `Makefile` for build flags and ldflags.

## Testing

Tests use **Ginkgo** with **Gomega** matchers. Each package has a `_suite_test.go` bootstrap file. Fakes are hand-written (not generated) and live in `fake_test.go` alongside the tests that use them.

- Test fixtures: `internal/index/testdata/`, `internal/chunk/testdata/`, `internal/source/testdata/`
- DB-dependent tests are excluded from `make test-unit` via `--label-filter='!db'`

## Architecture

The serve command (`internal/cli/serve.go`) wires everything together:
store → queue (River) → handlers (reindex, embed) → agent → server

Each `internal/` package defines interfaces at the top of its main file. See `docs/architecture/` for per-subsystem design.

## Config

Viper loads config from the path passed via `--config`. See `internal/config/config.go` for the full struct. Key sections: `database`, `tailscale`, `gitlab`, `embedding`, `workers`, `log`, `sessions`, `agent`.

## Change Hygiene

- Small, focused diffs — one logical change per commit
- Add or update tests for any behavioral change
- Follow existing patterns in the package you're modifying
- Don't commit `.env`, credentials, or `kb.local.yaml` with real tokens
- Run `make lint && make test-unit` before opening a PR
