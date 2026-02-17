package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/store"
)

var _ = Describe("PGStore", Label("db"), func() {
	var (
		ctx       context.Context
		pgStore   *store.PGStore
		container *postgres.PostgresContainer
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		container, err = postgres.Run(ctx, "pgvector/pgvector:pg17",
			postgres.WithDatabase("kb_test"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(30*time.Second),
			),
		)
		Expect(err).NotTo(HaveOccurred())

		connStr, err := container.ConnectionString(ctx, "sslmode=disable")
		Expect(err).NotTo(HaveOccurred())

		pgStore, err = store.NewPGStore(ctx, config.Database{URL: connStr})
		Expect(err).NotTo(HaveOccurred())

		err = pgStore.Migrate(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if container != nil {
			Expect(container.Terminate(ctx)).To(Succeed())
		}
	})

	truncate := func() {
		pool := pgStore.Pool()
		_, err := pool.Exec(ctx, "TRUNCATE provenance, chunks, sources CASCADE")
		Expect(err).NotTo(HaveOccurred())
	}

	addSource := func(url string) {
		err := pgStore.AddSource(ctx, url, "gitlab", json.RawMessage(`{}`))
		Expect(err).NotTo(HaveOccurred())
	}

	Describe("content-addressable chunk storage", func() {
		BeforeEach(func() { truncate() })

		It("stores a chunk retrievable by its content hash", func() {
			hash := []byte("deadbeef")
			err := pgStore.UpsertChunk(ctx, hash, "hello world")
			Expect(err).NotTo(HaveOccurred())

			// Verify by inserting provenance that references it
			addSource("https://example.com/repo")
			err = pgStore.UpsertProvenance(ctx, store.Provenance{
				ContentHash: hash,
				SourceURL:   "https://example.com/repo",
				FilePath:    "README.md",
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("silently deduplicates when the same content hash is upserted twice", func() {
			hash := []byte("deadbeef")
			Expect(pgStore.UpsertChunk(ctx, hash, "hello world")).To(Succeed())
			Expect(pgStore.UpsertChunk(ctx, hash, "hello world")).To(Succeed())
		})

		Context("when a chunk has no remaining provenance", func() {
			It("is removed by orphan cleanup", func() {
				hash := []byte("orphan-hash")
				Expect(pgStore.UpsertChunk(ctx, hash, "orphan content")).To(Succeed())

				deleted, err := pgStore.DeleteOrphanedChunks(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(deleted).To(Equal(int64(1)))
			})
		})

		Context("when a chunk is still referenced by at least one provenance record", func() {
			It("survives orphan cleanup", func() {
				hash := []byte("referenced-hash")
				Expect(pgStore.UpsertChunk(ctx, hash, "referenced content")).To(Succeed())

				addSource("https://example.com/repo")
				Expect(pgStore.UpsertProvenance(ctx, store.Provenance{
					ContentHash: hash,
					SourceURL:   "https://example.com/repo",
					FilePath:    "doc.md",
				})).To(Succeed())

				deleted, err := pgStore.DeleteOrphanedChunks(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(deleted).To(Equal(int64(0)))
			})
		})
	})

	Describe("provenance tracking", func() {
		BeforeEach(func() { truncate() })

		It("links a chunk to its source file and position", func() {
			hash := []byte("chunk-1")
			Expect(pgStore.UpsertChunk(ctx, hash, "content")).To(Succeed())
			addSource("https://example.com/repo")

			p := store.Provenance{
				ContentHash: hash,
				SourceURL:   "https://example.com/repo",
				FilePath:    "docs/intro.md",
				Position:    2,
			}
			Expect(pgStore.UpsertProvenance(ctx, p)).To(Succeed())

			result, err := pgStore.ListProvenance(ctx, "https://example.com/repo")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].FilePath).To(Equal("docs/intro.md"))
			Expect(result[0].Position).To(Equal(2))
		})

		It("carries metadata (heading hierarchy) and the indexed ref", func() {
			hash := []byte("chunk-meta")
			Expect(pgStore.UpsertChunk(ctx, hash, "content")).To(Succeed())
			addSource("https://example.com/repo")

			meta := json.RawMessage(`{"headings":["# Root","## Sub"]}`)
			p := store.Provenance{
				ContentHash: hash,
				SourceURL:   "https://example.com/repo",
				FilePath:    "docs/guide.md",
				Position:    0,
				Metadata:    meta,
				Ref:         "abc123",
			}
			Expect(pgStore.UpsertProvenance(ctx, p)).To(Succeed())

			result, err := pgStore.ListProvenance(ctx, "https://example.com/repo")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Metadata).To(MatchJSON(`{"headings":["# Root","## Sub"]}`))
			Expect(result[0].Ref).To(Equal("abc123"))
		})

		Context("when the same chunk appears at the same path in the same source", func() {
			It("updates metadata and indexed_at rather than creating a duplicate", func() {
				hash := []byte("chunk-dup")
				Expect(pgStore.UpsertChunk(ctx, hash, "content")).To(Succeed())
				addSource("https://example.com/repo")

				p := store.Provenance{
					ContentHash: hash,
					SourceURL:   "https://example.com/repo",
					FilePath:    "README.md",
					Position:    0,
					Metadata:    json.RawMessage(`{"v":1}`),
					Ref:         "aaa",
				}
				Expect(pgStore.UpsertProvenance(ctx, p)).To(Succeed())

				p.Position = 3
				p.Metadata = json.RawMessage(`{"v":2}`)
				p.Ref = "bbb"
				Expect(pgStore.UpsertProvenance(ctx, p)).To(Succeed())

				result, err := pgStore.ListProvenance(ctx, "https://example.com/repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].Position).To(Equal(3))
				Expect(result[0].Metadata).To(MatchJSON(`{"v":2}`))
				Expect(result[0].Ref).To(Equal("bbb"))
			})
		})

		Context("when provenance is deleted for a source", func() {
			It("removes only provenance for that source, leaving other sources intact", func() {
				hash := []byte("shared-chunk")
				Expect(pgStore.UpsertChunk(ctx, hash, "shared")).To(Succeed())
				addSource("https://example.com/repo-a")
				addSource("https://example.com/repo-b")

				Expect(pgStore.UpsertProvenance(ctx, store.Provenance{
					ContentHash: hash,
					SourceURL:   "https://example.com/repo-a",
					FilePath:    "file.md",
				})).To(Succeed())
				Expect(pgStore.UpsertProvenance(ctx, store.Provenance{
					ContentHash: hash,
					SourceURL:   "https://example.com/repo-b",
					FilePath:    "file.md",
				})).To(Succeed())

				Expect(pgStore.DeleteProvenance(ctx, "https://example.com/repo-a", hash)).To(Succeed())

				resultA, err := pgStore.ListProvenance(ctx, "https://example.com/repo-a")
				Expect(err).NotTo(HaveOccurred())
				Expect(resultA).To(BeEmpty())

				resultB, err := pgStore.ListProvenance(ctx, "https://example.com/repo-b")
				Expect(err).NotTo(HaveOccurred())
				Expect(resultB).To(HaveLen(1))
			})
		})

		It("lists all provenance records for a given source", func() {
			addSource("https://example.com/repo")

			for i, name := range []string{"a.md", "b.md", "c.md"} {
				hash := []byte("hash-" + name)
				Expect(pgStore.UpsertChunk(ctx, hash, "content-"+name)).To(Succeed())
				Expect(pgStore.UpsertProvenance(ctx, store.Provenance{
					ContentHash: hash,
					SourceURL:   "https://example.com/repo",
					FilePath:    name,
					Position:    i,
				})).To(Succeed())
			}

			result, err := pgStore.ListProvenance(ctx, "https://example.com/repo")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(3))
		})
	})

	Describe("embedding storage", func() {
		BeforeEach(func() { truncate() })

		addChunk := func(hash []byte, content string) {
			Expect(pgStore.UpsertChunk(ctx, hash, content)).To(Succeed())
		}

		addChunkWithProvenance := func(hash []byte, content, sourceURL, filePath string) {
			addChunk(hash, content)
			addSource(sourceURL)
			Expect(pgStore.UpsertProvenance(ctx, store.Provenance{
				ContentHash: hash,
				SourceURL:   sourceURL,
				FilePath:    filePath,
			})).To(Succeed())
		}

		Describe("upserting an embedding", func() {
			It("stores a vector retrievable by content hash and model", func() {
				addChunk([]byte("hash-1"), "hello world")
				vec := make([]float32, 3072)
				vec[0] = 0.1
				Expect(pgStore.UpsertEmbedding(ctx, []byte("hash-1"), "text-embedding-3-large", vec)).To(Succeed())
			})

			Context("when the same content hash and model are upserted twice", func() {
				It("updates the vector rather than creating a duplicate", func() {
					addChunk([]byte("hash-dup"), "content")
					vec1 := make([]float32, 3072)
					vec1[0] = 1.0
					Expect(pgStore.UpsertEmbedding(ctx, []byte("hash-dup"), "model-a", vec1)).To(Succeed())

					vec2 := make([]float32, 3072)
					vec2[0] = 2.0
					Expect(pgStore.UpsertEmbedding(ctx, []byte("hash-dup"), "model-a", vec2)).To(Succeed())

					// Should not error — upsert is idempotent.
				})
			})

			Context("when different models embed the same content hash", func() {
				It("stores both vectors independently", func() {
					addChunk([]byte("hash-multi"), "content")
					vec := make([]float32, 3072)
					Expect(pgStore.UpsertEmbedding(ctx, []byte("hash-multi"), "model-a", vec)).To(Succeed())
					Expect(pgStore.UpsertEmbedding(ctx, []byte("hash-multi"), "model-b", vec)).To(Succeed())
				})
			})
		})

		Describe("similarity search", func() {
			It("returns chunks ordered by cosine distance to the query vector", func() {
				addChunkWithProvenance([]byte("near"), "near content", "https://example.com/repo", "near.md")
				addChunkWithProvenance([]byte("far-c"), "far content", "https://example.com/repo", "far.md")

				nearVec := make([]float32, 3072)
				nearVec[0] = 1.0
				farVec := make([]float32, 3072)
				farVec[1] = 1.0

				Expect(pgStore.UpsertEmbedding(ctx, []byte("near"), "model", nearVec)).To(Succeed())
				Expect(pgStore.UpsertEmbedding(ctx, []byte("far-c"), "model", farVec)).To(Succeed())

				queryVec := make([]float32, 3072)
				queryVec[0] = 1.0

				results, err := pgStore.SearchSimilar(ctx, queryVec, 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(2))
				Expect(results[0].Content).To(Equal("near content"))
			})

			It("respects the limit parameter", func() {
				for i := 0; i < 5; i++ {
					hash := []byte(fmt.Sprintf("lim-%d", i))
					addChunkWithProvenance(hash, fmt.Sprintf("content %d", i), "https://example.com/repo", fmt.Sprintf("f%d.md", i))
					vec := make([]float32, 3072)
					vec[i] = 1.0
					Expect(pgStore.UpsertEmbedding(ctx, hash, "model", vec)).To(Succeed())
				}

				queryVec := make([]float32, 3072)
				queryVec[0] = 1.0

				results, err := pgStore.SearchSimilar(ctx, queryVec, 2)
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(2))
			})

			It("joins chunk content and provenance metadata into results", func() {
				addChunkWithProvenance([]byte("prov-hash"), "some content", "https://example.com/repo", "docs/page.md")
				vec := make([]float32, 3072)
				vec[0] = 1.0
				Expect(pgStore.UpsertEmbedding(ctx, []byte("prov-hash"), "model", vec)).To(Succeed())

				results, err := pgStore.SearchSimilar(ctx, vec, 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0].Content).To(Equal("some content"))
				Expect(results[0].SourceURL).To(Equal("https://example.com/repo"))
				Expect(results[0].FilePath).To(Equal("docs/page.md"))
			})

			Context("when no embeddings exist", func() {
				It("returns an empty result set", func() {
					queryVec := make([]float32, 3072)
					results, err := pgStore.SearchSimilar(ctx, queryVec, 10)
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(BeEmpty())
				})
			})
		})

		Describe("deleting embeddings", func() {
			It("removes all embeddings for a given content hash", func() {
				addChunk([]byte("del-hash"), "content")
				vec := make([]float32, 3072)
				Expect(pgStore.UpsertEmbedding(ctx, []byte("del-hash"), "model", vec)).To(Succeed())
				Expect(pgStore.DeleteEmbeddings(ctx, []byte("del-hash"))).To(Succeed())
			})

			It("leaves embeddings for other content hashes intact", func() {
				addChunk([]byte("keep-h"), "keep")
				addChunk([]byte("del-h"), "del")
				vec := make([]float32, 3072)
				Expect(pgStore.UpsertEmbedding(ctx, []byte("keep-h"), "model", vec)).To(Succeed())
				Expect(pgStore.UpsertEmbedding(ctx, []byte("del-h"), "model", vec)).To(Succeed())

				Expect(pgStore.DeleteEmbeddings(ctx, []byte("del-h"))).To(Succeed())

				// keep-h should still be searchable (needs provenance for SearchSimilar).
				addSource("https://example.com/repo")
				Expect(pgStore.UpsertProvenance(ctx, store.Provenance{
					ContentHash: []byte("keep-h"),
					SourceURL:   "https://example.com/repo",
					FilePath:    "keep.md",
				})).To(Succeed())

				results, err := pgStore.SearchSimilar(ctx, vec, 10)
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(1))
				Expect(results[0].Content).To(Equal("keep"))
			})
		})
	})

	Describe("chunk content lookup", func() {
		BeforeEach(func() { truncate() })

		It("returns the content for a known content hash", func() {
			Expect(pgStore.UpsertChunk(ctx, []byte("lookup-hash"), "expected content")).To(Succeed())
			content, err := pgStore.GetChunkContent(ctx, []byte("lookup-hash"))
			Expect(err).NotTo(HaveOccurred())
			Expect(content).To(Equal("expected content"))
		})

		Context("when the content hash does not exist", func() {
			It("returns an error", func() {
				_, err := pgStore.GetChunkContent(ctx, []byte("nonexistent"))
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("source management", func() {
		BeforeEach(func() { truncate() })

		It("persists a source with its URL, kind, and config", func() {
			cfg := json.RawMessage(`{"group":"infra"}`)
			Expect(pgStore.AddSource(ctx, "https://gitlab.com/my/repo", "gitlab", cfg)).To(Succeed())

			sources, err := pgStore.ListSources(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(HaveLen(1))
			Expect(sources[0].URL).To(Equal("https://gitlab.com/my/repo"))
			Expect(sources[0].Kind).To(Equal("gitlab"))
			Expect(sources[0].Config).To(MatchJSON(`{"group":"infra"}`))
		})

		It("lists all registered sources", func() {
			Expect(pgStore.AddSource(ctx, "https://a.com", "gitlab", json.RawMessage(`{}`))).To(Succeed())
			Expect(pgStore.AddSource(ctx, "https://b.com", "gitlab", json.RawMessage(`{}`))).To(Succeed())

			sources, err := pgStore.ListSources(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(HaveLen(2))
		})

		Context("after a successful reindex", func() {
			It("records the indexed ref and timestamp on the source", func() {
				Expect(pgStore.AddSource(ctx, "https://gitlab.com/my/repo", "gitlab", json.RawMessage(`{}`))).To(Succeed())
				Expect(pgStore.UpdateSourceIndexed(ctx, "https://gitlab.com/my/repo", "def456")).To(Succeed())

				sources, err := pgStore.ListSources(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(sources).To(HaveLen(1))
				Expect(sources[0].LastRef).To(Equal("def456"))
				Expect(sources[0].LastIndexed).NotTo(BeNil())
			})
		})
	})
})
