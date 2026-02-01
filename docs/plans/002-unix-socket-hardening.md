# Unix Socket Hardening

Implements [ADR-002: Unix socket directory security](../adrs/002-unix-socket-directory-security.md).

## Why this matters

Knowledge bases may contain sensitive personal or organizational information. The socket is the only network-level boundary between that data and other local users. If the socket directory is world-accessible, writable by others, or a symlink to an attacker-controlled path, the data is compromised. The server must refuse to start rather than operate insecurely.

**Every check in this plan exists because of a specific threat modeled in ADR-002.** Do not relax, skip, or refactor these checks without re-reading the ADR and understanding which threat you would reintroduce.

## Summary

Extract socket path resolution and binding into a new `internal/socket/` package with mandatory directory validation (Lstat, ownership, mode 0700, no symlinks). Both `kb-server` and `kb-mcp` switch to the shared package, eliminating their duplicated `socketPath()` functions.

## New files

### `internal/socket/doc.go`

Package doc describing purpose: resolve and validate Unix socket paths per ADR-002. Must reference the ADR by path so future readers understand the security rationale.

### `internal/socket/socket.go`

Four functions (~60 lines):

- **`ResolveSocketPath() (string, error)`** — Delegates to `resolveSocketPath("/tmp")`. Exported entry point for both binaries.

- **`resolveSocketPath(base string) (string, error)`** — When `XDG_RUNTIME_DIR` is set, returns `$XDG_RUNTIME_DIR/kb.sock` after validating the directory. When unset, logs a warning, creates `<base>/kb-$UID/` with mode 0700, validates, returns `<base>/kb-$UID/kb.sock`. The `base` parameter exists so tests can use `t.TempDir()` instead of writing to real `/tmp`. This is the only reason for the split — do not expose `base` publicly.

- **`ValidateDir(path string) error`** — `Lstat` the path; reject if symlink, not a directory, wrong uid, or mode != 0700. Error message identifies the specific failed check. Exported so `kb-mcp` can validate at connect time (fail-fast rather than cryptic "connection refused").

- **`ListenSocket(path string) (net.Listener, error)`** — Calls `ValidateDir` on the parent directory *before* any stale-socket removal, then binds. This enforces the ADR-002 ordering invariant structurally: you cannot remove a stale socket without passing validation first. `ListenSocket` validates independently of `ResolveSocketPath` because it is a public API that could be called with any path.

Code comments on `ValidateDir` and `ListenSocket` must reference ADR-002 and explain which threat each check mitigates. This is critical — coding agents will encounter these checks and may be tempted to "simplify" them without understanding the security context.

### `internal/socket/socket_test.go`

All tests use `t.TempDir()` and `t.Setenv()` for isolation. Each test function must include a comment referencing ADR-002 and stating which security invariant it validates, so that agents and future contributors understand these tests are not arbitrary — removing or weakening them reintroduces real attack surface.

| Test | Verifies |
|------|----------|
| `TestValidateDir_ValidDirectory` | Happy path (0700, correct owner, real dir) |
| `TestValidateDir_RejectsWrongMode` | Mode 0755 rejected — prevents world-readable socket dir |
| `TestValidateDir_RejectsSymlink` | Symlink to valid dir rejected — prevents symlink attacks |
| `TestValidateDir_RejectsWrongOwner` | UID mismatch — prevents another user's dir from being trusted. Probes `/private/var/root` (macOS) or `/root` (Linux); skips if path unavailable or running as root |
| `TestValidateDir_RejectsNonDirectory` | Regular file rejected |
| `TestResolveSocketPath_XDGSet` | Returns `$XDG_RUNTIME_DIR/kb.sock` after validation |
| `TestResolveSocketPath_XDGUnset` | Creates fallback dir, returns `<base>/kb-$UID/kb.sock`. Uses unexported `resolveSocketPath(t.TempDir())` to avoid writing to real `/tmp` |
| `TestResolveSocketPath_XDGInvalid` | Insecure `XDG_RUNTIME_DIR` rejected — XDG is not trusted blindly |
| `TestListenSocket_StaleRemoval` | Stale file removed, listener binds successfully |
| `TestListenSocket_RejectsInvalidDir` | Invalid dir blocks bind AND stale file is NOT removed — validates ADR-002 ordering invariant |

## Modified files

### `internal/server/server.go`

- Remove `listenSocket()` function entirely
- `Run()` calls `socket.ListenSocket(s.socketPath)` instead
- Remove `"net"` and `"os"` from imports, add `"github.com/ivy/kb/internal/socket"`

### `cmd/kb-server/main.go`

- Remove local `socketPath()` function
- Call `socket.ResolveSocketPath()`, `log.Fatalf` on error
- Pass resolved path to `server.New()`

### `cmd/kb-mcp/main.go`

- Same changes as kb-server: remove `socketPath()`, use `socket.ResolveSocketPath()`

## Implementation order

1. Create `internal/socket/doc.go`
2. Create `internal/socket/socket.go`
3. Create `internal/socket/socket_test.go` — run `go test ./internal/socket/`
4. Update `internal/server/server.go` — remove `listenSocket`
5. Update `cmd/kb-server/main.go` — use `socket.ResolveSocketPath()`
6. Update `cmd/kb-mcp/main.go` — use `socket.ResolveSocketPath()`
7. Run `go build ./...` and `go test ./...`

## Verification

```sh
go test ./internal/socket/ -v   # all validation and binding tests pass
go test ./...                    # no regressions
go build ./...                   # both binaries build
```

Manual smoke test:

```sh
# start server, verify it creates /tmp/kb-$UID/ with 0700
unset XDG_RUNTIME_DIR && go run ./cmd/kb-server &
ls -ld /tmp/kb-$(id -u)/
curl --unix-socket /tmp/kb-$(id -u)/kb.sock http://localhost/kb.v1.KnowledgeBaseService/List \
  -H 'Content-Type: application/json' -d '{}'
kill %1
```
