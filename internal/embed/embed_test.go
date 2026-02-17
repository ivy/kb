package embed_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/embed"
	"github.com/ivy/kb/internal/store"
)

// fakeStore implements store.Store for embed handler tests.
type fakeStore struct {
	mu     sync.Mutex
	chunks map[string]string
	err    error
}

func newFakeStore() *fakeStore {
	return &fakeStore{chunks: make(map[string]string)}
}

func (f *fakeStore) GetChunkContent(_ context.Context, contentHash []byte) (string, error) {
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

// Stub out remaining Store methods — embed handler only calls GetChunkContent.
func (f *fakeStore) UpsertChunk(context.Context, []byte, string) error                       { return nil }
func (f *fakeStore) DeleteOrphanedChunks(context.Context) (int64, error)                     { return 0, nil }
func (f *fakeStore) UpsertProvenance(context.Context, store.Provenance) error                { return nil }
func (f *fakeStore) DeleteProvenance(context.Context, string, []byte) error                  { return nil }
func (f *fakeStore) ListProvenance(context.Context, string) ([]store.Provenance, error)      { return nil, nil }
func (f *fakeStore) LookupByKeyword(context.Context, string, int) ([]store.SearchResult, error) {
	return nil, nil
}
func (f *fakeStore) GetDocument(context.Context, string, string) ([]store.ProvenanceRow, error) {
	return nil, nil
}
func (f *fakeStore) AddSource(context.Context, string, string, json.RawMessage) error { return nil }
func (f *fakeStore) ListSources(context.Context) ([]store.Source, error)              { return nil, nil }
func (f *fakeStore) RemoveSource(context.Context, string) error                       { return nil }
func (f *fakeStore) UpdateSourceIndexed(context.Context, string, string) error        { return nil }
func (f *fakeStore) ListFiles(context.Context, string) ([]string, error)              { return nil, nil }

var _ store.Store = (*fakeStore)(nil)

// fakeVectorStore captures UpsertEmbedding calls.
type fakeVectorStore struct {
	mu         sync.Mutex
	embeddings []fakeEmbedding
	err        error
}

type fakeEmbedding struct {
	contentHash []byte
	model       string
	vector      []float32
}

func (f *fakeVectorStore) UpsertEmbedding(_ context.Context, contentHash []byte, model string, vector []float32) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.embeddings = append(f.embeddings, fakeEmbedding{contentHash: contentHash, model: model, vector: vector})
	return nil
}

func (f *fakeVectorStore) SearchSimilar(context.Context, []float32, int) ([]store.SearchResult, error) {
	return nil, nil
}

func (f *fakeVectorStore) DeleteEmbeddings(context.Context, []byte) error { return nil }

var _ store.VectorStore = (*fakeVectorStore)(nil)

var _ = Describe("Embed handler", func() {
	var (
		ctx    context.Context
		cs     *fakeStore
		vs     *fakeVectorStore
		cfg    config.Embedding
		srv    *httptest.Server
		hash   []byte
		called bool
	)

	BeforeEach(func() {
		ctx = context.Background()
		cs = newFakeStore()
		vs = &fakeVectorStore{}
		hash = []byte("test-hash")
		called = false

		cfg = config.Embedding{
			Model:  "text-embedding-3-large",
			APIKey: "test-key",
		}

		cs.chunks[string(hash)] = "hello world"
	})

	AfterEach(func() {
		if srv != nil {
			srv.Close()
		}
	})

	setupServer := func(statusCode int, body string) {
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			Expect(r.Header.Get("Authorization")).To(Equal("Bearer test-key"))
			Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

			var reqBody map[string]any
			bodyBytes, _ := io.ReadAll(r.Body)
			Expect(json.Unmarshal(bodyBytes, &reqBody)).To(Succeed())
			Expect(reqBody["model"]).To(Equal("text-embedding-3-large"))
			Expect(reqBody["input"]).To(Equal("hello world"))

			w.WriteHeader(statusCode)
			_, _ = w.Write([]byte(body))
		}))
	}

	makePayload := func() []byte {
		payload, err := json.Marshal(embed.EmbedJob{ContentHash: hash})
		Expect(err).NotTo(HaveOccurred())
		return payload
	}

	Describe("embedding a chunk", func() {
		It("looks up the chunk content by its content hash", func() {
			setupServer(http.StatusOK, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`)
			handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
			Expect(handler(ctx, makePayload())).To(Succeed())
			Expect(called).To(BeTrue())
		})

		It("calls the embedding API with the chunk content and configured model", func() {
			var receivedModel, receivedInput string
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var reqBody map[string]any
				bodyBytes, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(bodyBytes, &reqBody)
				receivedModel, _ = reqBody["model"].(string)
				receivedInput, _ = reqBody["input"].(string)

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
			}))

			handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
			Expect(handler(ctx, makePayload())).To(Succeed())
			Expect(receivedModel).To(Equal("text-embedding-3-large"))
			Expect(receivedInput).To(Equal("hello world"))
		})

		It("persists the returned embedding vector to the vector store", func() {
			setupServer(http.StatusOK, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`)
			handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
			Expect(handler(ctx, makePayload())).To(Succeed())

			vs.mu.Lock()
			defer vs.mu.Unlock()
			Expect(vs.embeddings).To(HaveLen(1))
			Expect(vs.embeddings[0].contentHash).To(Equal(hash))
			Expect(vs.embeddings[0].model).To(Equal("text-embedding-3-large"))
			Expect(vs.embeddings[0].vector).To(Equal([]float32{0.1, 0.2, 0.3}))
		})
	})

	Describe("error handling", func() {
		Context("when the chunk does not exist in the store", func() {
			It("returns an error without calling the embedding API", func() {
				setupServer(http.StatusOK, `{"data":[{"embedding":[0.1]}]}`)
				cs.chunks = make(map[string]string) // empty
				handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
				err := handler(ctx, makePayload())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("chunk"))
				Expect(called).To(BeFalse())
			})
		})

		Context("when the embedding API returns a non-200 status", func() {
			It("returns an error containing the status code", func() {
				setupServer(http.StatusTooManyRequests, `{"error":"rate limited"}`)
				handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
				err := handler(ctx, makePayload())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("429"))
			})
		})

		Context("when the embedding API returns malformed JSON", func() {
			It("returns a decode error", func() {
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`not json`))
				}))
				handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
				err := handler(ctx, makePayload())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("decoding"))
			})
		})

		Context("when the vector store fails to persist", func() {
			It("returns the store error so the job can be retried", func() {
				setupServer(http.StatusOK, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`)
				vs.err = fmt.Errorf("connection refused")
				handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
				err := handler(ctx, makePayload())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("connection refused"))
			})
		})
	})

	Describe("API request format", func() {
		It("sends the Authorization header from config", func() {
			var authHeader string
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHeader = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1]}]}`))
			}))
			handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
			Expect(handler(ctx, makePayload())).To(Succeed())
			Expect(authHeader).To(Equal("Bearer test-key"))
		})

		It("sends a JSON body with model and input fields", func() {
			var reqBody map[string]any
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				bodyBytes, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(bodyBytes, &reqBody)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1]}]}`))
			}))
			handler := embed.EmbedHandlerWithClient(vs, cs, cfg, srv.Client(), srv.URL)
			Expect(handler(ctx, makePayload())).To(Succeed())
			Expect(reqBody).To(HaveKey("model"))
			Expect(reqBody).To(HaveKey("input"))
			Expect(reqBody["model"]).To(Equal("text-embedding-3-large"))
			Expect(reqBody["input"]).To(Equal("hello world"))
		})
	})
})
