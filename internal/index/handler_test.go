package index_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/index"
	"github.com/ivy/kb/internal/source"
)

var _ = Describe("Reindex pipeline", func() {
	var (
		ctx     context.Context
		fakeS   *FakeStore
		fakeQ   *FakeQueue
		fetcher source.Fetcher
		cfg     config.GitLab
		handler func(ctx context.Context, payload []byte) error
	)

	const sourceURL = "https://gitlab.com/test/repo"

	makePayload := func() []byte {
		b, err := json.Marshal(index.ReindexJob{SourceURL: sourceURL})
		Expect(err).NotTo(HaveOccurred())
		return b
	}

	Describe("first-time indexing of a source", func() {
		BeforeEach(func() {
			ctx = context.Background()
			fakeS = NewFakeStore()
			fakeQ = NewFakeQueue()
			cfg = config.GitLab{Token: "test-token"}

			fetcher = func(_ context.Context, url, token string) (*source.Result, error) {
				Expect(url).To(Equal(sourceURL))
				Expect(token).To(Equal("test-token"))
				return &source.Result{
					Ref: "abc123",
					Files: map[string][]byte{
						"docs/intro.md": []byte("# Intro\n\nWelcome to the project.\n"),
						"docs/guide.md": []byte("# Guide\n\n## Setup\n\nInstall the tool.\n\n## Usage\n\nRun the tool.\n"),
					},
				}, nil
			}
			handler = index.ReindexHandler(fakeS, fakeQ, fetcher, cfg)
		})

		It("persists every chunk from every Markdown file", func() {
			Expect(handler(ctx, makePayload())).To(Succeed())

			// intro.md → 1 chunk, guide.md → 2 chunks (Setup, Usage)
			Expect(fakeS.chunks).To(HaveLen(3))
		})

		It("creates provenance linking each chunk to its source file and position", func() {
			Expect(handler(ctx, makePayload())).To(Succeed())

			provenance, err := fakeS.ListProvenance(ctx, sourceURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(provenance).To(HaveLen(3))

			// Check that provenance entries have correct source URL and ref.
			for _, p := range provenance {
				Expect(p.SourceURL).To(Equal(sourceURL))
				Expect(p.Ref).To(Equal("abc123"))
			}
		})

		It("enqueues an embed job for every new chunk", func() {
			Expect(handler(ctx, makePayload())).To(Succeed())

			fakeQ.mu.Lock()
			defer fakeQ.mu.Unlock()
			Expect(fakeQ.jobs).To(HaveLen(3))
			for _, j := range fakeQ.jobs {
				Expect(j.Kind).To(Equal("embed"))
			}
		})

		It("records the fetched ref on the source", func() {
			Expect(handler(ctx, makePayload())).To(Succeed())

			fakeS.mu.Lock()
			defer fakeS.mu.Unlock()
			Expect(fakeS.indexedRef).To(Equal("abc123"))
		})
	})

	Describe("re-indexing unchanged content", func() {
		BeforeEach(func() {
			ctx = context.Background()
			fakeS = NewFakeStore()
			fakeQ = NewFakeQueue()
			cfg = config.GitLab{Token: "test-token"}

			fetcher = func(_ context.Context, _, _ string) (*source.Result, error) {
				return &source.Result{
					Ref: "abc123",
					Files: map[string][]byte{
						"README.md": []byte("# Hello\n\nWorld.\n"),
					},
				}, nil
			}
			handler = index.ReindexHandler(fakeS, fakeQ, fetcher, cfg)

			// First indexing run.
			Expect(handler(ctx, makePayload())).To(Succeed())

			// Reset queue to track only second run's jobs.
			fakeQ.mu.Lock()
			fakeQ.jobs = nil
			fakeQ.mu.Unlock()
		})

		It("enqueues zero embed jobs", func() {
			Expect(handler(ctx, makePayload())).To(Succeed())

			fakeQ.mu.Lock()
			defer fakeQ.mu.Unlock()
			Expect(fakeQ.jobs).To(BeEmpty())
		})

		It("refreshes provenance timestamps without creating new records", func() {
			Expect(handler(ctx, makePayload())).To(Succeed())

			provenance, err := fakeS.ListProvenance(ctx, sourceURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(provenance).To(HaveLen(1))
		})
	})

	Describe("re-indexing after content changes", func() {
		Context("when a file has been modified", func() {
			BeforeEach(func() {
				ctx = context.Background()
				fakeS = NewFakeStore()
				fakeQ = NewFakeQueue()
				cfg = config.GitLab{Token: "test-token"}

				callCount := 0
				fetcher = func(_ context.Context, _, _ string) (*source.Result, error) {
					callCount++
					if callCount == 1 {
						return &source.Result{
							Ref: "aaa",
							Files: map[string][]byte{
								"doc.md": []byte("# Doc\n\n## A\n\nOriginal A.\n\n## B\n\nOriginal B.\n"),
							},
						}, nil
					}
					// Second call: section A changed, section B unchanged.
					return &source.Result{
						Ref: "bbb",
						Files: map[string][]byte{
							"doc.md": []byte("# Doc\n\n## A\n\nModified A.\n\n## B\n\nOriginal B.\n"),
						},
					}, nil
				}
				handler = index.ReindexHandler(fakeS, fakeQ, fetcher, cfg)

				// First run.
				Expect(handler(ctx, makePayload())).To(Succeed())
				fakeQ.mu.Lock()
				fakeQ.jobs = nil
				fakeQ.mu.Unlock()
			})

			It("enqueues embed jobs only for chunks whose content hash changed", func() {
				Expect(handler(ctx, makePayload())).To(Succeed())

				fakeQ.mu.Lock()
				defer fakeQ.mu.Unlock()
				// Only the modified section A should be re-embedded.
				Expect(fakeQ.jobs).To(HaveLen(1))
			})
		})

		Context("when a file has been removed", func() {
			BeforeEach(func() {
				ctx = context.Background()
				fakeS = NewFakeStore()
				fakeQ = NewFakeQueue()
				cfg = config.GitLab{Token: "test-token"}

				callCount := 0
				fetcher = func(_ context.Context, _, _ string) (*source.Result, error) {
					callCount++
					if callCount == 1 {
						return &source.Result{
							Ref: "aaa",
							Files: map[string][]byte{
								"keep.md":   []byte("# Keep\n\nStays here.\n"),
								"remove.md": []byte("# Remove\n\nGoing away.\n"),
							},
						}, nil
					}
					return &source.Result{
						Ref: "bbb",
						Files: map[string][]byte{
							"keep.md": []byte("# Keep\n\nStays here.\n"),
						},
					}, nil
				}
				handler = index.ReindexHandler(fakeS, fakeQ, fetcher, cfg)

				// First run.
				Expect(handler(ctx, makePayload())).To(Succeed())
				fakeQ.mu.Lock()
				fakeQ.jobs = nil
				fakeQ.mu.Unlock()
			})

			It("deletes provenance for the removed file's chunks", func() {
				Expect(handler(ctx, makePayload())).To(Succeed())

				provenance, err := fakeS.ListProvenance(ctx, sourceURL)
				Expect(err).NotTo(HaveOccurred())
				// Only keep.md's chunk should remain.
				Expect(provenance).To(HaveLen(1))
				Expect(provenance[0].FilePath).To(Equal("keep.md"))
			})

			It("cleans up chunks that no longer have any provenance", func() {
				Expect(handler(ctx, makePayload())).To(Succeed())

				fakeS.mu.Lock()
				defer fakeS.mu.Unlock()
				// Only keep.md's chunk should remain in the store.
				Expect(fakeS.chunks).To(HaveLen(1))
			})
		})

		Context("when a file has been renamed without content changes", func() {
			It("creates new provenance at the new path without re-embedding", func() {
				ctx = context.Background()
				fakeS = NewFakeStore()
				fakeQ = NewFakeQueue()
				cfg = config.GitLab{Token: "test-token"}

				callCount := 0
				fetcher = func(_ context.Context, _, _ string) (*source.Result, error) {
					callCount++
					content := []byte("# Shared\n\nSame content in both files.\n")
					if callCount == 1 {
						return &source.Result{
							Ref:   "aaa",
							Files: map[string][]byte{"old-name.md": content},
						}, nil
					}
					return &source.Result{
						Ref:   "bbb",
						Files: map[string][]byte{"new-name.md": content},
					}, nil
				}
				handler = index.ReindexHandler(fakeS, fakeQ, fetcher, cfg)

				// First run.
				Expect(handler(ctx, makePayload())).To(Succeed())
				fakeQ.mu.Lock()
				fakeQ.jobs = nil
				fakeQ.mu.Unlock()

				// Second run: renamed file.
				Expect(handler(ctx, makePayload())).To(Succeed())

				// Provenance should point to new path only.
				provenance, err := fakeS.ListProvenance(ctx, sourceURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(provenance).To(HaveLen(1))
				Expect(provenance[0].FilePath).To(Equal("new-name.md"))

				// No new embed jobs — content hash is the same.
				fakeQ.mu.Lock()
				defer fakeQ.mu.Unlock()
				Expect(fakeQ.jobs).To(BeEmpty())
			})
		})
	})
})
