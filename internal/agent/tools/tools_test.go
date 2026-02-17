package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	agenttools "github.com/ivy/kb/internal/agent/tools"
	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/embed"
	"github.com/ivy/kb/internal/store"
)

var _ = Describe("search", func() {
	var (
		ctx  context.Context
		srv  *httptest.Server
		vs   *fakeVectorStore
		tool *agenttools.SearchTool
	)

	BeforeEach(func() {
		ctx = context.Background()
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
		}))
		vs = &fakeVectorStore{}
		embedder := embed.NewClientWithHTTP(config.Embedding{
			Model:  "text-embedding-3-large",
			APIKey: "test-key",
		}, srv.Client(), srv.URL)
		tool = agenttools.NewSearchTool(embedder, vs)
	})

	AfterEach(func() {
		srv.Close()
	})

	Context("with indexed content about deployment patterns", func() {
		BeforeEach(func() {
			vs.results = []store.SearchResult{
				{
					ContentHash: []byte("hash-1"),
					Content:     "canary deployment content",
					Score:       0.95,
					SourceURL:   "https://gitlab.example.com/grove/atlas",
					FilePath:    "docs/deploy.md",
					Metadata:    json.RawMessage(`{"headings":["# Architecture","## Deployment"]}`),
					IndexedAt:   time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
					Ref:         "abc1234",
				},
			}
		})

		It("returns source URLs, file paths, and heading hierarchy", func() {
			result, err := tool.Call(ctx, "deployment patterns")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("gitlab.example.com/grove/atlas"))
			Expect(result).To(ContainSubstring("docs/deploy.md"))
			Expect(result).To(ContainSubstring("Architecture > Deployment"))
		})

		It("omits chunk content from results", func() {
			result, err := tool.Call(ctx, "deployment patterns")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(ContainSubstring("canary deployment content"))
		})
	})

	Context("when the query has no close matches", func() {
		BeforeEach(func() {
			vs.results = nil
		})

		It("returns a no results message", func() {
			result, err := tool.Call(ctx, "something obscure")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("No results"))
		})
	})
})

var _ = Describe("lookup", func() {
	var (
		ctx  context.Context
		fs   *fakeStore
		tool *agenttools.LookupTool
	)

	BeforeEach(func() {
		ctx = context.Background()
		fs = &fakeStore{}
		tool = agenttools.NewLookupTool(fs)
	})

	Context("with a chunk containing the keyword 'INC-002'", func() {
		BeforeEach(func() {
			fs.lookupResults = []store.SearchResult{
				{
					Content:   "Incident INC-002 occurred...",
					SourceURL: "https://gitlab.example.com/ops/incidents",
					FilePath:  "INC-002.md",
					Metadata:  json.RawMessage(`{"headings":["# INC-002"]}`),
					IndexedAt: time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC),
					Ref:       "def5678",
				},
			}
		})

		It("returns the matching chunk's provenance metadata", func() {
			result, err := tool.Call(ctx, "INC-002")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("ops/incidents"))
			Expect(result).To(ContainSubstring("INC-002.md"))
		})
	})

	Context("when no chunks contain the keyword", func() {
		It("returns a no results message", func() {
			result, err := tool.Call(ctx, "nonexistent")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("No results"))
		})
	})
})

var _ = Describe("read", func() {
	var (
		ctx  context.Context
		fs   *fakeStore
		tool *agenttools.ReadTool
	)

	BeforeEach(func() {
		ctx = context.Background()
		fs = &fakeStore{}
		tool = agenttools.NewReadTool(fs)
	})

	Context("with a 4-chunk document about deployment", func() {
		BeforeEach(func() {
			fs.document = []store.ProvenanceRow{
				{Content: "# Deployment\n\nOverview of deployment."},
				{Content: "## Canary Config\n\nCanary settings here."},
				{Content: "## Promotion\n\nPromotion steps."},
				{Content: "## Rollback\n\nRollback procedures."},
			}
		})

		Context("when called without a position", func() {
			It("returns the heading hierarchy as a table of contents", func() {
				result, err := tool.Call(ctx, "https://example.com/repo docs/deploy.md")
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(ContainSubstring("Table of contents"))
				Expect(result).To(ContainSubstring("Deployment"))
				Expect(result).To(ContainSubstring("Canary Config"))
				Expect(result).To(ContainSubstring("4 chunks"))
			})
		})

		Context("when called with a single position", func() {
			It("returns the chunk content at that position", func() {
				result, err := tool.Call(ctx, "https://example.com/repo docs/deploy.md 1")
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(ContainSubstring("Canary Config"))
				Expect(result).To(ContainSubstring("Canary settings here"))
			})
		})

		Context("when called with a position range like '1-3'", func() {
			It("returns chunk content for positions 1 through 3", func() {
				result, err := tool.Call(ctx, "https://example.com/repo docs/deploy.md 1-3")
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(ContainSubstring("Canary Config"))
				Expect(result).To(ContainSubstring("Promotion"))
				Expect(result).To(ContainSubstring("Rollback"))
			})
		})
	})

	Context("when the source URL and file path don't match any provenance", func() {
		It("returns a document not found message", func() {
			fs.document = nil
			result, err := tool.Call(ctx, "https://example.com/nope nope.md")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("not found"))
		})
	})
})

var _ = Describe("list", func() {
	var (
		ctx  context.Context
		fs   *fakeStore
		tool *agenttools.ListTool
	)

	BeforeEach(func() {
		ctx = context.Background()
		fs = &fakeStore{}
		tool = agenttools.NewListTool(fs)
	})

	Context("when called with no input", func() {
		BeforeEach(func() {
			indexed := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
			fs.sources = []store.Source{
				{URL: "https://gitlab.example.com/grove/atlas", Kind: "gitlab", LastIndexed: &indexed},
				{URL: "https://gitlab.example.com/grove/policy", Kind: "gitlab", LastIndexed: &indexed},
			}
		})

		It("returns all indexed sources with their kind and last-indexed date", func() {
			result, err := tool.Call(ctx, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("grove/atlas"))
			Expect(result).To(ContainSubstring("grove/policy"))
			Expect(result).To(ContainSubstring("gitlab"))
			Expect(result).To(ContainSubstring("2026-02-15"))
		})
	})

	Context("when called with a source URL", func() {
		BeforeEach(func() {
			fs.files = []string{"docs/deploy.md", "docs/architecture.md", "README.md"}
		})

		It("returns the file paths indexed from that source", func() {
			result, err := tool.Call(ctx, "https://gitlab.example.com/grove/atlas")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("docs/deploy.md"))
			Expect(result).To(ContainSubstring("docs/architecture.md"))
			Expect(result).To(ContainSubstring("README.md"))
		})
	})

	Context("when called with an unknown source URL", func() {
		It("returns an empty list message", func() {
			fs.files = nil
			result, err := tool.Call(ctx, "https://example.com/unknown")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("No files"))
		})
	})
})
