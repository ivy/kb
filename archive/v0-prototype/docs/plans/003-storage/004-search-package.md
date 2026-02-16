# 004: Search + Query RPC

## Goal

Vector search + result formatting. Wire Query RPC to return real ranked results.

## Search Package

Create `internal/search/search.go`:

```go
package search

import (
    "context"
    "fmt"
    "strings"

    "github.com/ivy/kb/internal/db"
    "github.com/ivy/kb/internal/embed"
)

type Searcher struct {
    db     *db.DB
    embed  *embed.Client
}

func New(database *db.DB, embedder *embed.Client) *Searcher {
    return &Searcher{db: database, embed: embedder}
}

type Result struct {
    FilePath   string
    ChunkIndex int
    Content    string
    Score      float32  // Similarity (1 - distance)
}

func (s *Searcher) Search(ctx context.Context, query string, topK int) ([]Result, error) {
    // 1. Embed query
    queryVec, err := s.embed.Embed(ctx, query)
    if err != nil {
        return nil, err
    }

    // 2. KNN search
    chunks, err := s.db.SearchSimilar(queryVec, topK)
    if err != nil {
        return nil, err
    }

    // 3. Convert to results (distance → similarity)
    results := make([]Result, len(chunks))
    for i, chunk := range chunks {
        results[i] = Result{
            FilePath:   chunk.FilePath,
            ChunkIndex: chunk.ChunkIndex,
            Content:    chunk.Content,
            Score:      1.0 - chunk.Distance,  // Cosine distance → similarity
        }
    }

    return results, nil
}

// FormatResults creates human-readable output
func FormatResults(results []Result) (answer string, citations []string) {
    var buf strings.Builder

    if len(results) == 0 {
        return "No relevant results found.", nil
    }

    fmt.Fprintf(&buf, "Found %d relevant chunks:\n\n", len(results))

    seen := make(map[string]bool)
    for i, r := range results {
        // Format result
        excerpt := truncate(r.Content, 200)
        fmt.Fprintf(&buf, "%d. %s (chunk %d, score: %.2f)\n",
            i+1, r.FilePath, r.ChunkIndex, r.Score)
        fmt.Fprintf(&buf, "   %q\n\n", excerpt)

        // Collect unique citations
        if !seen[r.FilePath] {
            citations = append(citations, r.FilePath)
            seen[r.FilePath] = true
        }
    }

    return buf.String(), citations
}

func truncate(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max] + "..."
}
```

## Wire to Server

Update `internal/server/server.go`:

```go
type Server struct {
    socketPath string
    httpServer *http.Server
    searcher   *search.Searcher  // Add this
}

func New(socketPath string, searcher *search.Searcher) *Server {
    return &Server{
        socketPath: socketPath,
        searcher:   searcher,
    }
}

func (s *Server) Query(
    ctx context.Context,
    req *connect.Request[kbv1.QueryRequest],
) (*connect.Response[kbv1.QueryResponse], error) {
    log.Printf("query: %q", req.Msg.Question)

    // Execute search
    results, err := s.searcher.Search(ctx, req.Msg.Question, 10)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // Format response
    answer, citations := search.FormatResults(results)

    resp := &kbv1.QueryResponse{
        Answer:    answer,
        Citations: citations,
    }

    return connect.NewResponse(resp), nil
}
```

Update `cmd/kb-server/main.go`:

```go
func main() {
    // ... socket setup ...

    // Initialize components
    dbPath := filepath.Join(os.Getenv("HOME"), ".local/share/kb/testdata.db")
    database, err := db.Open(dbPath)
    if err != nil {
        log.Fatal(err)
    }

    embedder := embed.New(os.Getenv("OPENAI_API_KEY"))
    searcher := search.New(database, embedder)

    // Create server
    srv := server.New(socketPath, searcher)

    // ... run server ...
}
```

## Update Seed Script

Modify `cmd/seed-testdata/main.go` to embed chunks with real embeddings:

```go
import "github.com/ivy/kb/internal/embed"

func main() {
    // ... existing setup ...

    // Create embedder
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        log.Fatal("OPENAI_API_KEY required")
    }
    embedder := embed.New(apiKey)

    // ... walk files ...

        // Chunk file
        chunks, _ := chunker.Chunk(string(content))

        // Embed and insert each chunk
        for _, chunk := range chunks {
            // Embed chunk
            embedding, err := embedder.Embed(context.Background(), chunk.Content)
            if err != nil {
                log.Printf("Failed to embed %s chunk %d: %v", relPath, chunk.Index, err)
                continue
            }

            // Insert with real embedding
            err = database.InsertChunk(vaultID, relPath, chunk.Index, chunk.Content, chunk.TokenCount, embedding)
            if err != nil {
                return err
            }
        }
    // ...
}
```

**Run once with API key:**
```bash
OPENAI_API_KEY=sk-... go run cmd/seed-testdata/main.go
```

This replaces zero embeddings with real ones.

## Validation

After this step, full end-to-end search works:

```bash
# Start server
kb-server

# Query
curl --unix-socket /tmp/kb.sock \
  http://localhost/kb.v1.KnowledgeBaseService/Query \
  -H 'Content-Type: application/json' \
  -d '{"question": "Who handles security?"}'
```

Expected output:
```
Found 2 relevant chunks:

1. people/alice.md (chunk 0, score: 0.89)
   "Alice handles security and infrastructure..."

2. meetings/2026-01.md (chunk 1, score: 0.76)
   "Discussed security concerns with Alice..."

Citations:
- people/alice.md
- meetings/2026-01.md
```

## Validation Checklist

- [ ] Search package implemented
- [ ] Server wired to use searcher
- [ ] Seed script embeds chunks with real embeddings
- [ ] curl query returns ranked results
- [ ] Similarity scores in 0.0-1.0 range
- [ ] Citations list unique file paths
- [ ] **Phase 2 complete - vector search works end-to-end**

## Time Estimate

2-3 hours

## Notes

- Each chunk is a separate result (no grouping by file)
- Simple truncation for excerpts (first 200 chars)
- Distance → similarity conversion: `1.0 - distance`
- sqlite-vec returns cosine distance (0 = identical, 2 = opposite)
