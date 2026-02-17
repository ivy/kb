package index_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/store"
)

// FakeStore implements store.Store with in-memory maps for handler tests.
type FakeStore struct {
	mu         sync.Mutex
	chunks     map[string]string // string(hash) → content
	provenance []store.Provenance
	sources    []store.Source
	orphans    int64 // count returned by DeleteOrphanedChunks
	indexedRef string
	err        error
}

func NewFakeStore() *FakeStore {
	return &FakeStore{
		chunks: make(map[string]string),
	}
}

func (f *FakeStore) UpsertChunk(_ context.Context, contentHash []byte, content string) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunks[string(contentHash)] = content
	return nil
}

func (f *FakeStore) DeleteOrphanedChunks(_ context.Context) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	// Find chunks not referenced by any provenance.
	referenced := make(map[string]struct{})
	for _, p := range f.provenance {
		referenced[string(p.ContentHash)] = struct{}{}
	}
	var deleted int64
	for hash := range f.chunks {
		if _, ok := referenced[hash]; !ok {
			delete(f.chunks, hash)
			deleted++
		}
	}
	f.orphans += deleted
	return deleted, nil
}

func (f *FakeStore) UpsertProvenance(_ context.Context, p store.Provenance) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	// Upsert: match on (contentHash, sourceURL, filePath).
	for i, existing := range f.provenance {
		if string(existing.ContentHash) == string(p.ContentHash) &&
			existing.SourceURL == p.SourceURL &&
			existing.FilePath == p.FilePath {
			f.provenance[i] = p
			return nil
		}
	}
	f.provenance = append(f.provenance, p)
	return nil
}

func (f *FakeStore) DeleteProvenance(_ context.Context, sourceURL string, contentHash []byte) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	filtered := f.provenance[:0]
	for _, p := range f.provenance {
		if p.SourceURL != sourceURL || string(p.ContentHash) != string(contentHash) {
			filtered = append(filtered, p)
		}
	}
	f.provenance = filtered
	return nil
}

func (f *FakeStore) ListProvenance(_ context.Context, sourceURL string) ([]store.Provenance, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []store.Provenance
	for _, p := range f.provenance {
		if p.SourceURL == sourceURL {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *FakeStore) LookupByKeyword(_ context.Context, _ string, _ int) ([]store.SearchResult, error) {
	return nil, nil
}

func (f *FakeStore) GetDocument(_ context.Context, _, _ string) ([]store.ProvenanceRow, error) {
	return nil, nil
}

func (f *FakeStore) AddSource(_ context.Context, url, kind string, cfg json.RawMessage) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources = append(f.sources, store.Source{URL: url, Kind: kind, Config: cfg})
	return nil
}

func (f *FakeStore) ListSources(_ context.Context) ([]store.Source, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sources, nil
}

func (f *FakeStore) RemoveSource(_ context.Context, _ string) error {
	return nil
}

func (f *FakeStore) UpdateSourceIndexed(_ context.Context, _ string, ref string) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.indexedRef = ref
	return nil
}

func (f *FakeStore) GetChunkContent(_ context.Context, contentHash []byte) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	content, ok := f.chunks[string(contentHash)]
	if !ok {
		return "", fmt.Errorf("chunk not found")
	}
	return content, nil
}

var _ store.Store = (*FakeStore)(nil)

// FakeQueue implements queue.Queue by collecting enqueued jobs.
type FakeQueue struct {
	mu   sync.Mutex
	jobs []queue.Job
	err  error
}

func NewFakeQueue() *FakeQueue {
	return &FakeQueue{}
}

func (f *FakeQueue) Enqueue(_ context.Context, job queue.Job) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = append(f.jobs, job)
	return nil
}

func (f *FakeQueue) Stats(_ context.Context) (queue.Stats, error) {
	return queue.Stats{}, nil
}

func (f *FakeQueue) Start(_ context.Context) error { return nil }
func (f *FakeQueue) Stop(_ context.Context) error  { return nil }

var _ queue.Queue = (*FakeQueue)(nil)
