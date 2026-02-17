# 005: Test Scaffolding

**Dependencies**: 002-interfaces
**Blocks**: 006-composition-root (soft — composition root doesn't need tests to exist, but landing tests alongside code is best practice)
**Parallel with**: 001-config, 003-cli-skeleton, 004-server-handlers

Ginkgo suite bootstraps, hand-written fakes for all interfaces, and test fixtures. This slice makes every package test-ready so implementation streams can write specs immediately.

## Deliverables

### Ginkgo suite files

One `*_suite_test.go` per package:

```
internal/config/config_suite_test.go
internal/store/store_suite_test.go
internal/queue/queue_suite_test.go
internal/agent/agent_suite_test.go
internal/index/index_suite_test.go
internal/embed/embed_suite_test.go
internal/session/session_suite_test.go
internal/server/server_suite_test.go
```

Each contains the standard Ginkgo bootstrap:

```go
func TestXxx(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Xxx Suite")
}
```

### Hand-written fakes

Small fakes implementing each interface for use in behavioral specs. No mocking frameworks.

**internal/store/fake_test.go** (or a shared `internal/testutil/` package if needed across packages):

- `FakeStore` implementing `store.Store` — in-memory maps
- `FakeVectorStore` implementing `store.VectorStore` — in-memory slice

**internal/queue/fake_test.go**:

- `FakeQueue` implementing `queue.Queue` — captures enqueued jobs in a slice

**internal/session/fake_test.go**:

- `FakeManager` implementing `session.Manager` — in-memory map

**internal/agent/fake_test.go**:

- `FakeAgent` implementing `agent.Agent` — returns canned responses

### Test fixtures

```
internal/index/testdata/
  simple.md              # single heading, single chunk
  multi-section.md       # multiple H2s → multiple chunks
  oversized-section.md   # exceeds token limit, tests splitting
  frontmatter.md         # YAML frontmatter to skip
  empty.md               # edge case

internal/index/testdata/archives/
  sample.tar.gz          # minimal repo tarball for fetcher specs

internal/server/testdata/
  llms.txt               # expected /llms.txt response body
```

### Placeholder specs

One `_test.go` per package with a single `Describe` block and a pending `It` — proves the suite runs without failures:

```go
var _ = Describe("Chunk storage", func() {
    It("is pending implementation", func() {
        Skip("placeholder — implemented in database stream")
    })
})
```

## Dependencies to add

- `github.com/onsi/ginkgo/v2`
- `github.com/onsi/gomega`

## Acceptance

- `ginkgo ./...` runs all suites, all specs are pending/skipped, zero failures
- `ginkgo --label-filter='!db' ./...` runs without needing PostgreSQL
- Fakes compile and satisfy their interfaces
- Fixture files exist in `testdata/` directories

## Notes

- Fakes are `_test.go` files within each package (not exported) unless multiple packages need the same fake — in that case, consider `internal/testutil/`
- The parent plan says "no mocking frameworks — hand-written fakes that implement the interfaces. They're small (3-12 methods each) and serve as living documentation of expected behavior."
- DB-dependent specs get `Label("db")` on their `Describe` blocks for filtering
