# 001: Config Types & Loading

**Dependencies**: 000-module-init
**Blocks**: 003-cli-skeleton, 006-composition-root

Config is the shared vocabulary. Every package receives a slice of the config struct, so this needs to land before CLI wiring or composition root work begins. However, interface definitions (002) don't depend on config types — they reference them but can use forward declarations or be committed alongside.

## Deliverables

### internal/config/config.go

Full `Config` struct with all nested sub-structs as specified in the parent plan:

- `Config` (top-level)
- `Database`, `Tailscale`, `GitLab`, `Embedding`, `Agent`, `Sessions`, `Workers`, `Log`

`Load() (*Config, error)` function using viper:

- Reads from `kb.yaml` (or path specified by `--config` flag)
- Binds environment variables with `KB_` prefix (e.g., `KB_DATABASE_URL`)
- Returns validated config

### Dependencies to add

- `github.com/spf13/viper`

## Acceptance

- `config.Load()` reads from YAML file
- `config.Load()` reads from env vars with `KB_` prefix
- Env vars override file values
- All sub-structs are exported and tagged with `mapstructure`

## Notes

- Validation is intentionally minimal at this stage — "does the required field exist?" not "is the database reachable?"
- The parent plan specifies ISP at the package boundary: each package receives only its config slice (e.g., `store.New(ctx, cfg.Database)`)
