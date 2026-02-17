package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/index"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/source"
	"github.com/ivy/kb/internal/store"
)

// buildArchive creates a tar.gz byte slice with the given files under a root
// directory prefix, optionally including a PAX global header (as GitLab does).
func buildArchive(root string, files map[string][]byte, withPAX bool) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	if withPAX {
		// Write a PAX global header like GitLab archives include.
		_ = tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeXGlobalHeader,
			Name:     "pax_global_header",
			Size:     0,
		})
	}

	// Write the root directory entry.
	_ = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     root + "/",
		Mode:     0o755,
	})

	for name, content := range files {
		_ = tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     root + "/" + name,
			Size:     int64(len(content)),
			Mode:     0o644,
		})
		_, _ = tw.Write(content)
	}

	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

// fakeQueue collects enqueued jobs for assertion. Mirrors FakeQueue from index tests.
type fakeQueue struct {
	mu   sync.Mutex
	jobs []queue.Job
}

func (f *fakeQueue) Enqueue(_ context.Context, job queue.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = append(f.jobs, job)
	return nil
}

func (f *fakeQueue) Stats(_ context.Context) (queue.Stats, error) { return queue.Stats{}, nil }
func (f *fakeQueue) Start(_ context.Context) error                { return nil }
func (f *fakeQueue) Stop(_ context.Context) error                 { return nil }

var _ queue.Queue = (*fakeQueue)(nil)

var _ = Describe("Indexing pipeline", Label("integration"), func() {
	var (
		ctx       context.Context
		container *postgres.PostgresContainer
		pgStore   *store.PGStore
		fq        *fakeQueue
	)

	const sourceURL = "https://gitlab.example.com/test/repo"

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
		Expect(pgStore.Migrate(ctx)).To(Succeed())

		fq = &fakeQueue{}
	})

	AfterEach(func() {
		if container != nil {
			Expect(container.Terminate(ctx)).To(Succeed())
		}
	})

	truncate := func() {
		_, err := pgStore.Pool().Exec(ctx, "TRUNCATE provenance, chunks, sources CASCADE")
		Expect(err).NotTo(HaveOccurred())
	}

	// runHandler sets up an httptest server returning the given archive,
	// creates a fetcher that rewrites URLs to the test server, and invokes
	// the ReindexHandler.
	runHandler := func(archive []byte) error {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Header.Get("PRIVATE-TOKEN")).To(Equal("test-token"))
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(archive)
		}))
		defer srv.Close()

		// Build a fetcher that calls FetchGitLab but rewrites the base URL
		// to point at the httptest server.
		fetcher := func(ctx context.Context, srcURL, token string) (*source.Result, error) {
			return source.FetchGitLab(ctx, srv.URL+"/test/repo", token)
		}

		cfg := config.GitLab{Token: "test-token"}
		handler := index.ReindexHandler(pgStore, fq, fetcher, cfg)

		payload, err := json.Marshal(index.ReindexJob{SourceURL: sourceURL})
		Expect(err).NotTo(HaveOccurred())
		return handler(ctx, payload)
	}

	Describe("indexing a source for the first time", func() {
		var archive []byte

		BeforeEach(func() {
			truncate()
			Expect(pgStore.AddSource(ctx, sourceURL, "gitlab", json.RawMessage(`{}`))).To(Succeed())

			archive = buildArchive("repo-main-abc123", map[string][]byte{
				"docs/intro.md": []byte("# Intro\n\nWelcome to the project.\n"),
				"docs/guide.md": []byte("# Guide\n\n## Setup\n\nInstall the tool.\n\n## Usage\n\nRun the tool.\n"),
				"src/main.go":   []byte("package main\n"), // non-Markdown, should be skipped
			}, true) // include PAX header
		})

		It("fetches the archive, chunks Markdown files, and persists chunks to the database", func() {
			Expect(runHandler(archive)).To(Succeed())

			// intro.md → 1 chunk, guide.md → 2 chunks (Setup, Usage)
			provenance, err := pgStore.ListProvenance(ctx, sourceURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(provenance).To(HaveLen(3))
		})

		It("creates provenance records linking chunks to their source files", func() {
			Expect(runHandler(archive)).To(Succeed())

			provenance, err := pgStore.ListProvenance(ctx, sourceURL)
			Expect(err).NotTo(HaveOccurred())

			filePaths := make(map[string]bool)
			for _, p := range provenance {
				Expect(p.SourceURL).To(Equal(sourceURL))
				filePaths[p.FilePath] = true
			}
			Expect(filePaths).To(HaveKey("docs/intro.md"))
			Expect(filePaths).To(HaveKey("docs/guide.md"))
		})

		It("records the commit ref on the source", func() {
			Expect(runHandler(archive)).To(Succeed())

			sources, err := pgStore.ListSources(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(sources).To(HaveLen(1))
			Expect(sources[0].LastRef).To(Equal("abc123"))
		})

		It("enqueues embed jobs for each unique chunk", func() {
			Expect(runHandler(archive)).To(Succeed())

			fq.mu.Lock()
			defer fq.mu.Unlock()
			Expect(fq.jobs).To(HaveLen(3))
			for _, j := range fq.jobs {
				Expect(j.Kind).To(Equal("embed"))
			}
		})
	})

	Describe("handling real-world tar archives", func() {
		BeforeEach(func() {
			truncate()
			Expect(pgStore.AddSource(ctx, sourceURL, "gitlab", json.RawMessage(`{}`))).To(Succeed())
		})

		It("skips PAX global headers without corrupting paths or refs", func() {
			archive := buildArchive("repo-main-def456", map[string][]byte{
				"README.md": []byte("# README\n\nHello.\n"),
			}, true)

			Expect(runHandler(archive)).To(Succeed())

			provenance, err := pgStore.ListProvenance(ctx, sourceURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(provenance).To(HaveLen(1))
			Expect(provenance[0].FilePath).To(Equal("README.md"))

			sources, err := pgStore.ListSources(ctx)
			Expect(err).NotTo(HaveOccurred())
			// Ref should come from the directory name, not the PAX header.
			Expect(sources[0].LastRef).NotTo(Equal("pax_global_header"))
		})

		It("strips the archive root prefix from file paths", func() {
			archive := buildArchive("my-project-v1.0.0-abc123def", map[string][]byte{
				"docs/setup.md": []byte("# Setup\n\nSteps.\n"),
			}, false)

			Expect(runHandler(archive)).To(Succeed())

			provenance, err := pgStore.ListProvenance(ctx, sourceURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(provenance).To(HaveLen(1))
			// Path should be relative, not include the archive root.
			Expect(provenance[0].FilePath).To(Equal("docs/setup.md"))
		})
	})

	Describe("re-indexing a source", func() {
		Context("when content has not changed", func() {
			It("does not enqueue duplicate embed jobs", func() {
				truncate()
				Expect(pgStore.AddSource(ctx, sourceURL, "gitlab", json.RawMessage(`{}`))).To(Succeed())

				archive := buildArchive("repo-main-aaa", map[string][]byte{
					"README.md": []byte("# Hello\n\nWorld.\n"),
				}, true)

				// First index.
				Expect(runHandler(archive)).To(Succeed())

				// Reset queue.
				fq.mu.Lock()
				fq.jobs = nil
				fq.mu.Unlock()

				// Re-index with same content.
				Expect(runHandler(archive)).To(Succeed())

				fq.mu.Lock()
				defer fq.mu.Unlock()
				Expect(fq.jobs).To(BeEmpty())
			})
		})

		Context("when a file has been modified", func() {
			It("persists only the changed chunks and enqueues embeds for them", func() {
				truncate()
				Expect(pgStore.AddSource(ctx, sourceURL, "gitlab", json.RawMessage(`{}`))).To(Succeed())

				archiveV1 := buildArchive("repo-main-aaa", map[string][]byte{
					"doc.md": []byte("# Doc\n\n## A\n\nOriginal A.\n\n## B\n\nOriginal B.\n"),
				}, true)
				Expect(runHandler(archiveV1)).To(Succeed())

				fq.mu.Lock()
				fq.jobs = nil
				fq.mu.Unlock()

				// Section A changed, section B unchanged.
				archiveV2 := buildArchive("repo-main-bbb", map[string][]byte{
					"doc.md": []byte("# Doc\n\n## A\n\nModified A.\n\n## B\n\nOriginal B.\n"),
				}, true)
				Expect(runHandler(archiveV2)).To(Succeed())

				fq.mu.Lock()
				defer fq.mu.Unlock()
				// Only the modified section A should be re-embedded.
				Expect(fq.jobs).To(HaveLen(1))
			})
		})

		Context("when a file has been removed", func() {
			It("deletes its provenance and cleans up orphaned chunks", func() {
				truncate()
				Expect(pgStore.AddSource(ctx, sourceURL, "gitlab", json.RawMessage(`{}`))).To(Succeed())

				archiveV1 := buildArchive("repo-main-aaa", map[string][]byte{
					"keep.md":   []byte("# Keep\n\nStays here.\n"),
					"remove.md": []byte("# Remove\n\nGoing away.\n"),
				}, true)
				Expect(runHandler(archiveV1)).To(Succeed())

				// Verify both files were indexed.
				prov, err := pgStore.ListProvenance(ctx, sourceURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(prov).To(HaveLen(2))

				// Re-index without remove.md.
				archiveV2 := buildArchive("repo-main-bbb", map[string][]byte{
					"keep.md": []byte("# Keep\n\nStays here.\n"),
				}, true)
				Expect(runHandler(archiveV2)).To(Succeed())

				// Only keep.md's provenance should remain.
				prov, err = pgStore.ListProvenance(ctx, sourceURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(prov).To(HaveLen(1))
				Expect(prov[0].FilePath).To(Equal("keep.md"))
			})
		})
	})
})
