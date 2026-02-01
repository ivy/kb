# 004: Seed and Integrate

**Time estimate:** ~1 hour

## Goal

Wire search into kb-server and validate end-to-end with seed data.

## Seed Data

Create a test vault with ~5 dummy notes to validate the pipeline:

```
test-vault/
├── people/
│   ├── taylor-swift.md      # "Taylor handles security and infrastructure..."
│   └── alice-chen.md        # "Alice leads the design team..."
├── meetings/
│   ├── 2026-01-pizza-party.md  # "Discussed process isolation with Taylor..."
│   └── 2026-01-design-sync.md      # "Alice presented new mockups..."
└── projects/
    └── kb-project.md        # "Building semantic search for Obsidian vaults..."
```

**Content:** Simple markdown with clear topics for testing search relevance.

## Server Integration

### Update `internal/server/server.go`

Replace stub in `Query()` handler:

```go
func (s *Server) Query(
    ctx context.Context,
    req *connect.Request[kbv1.QueryRequest],
) (*connect.Response[kbv1.QueryResponse], error) {
    // 1. Map vault names to IDs (or use all if unspecified)
    // 2. Call search.Search(ctx, req.Msg.Question, opts)
    // 3. Format results as answer with citations
    // 4. Return response
}
```

### Response Format

```
Found 2 relevant notes:

1. people/taylor-swift.md (similarity: 0.89)
   "Taylor handles infrastructure for the engineering team..."

2. meetings/2026-01-pizza-party.md (similarity: 0.76)
   "Discussed process isolation with Taylor. Key concerns: resource boundaries..."
```

## Initialization

Add initialization to `cmd/kb-server/main.go`:

```go
func main() {
    // ... existing socket setup ...

    // Initialize database
    dbPath := getDataDir() + "/test-vault-12345678.sqlite"
    db, err := db.Open(dbPath)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }

    // Seed if empty
    if empty, _ := db.IsEmpty(); empty {
        log.Println("Seeding test vault...")
        seedTestVault(db)
    }

    // Create server with dependencies
    srv := server.New(socketPath, db, embed, search)

    // ... existing Run() call ...
}
```

## Validation Flow

### Manual Test

```bash
# Terminal 1: Start server
kb-server
# Output:
# INFO: Seeding test vault...
# INFO: Embedded 5 notes
# INFO: Listening on /tmp/kb-12345/kb.sock

# Terminal 2: Add kb-mcp to Claude Code config

# Terminal 3: Use Claude Code
> Who handles security at Acme Corp?

# Expected output (from kb-server logs):
# INFO: Received query: "Who handles security at Acme Corp?"
# INFO: Found 2 results (top score: 0.89)

# Expected response (in Claude Code):
Found 2 relevant notes:
1. people/taylor-swift.md (similarity: 0.89)
   "Taylor handles security and infrastructure..."
2. meetings/2026-01-pizza-party.md (similarity: 0.76)
   "Discussed process isolation with Taylor..."
```

### Curl Test

```bash
curl --unix-socket /tmp/kb-12345/kb.sock \
  http://localhost/kb.v1.KnowledgeBaseService/Query \
  -H 'Content-Type: application/json' \
  -d '{"question": "Who handles security?"}'
```

## Success Criteria

- [ ] Server initializes database on startup
- [ ] Seed data embeds successfully (~5 notes)
- [ ] Query via kb-mcp returns real search results
- [ ] Results show similarity scores
- [ ] Results include relevant excerpts
- [ ] Can query different topics and get different results
- [ ] Vault filtering works (if multiple vaults)

## Files Modified

- `cmd/kb-server/main.go` - Initialization, seeding
- `internal/server/server.go` - Replace Query stub with search integration
- `internal/server/seed.go` (new) - Test vault seeding logic

## Notes

- **Seed data location:** Could use `testdata/` directory or hardcode strings
- **Temporary vault:** Consider using `t.TempDir()` pattern for seed vault path
- **Idempotency:** Check if vault already seeded to avoid re-embedding on restart
- **Future:** Real vault indexing comes in Phase 3 (indexer package)
