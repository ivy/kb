package tools_test

import (
	"context"
	"encoding/json"

	"github.com/ivy/kb/internal/store"
)

type fakeVectorStore struct {
	results []store.SearchResult
	err     error
}

func (f *fakeVectorStore) UpsertEmbedding(context.Context, []byte, string, []float32) error {
	return nil
}

func (f *fakeVectorStore) SearchSimilar(_ context.Context, _ []float32, _ int) ([]store.SearchResult, error) {
	return f.results, f.err
}

func (f *fakeVectorStore) DeleteEmbeddings(context.Context, []byte) error { return nil }

var _ store.VectorStore = (*fakeVectorStore)(nil)

type fakeStore struct {
	lookupResults []store.SearchResult
	document      []store.ProvenanceRow
	files         []string
	sources       []store.Source
	lookupErr     error
	documentErr   error
	filesErr      error
	sourcesErr    error
}

func (f *fakeStore) LookupByKeyword(_ context.Context, _ string, _ int) ([]store.SearchResult, error) {
	return f.lookupResults, f.lookupErr
}

func (f *fakeStore) GetDocument(_ context.Context, _, _ string) ([]store.ProvenanceRow, error) {
	return f.document, f.documentErr
}

func (f *fakeStore) ListFiles(_ context.Context, _ string) ([]string, error) {
	return f.files, f.filesErr
}

func (f *fakeStore) ListSources(_ context.Context) ([]store.Source, error) {
	return f.sources, f.sourcesErr
}

// Stub remaining Store interface methods.
func (f *fakeStore) UpsertChunk(context.Context, []byte, string) error                        { return nil }
func (f *fakeStore) DeleteOrphanedChunks(context.Context) (int64, error)                      { return 0, nil }
func (f *fakeStore) UpsertProvenance(context.Context, store.Provenance) error                 { return nil }
func (f *fakeStore) DeleteProvenance(context.Context, string, []byte) error                   { return nil }
func (f *fakeStore) ListProvenance(context.Context, string) ([]store.Provenance, error)       { return nil, nil }
func (f *fakeStore) AddSource(context.Context, string, string, json.RawMessage) error         { return nil }
func (f *fakeStore) RemoveSource(context.Context, string) error                               { return nil }
func (f *fakeStore) UpdateSourceIndexed(context.Context, string, string) error                { return nil }
func (f *fakeStore) GetChunkContent(context.Context, []byte) (string, error)                  { return "", nil }

var _ store.Store = (*fakeStore)(nil)
