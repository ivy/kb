# 003: search Package

**Time estimate:** ~45 minutes

## Goal

Vector similarity search combining `embed` and `db` packages.

## Interface

```go
type SearchOpts struct {
    VaultIDs []int64  // Filter to specific vaults (empty = all)
    TopK     int      // Number of results (default: 10)
}

type Result struct {
    Note       *db.Note
    Score      float32  // Similarity score (0.0 - 1.0)
    Excerpt    string   // Relevant excerpt from content
}

func Search(ctx context.Context, query string, opts SearchOpts) ([]Result, error)
```

## Implementation Flow

1. **Embed query** via `embed.Embed(query)`
2. **Vector search** via sqlite-vec similarity query
3. **Fetch notes** with top-k results
4. **Extract excerpts** from matched content
5. **Return ranked results** ordered by score

## SQL Query Pattern

```sql
SELECT
    n.id, n.vault_id, n.path, n.title, n.content,
    vec_distance_cosine(nv.embedding, ?) as distance
FROM note n
JOIN note_vec nv ON n.id = nv.rowid
WHERE (:vault_filter OR n.vault_id IN (?))
ORDER BY distance ASC
LIMIT ?
```

## Excerpt Generation

Extract ~200 character window around best match:
- Simple: First 200 chars of content
- Better: Find query terms, extract surrounding context

## Validation

- [ ] Returns relevant results for test queries
- [ ] Scores are reasonable (0.0 - 1.0 range)
- [ ] Vault filtering works
- [ ] Top-k limiting works
- [ ] Results ordered by similarity

## Testing Strategy

- Create test vault with known notes
- Embed and insert test data
- Query with known terms
- Verify correct notes returned
- Check score ordering

## Dependencies

- `internal/db` - Database access
- `internal/embed` - Query embedding

## Notes

- **Similarity metric:** Cosine distance (sqlite-vec default)
- **Score normalization:** May need to convert distance to similarity (1 - distance)
- **Excerpt quality:** Start simple, improve iteratively
- **Hybrid search:** Future enhancement (combine with keyword/recency)
