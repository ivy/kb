# 001: Go Boilerplate — Sliced Work

Parent plan: [../001-go-boilerplate.md](../001-go-boilerplate.md)

## Dependency Graph

```
000-module-init
├──→ 001-config ──────────┐
│                         ├──→ 003-cli-skeleton ──┐
├──→ 002-interfaces ──┬───┘                       │
│                     ├──→ 004-server-handlers ───┤
│                     └──→ 005-test-scaffolding ──┤
│                                                 │
└─────────────────────────────────────────────────┴──→ 006-composition-root
```

## Execution Order

| Phase | Slices | Notes |
|-------|--------|-------|
| **1** | 000-module-init | Foundation — must land first |
| **2** | 001-config, 002-interfaces | Fully parallel |
| **3** | 003-cli-skeleton, 004-server-handlers, 005-test-scaffolding | Fully parallel (003 needs both 001+002; 004 and 005 need only 002) |
| **4** | 006-composition-root | Capstone — wires everything |

Phases 2 and 3 are the parallelization opportunities. With 2-3 agents, phase 2 takes one pass, phase 3 takes one pass, and the whole boilerplate lands in 4 sequential steps.

## Slice Index

| # | Slice | File |
|---|-------|------|
| 000 | Module init | [000-module-init.md](000-module-init.md) |
| 001 | Config types & loading | [001-config.md](001-config.md) |
| 002 | Interfaces & placeholders | [002-interfaces.md](002-interfaces.md) |
| 003 | CLI skeleton | [003-cli-skeleton.md](003-cli-skeleton.md) |
| 004 | Server & handler stubs | [004-server-handlers.md](004-server-handlers.md) |
| 005 | Test scaffolding | [005-test-scaffolding.md](005-test-scaffolding.md) |
| 006 | Composition root | [006-composition-root.md](006-composition-root.md) |
