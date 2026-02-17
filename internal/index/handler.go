package index

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ivy/kb/internal/chunk"
	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/source"
	"github.com/ivy/kb/internal/store"
)

// ReindexJob represents a reindexing request.
type ReindexJob struct {
	SourceURL string `json:"source_url"`
}

// EmbedJob represents an embedding request enqueued after indexing.
type EmbedJob struct {
	ContentHash []byte `json:"content_hash"`
}

// ReindexHandler returns a queue.HandlerFunc that fetches, chunks, and diffs a source.
func ReindexHandler(s store.Store, q queue.Queue, fetch source.Fetcher, cfg config.GitLab) queue.HandlerFunc {
	return func(ctx context.Context, payload []byte) error {
		// 1. Decode payload.
		var job ReindexJob
		if err := json.Unmarshal(payload, &job); err != nil {
			return fmt.Errorf("decoding reindex job: %w", err)
		}

		// 2. Fetch source content.
		result, err := fetch(ctx, job.SourceURL, cfg.Token)
		if err != nil {
			return fmt.Errorf("fetching source %s: %w", job.SourceURL, err)
		}

		// 3. Chunk each file. Process files in sorted order for determinism.
		type fileChunk struct {
			filePath string
			chunk    chunk.Chunk
		}
		var fetched []fileChunk

		paths := sortedKeys(result.Files)
		for _, path := range paths {
			content := result.Files[path]
			chunks, err := chunk.ChunkMarkdown(content)
			if err != nil {
				return fmt.Errorf("chunking %s: %w", path, err)
			}
			for _, c := range chunks {
				fetched = append(fetched, fileChunk{filePath: path, chunk: c})
			}
		}

		// 4. Get existing provenance for this source.
		existing, err := s.ListProvenance(ctx, job.SourceURL)
		if err != nil {
			return fmt.Errorf("listing provenance for %s: %w", job.SourceURL, err)
		}

		// Build set of existing content hashes for this source.
		existingHashes := make(map[string]struct{})
		for _, p := range existing {
			existingHashes[string(p.ContentHash)] = struct{}{}
		}

		// Build set of fetched (filePath, contentHash) pairs for diff.
		type provKey struct {
			filePath    string
			contentHash string
		}
		fetchedSet := make(map[provKey]struct{})
		for _, fc := range fetched {
			key := provKey{filePath: fc.filePath, contentHash: string(fc.chunk.ContentHash)}
			fetchedSet[key] = struct{}{}
		}

		// 5. Delete stale provenance BEFORE upserting, so renames work correctly.
		// DeleteProvenance removes all records for a (sourceURL, contentHash) pair,
		// so it must run before UpsertProvenance creates records at the new path.
		for _, p := range existing {
			key := provKey{filePath: p.FilePath, contentHash: string(p.ContentHash)}
			if _, found := fetchedSet[key]; !found {
				if err := s.DeleteProvenance(ctx, job.SourceURL, p.ContentHash); err != nil {
					return fmt.Errorf("deleting provenance: %w", err)
				}
			}
		}

		// 6. Upsert new/changed chunks and provenance, enqueue embeds for new hashes.
		for _, fc := range fetched {
			if err := s.UpsertChunk(ctx, fc.chunk.ContentHash, fc.chunk.Content); err != nil {
				return fmt.Errorf("upserting chunk: %w", err)
			}

			meta, err := json.Marshal(map[string]any{
				"headings": fc.chunk.HeadingHierarchy,
			})
			if err != nil {
				return fmt.Errorf("marshaling metadata: %w", err)
			}

			if err := s.UpsertProvenance(ctx, store.Provenance{
				SourceURL:   job.SourceURL,
				FilePath:    fc.filePath,
				ContentHash: fc.chunk.ContentHash,
				Position:    fc.chunk.Position,
				Metadata:    meta,
				Ref:         result.Ref,
			}); err != nil {
				return fmt.Errorf("upserting provenance: %w", err)
			}

			// Enqueue embed job only for content hashes not previously indexed.
			if _, existed := existingHashes[string(fc.chunk.ContentHash)]; !existed {
				embedPayload, err := json.Marshal(EmbedJob{ContentHash: fc.chunk.ContentHash})
				if err != nil {
					return fmt.Errorf("marshaling embed job: %w", err)
				}
				if err := q.Enqueue(ctx, queue.Job{Kind: "embed", Payload: embedPayload}); err != nil {
					return fmt.Errorf("enqueuing embed job: %w", err)
				}
			}
		}

		// 8. Clean up orphaned chunks.
		if _, err := s.DeleteOrphanedChunks(ctx); err != nil {
			return fmt.Errorf("cleaning orphaned chunks: %w", err)
		}

		// 9. Update source indexed ref.
		if err := s.UpdateSourceIndexed(ctx, job.SourceURL, result.Ref); err != nil {
			return fmt.Errorf("updating source ref: %w", err)
		}

		return nil
	}
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
