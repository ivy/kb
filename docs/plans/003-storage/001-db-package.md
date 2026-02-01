# 001: db Package

**Time estimate:** ~1 hour

## Goal

SQLite + sqlite-vec setup with vault and note storage.

## Database Location

```
${XDG_DATA_HOME:-~/.local/share}/kb/<vault-id>.sqlite
```

## Vault ID Convention

**Format:** `<basename>-<hash8>.sqlite`

Where `hash8` = first 8 chars of SHA256 of absolute vault path.

**Examples:**
```
/Users/ivy/Obsidian/Private  →  Private-a1b2c3d4.sqlite
/Users/ivy/Obsidian/Public   →  Public-e5f6g7h8.sqlite
```

Human-readable, collision-resistant, reversible via lookup.

## Schema

```sql
-- Vault metadata
CREATE TABLE vault (
    id INTEGER PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    last_sync TIMESTAMP
);

-- Notes with embeddings
CREATE TABLE note (
    id INTEGER PRIMARY KEY,
    vault_id INTEGER REFERENCES vault(id),
    path TEXT NOT NULL,           -- Relative to vault root
    mtime INTEGER NOT NULL,       -- For change detection
    title TEXT,
    content TEXT,
    embedding BLOB,               -- sqlite-vec vector
    UNIQUE(vault_id, path)
);

-- Virtual table for vector search
CREATE VIRTUAL TABLE note_vec USING vec0(
    embedding float[1536]         -- OpenAI embedding dimension
);
```

## Go Types

```go
type Vault struct {
    ID       int64
    Path     string
    Name     string
    LastSync time.Time
}

type Note struct {
    ID        int64
    VaultID   int64
    Path      string
    Mtime     time.Time
    Title     string
    Content   string
    Embedding []float32
}
```

## Operations

### Vault Operations
- `CreateVault(path string) (*Vault, error)` - Generate vault ID, create database
- `GetVault(id int64) (*Vault, error)`
- `GetVaultByPath(path string) (*Vault, error)`
- `ListVaults() ([]*Vault, error)`

### Note Operations
- `UpsertNote(note *Note) error` - Insert or update by (vault_id, path)
- `GetNote(vaultID int64, path string) (*Note, error)`
- `ListNotes(vaultID int64) ([]*Note, error)`
- `DeleteNote(vaultID int64, path string) error`

### Vault ID Generation
- `GenerateVaultID(path string) string` - Returns `<basename>-<hash8>`

## Validation

- [ ] Database creates successfully
- [ ] Can insert/query vaults and notes
- [ ] sqlite-vec extension loads
- [ ] Vault ID generation is deterministic and unique

## Dependencies

- `modernc.org/sqlite` - Pure Go SQLite driver
- sqlite-vec extension (CGo dependency - deferred decision on which bindings)

## Notes

- **sqlite-vec bindings**: May use `github.com/asg017/sqlite-vec-go-bindings` or evaluate alternatives
- **Fallback**: If sqlite-vec proves problematic, can swap for simpler solution
- Pure Go SQLite driver (`modernc.org/sqlite`) avoids CGo for the driver itself
