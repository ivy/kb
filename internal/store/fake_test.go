package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ivy/kb/internal/store"
)

// FakeStore implements store.Store with in-memory maps.
type FakeStore struct {
	mu         sync.Mutex
	chunks     map[string]string // hex(hash) -> content
	provenance []store.Provenance
	sources    []store.Source
	err        error // if set, all methods return this error
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
	return 0, nil
}

func (f *FakeStore) UpsertProvenance(_ context.Context, p store.Provenance) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.provenance = append(f.provenance, p)
	return nil
}

func (f *FakeStore) DeleteProvenance(_ context.Context, _ string, _ []byte) error {
	if f.err != nil {
		return f.err
	}
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
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *FakeStore) GetDocument(_ context.Context, _, _ string) ([]store.ProvenanceRow, error) {
	if f.err != nil {
		return nil, f.err
	}
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
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *FakeStore) UpdateSourceIndexed(_ context.Context, _, _ string) error {
	if f.err != nil {
		return f.err
	}
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

func (f *FakeStore) ListFiles(_ context.Context, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

// Compile-time check
var _ store.Store = (*FakeStore)(nil)

// FakeVectorStore implements store.VectorStore with in-memory slices.
type FakeVectorStore struct {
	embeddings []fakeEmbedding
	err        error
	mu         sync.Mutex
}

type fakeEmbedding struct {
	contentHash []byte
	model       string
	vector      []float32
}

func NewFakeVectorStore() *FakeVectorStore {
	return &FakeVectorStore{}
}

func (f *FakeVectorStore) UpsertEmbedding(_ context.Context, contentHash []byte, model string, vector []float32) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.embeddings = append(f.embeddings, fakeEmbedding{contentHash: contentHash, model: model, vector: vector})
	return nil
}

func (f *FakeVectorStore) SearchSimilar(_ context.Context, _ []float32, _ int) ([]store.SearchResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *FakeVectorStore) DeleteEmbeddings(_ context.Context, _ []byte) error {
	if f.err != nil {
		return f.err
	}
	return nil
}

var _ store.VectorStore = (*FakeVectorStore)(nil)
