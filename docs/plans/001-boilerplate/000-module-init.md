# 000: Module Init

**Dependencies**: none
**Blocks**: all other slices

Foundation slice. Creates the Go module, directory tree, tooling config, and build system. Everything else depends on this being committed first.

## Deliverables

### go.mod

```
module github.com/ivy/kb

go 1.26
```

No dependencies yet — each subsequent slice adds its own.

### mise.toml

```toml
[tools]
go = "1"
golangci-lint = "2"
"go:github.com/onsi/ginkgo/v2/ginkgo" = "2.28"
```

### Directory structure

Create all package directories with `.gitkeep` or a doc.go so they exist in git:

```
cmd/kb/
internal/cli/
internal/config/
internal/server/
internal/agent/
internal/store/
internal/store/migrations/
internal/queue/
internal/index/
internal/index/testdata/
internal/index/testdata/archives/
internal/embed/
internal/session/
internal/server/testdata/
```

### cmd/kb/main.go

Minimal entry point:

```go
package main

import (
    "os"

    "github.com/ivy/kb/internal/cli"
)

func main() {
    os.Exit(cli.Execute())
}
```

### Makefile

Full Makefile from the parent plan — build, test, test-unit, lint, clean targets with ldflags version injection.

### .gitignore

Add `bin/` to existing `.gitignore`.

## Acceptance

- `go build ./...` succeeds (with stub cli.Execute returning 0)
- `make build` produces `bin/kb`
- `mise install` installs Go, golangci-lint, and ginkgo
- Directory tree matches the parent plan's package tree

## Notes

- `cli.Execute()` returns `int` (exit code), not `error` — keeps main.go dead simple
- Stub `cli/root.go` needs to exist with `func Execute() int { return 0 }` for the build to pass
