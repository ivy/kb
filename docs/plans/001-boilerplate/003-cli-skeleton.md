# 003: CLI Skeleton

**Dependencies**: 001-config, 002-interfaces
**Blocks**: 006-composition-root
**Parallel with**: 004-server-handlers, 005-test-scaffolding

Cobra command tree with all subcommands stubbed. Each command parses flags and prints "not implemented" — the real logic comes in later implementation streams.

## Deliverables

### internal/cli/root.go

- Root cobra command with global flags: `--config`, `--log-level`, `--log-format`
- `PersistentPreRunE`: loads config via `config.Load()`, sets up logging, stores config in context
- `func Execute() int` — runs root command, returns exit code
- Logging setup per parent plan: `log/slog` with journal or text handler

### internal/cli/version.go

- `kb version` command
- Build metadata vars: `version`, `gitRef`, `buildTag`, `buildDate` (injected via ldflags)
- Plain text output by default, `--json` flag for structured output
- This is the one command that should fully work after this slice

### internal/cli/serve.go

- `kb serve` command stub
- Placeholder `runServe` that loads config and prints "starting server..." then exits
- Real wiring comes in 006-composition-root

### internal/cli/setup.go

- `kb setup` command stub
- Will run migrations, check prerequisites, configure webhooks

### internal/cli/sources.go

- `kb sources` with subcommands: `add`, `list`, `remove`
- All stubbed with flag parsing

### internal/cli/reindex.go

- `kb reindex [url]` command stub

### internal/cli/status.go

- `kb status` command stub

## Dependencies to add

- `github.com/spf13/cobra`

## Acceptance

- `kb version` prints build info (plain text and `--json`)
- `kb serve`, `kb setup`, `kb sources list`, `kb reindex`, `kb status` all parse flags and exit cleanly
- `kb --help` shows all commands
- Config loads in PersistentPreRunE and is available to subcommands via context
- `make build && bin/kb version` works end-to-end

## Notes

- `Execute()` returns `int` not `error` — main.go calls `os.Exit(cli.Execute())`
- Logging uses `slog.Default()` — no custom logger abstraction per parent plan
- The journal handler (`github.com/jcfergus/journal-handler` or similar) is a nice-to-have; text handler is fine for now
