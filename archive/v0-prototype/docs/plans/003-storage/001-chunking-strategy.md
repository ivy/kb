# 001: Chunking + Test Vault

## Goal

Split markdown into embeddable chunks with heading context. Create test vault with seed data.

## Chunking Parameters

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Max tokens | 4096 | Half of model's 8191 limit for better granularity |
| Overlap | 150 tokens | ~2-3 sentences for continuity |
| Encoding | cl100k_base | OpenAI's tokenizer for text-embedding-3-large |

## Algorithm

1. **Parse heading hierarchy** (H1-H6)
2. **Split at heading boundaries** when possible
3. **Add heading path** to each chunk: `# Meeting Notes > ## 2026 > ### January`
4. **Split mid-section** if section exceeds 4096 tokens (at paragraph breaks)
5. **Add 150-token overlap** between consecutive chunks

## Chunk Output

```go
type ChunkData struct {
    Index      int    // 0-based position within file
    Content    string // Full heading path + overlap + chunk text
    TokenCount int    // Exact cl100k_base count
}
```

## Test Vault

Create `testdata/vault/` with 5 markdown files:

```
testdata/vault/
├── people/
│   ├── alice.md      # "Alice handles security..."
│   └── taylor.md     # "Taylor leads design..."
├── meetings/
│   └── 2026-01.md    # "Discussed security with Alice..."
└── projects/
    └── kb.md         # "Building semantic search..."
```

**Content guidelines:**
- 200-800 tokens per file
- Clear topics for search testing (people, projects, dates)
- Include headings to test heading path extraction
- Keep it simple - this is test data

## Implementation

```go
package chunk

import "github.com/pkoukk/tiktoken-go"

type Chunker struct {
    maxTokens     int    // 4096
    overlapTokens int    // 150
    encoding      *tiktoken.Tiktoken
}

func New() (*Chunker, error) {
    enc, err := tiktoken.GetEncoding("cl100k_base")
    if err != nil {
        return nil, err
    }
    return &Chunker{
        maxTokens:     4096,
        overlapTokens: 150,
        encoding:      enc,
    }, nil
}

func (c *Chunker) Chunk(markdown string) ([]ChunkData, error) {
    // 1. Parse headings
    // 2. Split at heading boundaries
    // 3. Add overlap
    // 4. Count tokens
    // 5. Return chunks
}
```

**Dependencies:** `github.com/pkoukk/tiktoken-go`

## Edge Cases (Keep Simple)

- **Empty file** - return no chunks
- **File under 4096 tokens** - return single chunk
- **Very large section** - split at paragraph breaks, preserve heading context
- **Skip frontmatter** - YAML between `---` delimiters

Don't handle every edge case. Handle the common cases well.

## Validation

- [ ] Chunks respect 4096 token max
- [ ] Overlap works (last 150 tokens of chunk N appear in chunk N+1)
- [ ] Heading paths included: `# Parent > ## Child`
- [ ] Token counts match OpenAI's cl100k_base
- [ ] Test vault has 5 markdown files
- [ ] Unit tests cover basic functionality

## Time Estimate

1-2 hours
