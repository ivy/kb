# 002: Database + Manual Seed

## Goal

SQLite + sqlite-vec storage for chunks. Manual seeding script for test vault.

## Simplified Schema

```sql
-- Vault metadata (one row for now)
CREATE TABLE vault (
    id INTEGER PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL
);

-- Chunks only (no note table - files stay on disk)
CREATE TABLE chunk (
    id INTEGER PRIMARY KEY,
    vault_id INTEGER NOT NULL REFERENCES vault(id),
    file_path TEXT NOT NULL,        -- "people/alice.md"
    chunk_index INTEGER NOT NULL,   -- 0-based position within file
    content TEXT NOT NULL,           -- Heading path + chunk text
    token_count INTEGER NOT NULL,
    UNIQUE(vault_id, file_path, chunk_index)
);

-- Vector embeddings (rowid maps to chunk.id)
CREATE VIRTUAL TABLE chunk_vec USING vec0(
    embedding float[3072]
);
```

**Key points:**
- No `note` table - markdown files stay on disk
- `file_path` in chunk table links back to source file
- `chunk_vec` is a virtual table for vector search

## Database Package

Create `internal/db/db.go`:

```go
package db

import (
    "database/sql"
    _ "github.com/ncruces/go-sqlite3/embed"
    sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

type DB struct {
    db *sql.DB
}

func Open(path string) (*DB, error) {
    // Auto-register sqlite-vec extension
    sqlite_vec.Auto()

    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }

    // Create schema if not exists
    if err := createSchema(db); err != nil {
        db.Close()
        return nil, err
    }

    return &DB{db: db}, nil
}

func createSchema(db *sql.DB) error {
    schema := `
    CREATE TABLE IF NOT EXISTS vault (
        id INTEGER PRIMARY KEY,
        path TEXT UNIQUE NOT NULL,
        name TEXT NOT NULL
    );

    CREATE TABLE IF NOT EXISTS chunk (
        id INTEGER PRIMARY KEY,
        vault_id INTEGER NOT NULL REFERENCES vault(id),
        file_path TEXT NOT NULL,
        chunk_index INTEGER NOT NULL,
        content TEXT NOT NULL,
        token_count INTEGER NOT NULL,
        UNIQUE(vault_id, file_path, chunk_index)
    );

    CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vec USING vec0(
        embedding float[3072]
    );
    `
    _, err := db.Exec(schema)
    return err
}

// EnsureVault creates or returns existing vault
func (d *DB) EnsureVault(path, name string) (int64, error) {
    // INSERT OR IGNORE then SELECT
}

// InsertChunk stores chunk + embedding
func (d *DB) InsertChunk(vaultID int64, filePath string, index int, content string, tokens int, embedding []float32) error {
    tx, _ := d.db.Begin()
    defer tx.Rollback()

    // Insert chunk metadata
    result, _ := tx.Exec(
        "INSERT INTO chunk (vault_id, file_path, chunk_index, content, token_count) VALUES (?, ?, ?, ?, ?)",
        vaultID, filePath, index, content, tokens,
    )
    chunkID, _ := result.LastInsertId()

    // Serialize and insert embedding
    vecBlob, _ := sqlite_vec.SerializeFloat32(embedding)
    _, _ = tx.Exec(
        "INSERT INTO chunk_vec (rowid, embedding) VALUES (?, ?)",
        chunkID, vecBlob,
    )

    return tx.Commit()
}

type ChunkResult struct {
    FilePath   string
    ChunkIndex int
    Content    string
    Distance   float32
}

// SearchSimilar performs KNN search
func (d *DB) SearchSimilar(queryEmbedding []float32, limit int) ([]ChunkResult, error) {
    vecBlob, _ := sqlite_vec.SerializeFloat32(queryEmbedding)

    query := `
        SELECT c.file_path, c.chunk_index, c.content, cv.distance
        FROM chunk_vec cv
        JOIN chunk c ON c.id = cv.rowid
        WHERE cv.embedding MATCH ?
        ORDER BY cv.distance ASC
        LIMIT ?
    `

    rows, _ := d.db.Query(query, vecBlob, limit)
    defer rows.Close()

    var results []ChunkResult
    for rows.Next() {
        var r ChunkResult
        rows.Scan(&r.FilePath, &r.ChunkIndex, &r.Content, &r.Distance)
        results = append(results, r)
    }

    return results, nil
}
```

**Dependencies:**
- `github.com/ncruces/go-sqlite3` - pure Go SQLite
- `github.com/asg017/sqlite-vec-go-bindings/ncruces` - sqlite-vec bindings

## Manual Seeding Script

Create `cmd/seed-testdata/main.go`:

```go
package main

import (
    "log"
    "os"
    "path/filepath"

    "github.com/ivy/kb/internal/chunk"
    "github.com/ivy/kb/internal/db"
)

func main() {
    // Open database
    dbPath := filepath.Join(os.Getenv("HOME"), ".local/share/kb/testdata.db")
    os.MkdirAll(filepath.Dir(dbPath), 0755)

    database, err := db.Open(dbPath)
    if err != nil {
        log.Fatal(err)
    }

    // Create vault
    vaultPath := "testdata/vault"
    vaultID, err := database.EnsureVault(vaultPath, "testdata")
    if err != nil {
        log.Fatal(err)
    }

    // Create chunker
    chunker, err := chunk.New()
    if err != nil {
        log.Fatal(err)
    }

    // Walk vault and chunk files
    filepath.Walk(vaultPath, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() || filepath.Ext(path) != ".md" {
            return nil
        }

        // Read file
        content, err := os.ReadFile(path)
        if err != nil {
            return err
        }

        // Chunk it
        chunks, err := chunker.Chunk(string(content))
        if err != nil {
            return err
        }

        // Get relative path
        relPath, _ := filepath.Rel(vaultPath, path)

        // Insert chunks (with zero embeddings for now)
        for _, chunk := range chunks {
            zeroEmbedding := make([]float32, 3072)
            err = database.InsertChunk(vaultID, relPath, chunk.Index, chunk.Content, chunk.TokenCount, zeroEmbedding)
            if err != nil {
                return err
            }
        }

        log.Printf("Indexed %s (%d chunks)", relPath, len(chunks))
        return nil
    })

    log.Println("Seeding complete")
}
```

**Usage:**
```bash
go run cmd/seed-testdata/main.go
```

This populates the DB with chunks (zero embeddings). Step 004 will update with real embeddings.

## Validation

- [ ] Database creates at `~/.local/share/kb/testdata.db`
- [ ] Schema exists (vault, chunk, chunk_vec tables)
- [ ] Seed script runs without errors
- [ ] Test vault chunks inserted (check with sqlite3 CLI)
- [ ] Zero embeddings stored (OK for now)

## Notes

- **No migrations framework** - just CREATE TABLE IF NOT EXISTS
- **No mtime tracking** - re-seed manually if test vault changes
- **No error handling** - fail fast for now
- **rowid critical** - chunk_vec.rowid must match chunk.id for JOIN

## Time Estimate

2-3 hours
