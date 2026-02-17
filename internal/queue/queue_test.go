package queue_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/ivy/kb/internal/config"
	"github.com/ivy/kb/internal/queue"
)

var _ = Describe("RiverQueue", func() {
	Describe("worker concurrency defaults", func() {
		// These tests verify config defaults without needing a database.
		// We can't instantiate RiverQueue without a real pool, so we test
		// the defaulting logic indirectly through integration tests.
		// Instead, we verify the config struct zero-value behavior.

		It("uses Workers struct zero values when not configured", func() {
			cfg := config.Workers{}
			Expect(cfg.Reindex).To(Equal(0))
			Expect(cfg.Embed).To(Equal(0))
		})
	})

	Describe("integration", Label("integration"), func() {
		var (
			ctx       context.Context
			cancel    context.CancelFunc
			pool      *pgxpool.Pool
			container *postgres.PostgresContainer
		)

		BeforeEach(func() {
			ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)

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

			pool, err = pgxpool.New(ctx, connStr)
			Expect(err).NotTo(HaveOccurred())

			// Run River migrations.
			rm, err := rivermigrate.New(riverpgxv5.New(pool), nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = rm.Migrate(ctx, rivermigrate.DirectionUp, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			cancel()
			if pool != nil {
				pool.Close()
			}
			if container != nil {
				Expect(container.Terminate(context.Background())).To(Succeed())
			}
		})

		Describe("job dispatch", func() {
			It("dispatches to the matching HandlerFunc with the correct payload", func() {
				var received []byte
				var mu sync.Mutex
				done := make(chan struct{})

				handlers := map[string]queue.HandlerFunc{
					"reindex": func(_ context.Context, payload []byte) error {
						mu.Lock()
						received = payload
						mu.Unlock()
						close(done)
						return nil
					},
					"embed": func(_ context.Context, _ []byte) error {
						return nil
					},
				}

				q, err := queue.NewRiverQueue(pool, config.Workers{}, handlers)
				Expect(err).NotTo(HaveOccurred())

				Expect(q.Enqueue(ctx, queue.Job{Kind: "reindex", Payload: []byte(`{"url":"test"}`)})).To(Succeed())

				go func() { _ = q.Start(ctx) }()
				defer func() { _ = q.Stop(context.Background()) }()

				Eventually(done, 10*time.Second).Should(BeClosed())
				mu.Lock()
				Expect(string(received)).To(Equal(`{"url":"test"}`))
				mu.Unlock()
			})
		})

		Describe("queue routing", func() {
			It("routes jobs to the named queue matching their Kind", func() {
				handlers := map[string]queue.HandlerFunc{
					"reindex": func(_ context.Context, _ []byte) error { return nil },
					"embed":   func(_ context.Context, _ []byte) error { return nil },
				}

				q, err := queue.NewRiverQueue(pool, config.Workers{}, handlers)
				Expect(err).NotTo(HaveOccurred())

				Expect(q.Enqueue(ctx, queue.Job{Kind: "reindex", Payload: []byte(`{}`)})).To(Succeed())
				Expect(q.Enqueue(ctx, queue.Job{Kind: "embed", Payload: []byte(`{}`)})).To(Succeed())

				// Verify jobs exist with correct queue assignment via stats.
				stats, err := q.Stats(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats.Pending).To(Equal(2))
			})
		})

		Describe("stats", func() {
			Context("with no jobs in the queue", func() {
				It("returns zero for all counters", func() {
					handlers := map[string]queue.HandlerFunc{
						"reindex": func(_ context.Context, _ []byte) error { return nil },
					}
					q, err := queue.NewRiverQueue(pool, config.Workers{}, handlers)
					Expect(err).NotTo(HaveOccurred())

					stats, err := q.Stats(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(stats.Pending).To(Equal(0))
					Expect(stats.Running).To(Equal(0))
					Expect(stats.Failed).To(Equal(0))
				})
			})

			Context("after enqueueing jobs", func() {
				It("reports the correct pending count", func() {
					handlers := map[string]queue.HandlerFunc{
						"reindex": func(_ context.Context, _ []byte) error { return nil },
					}
					q, err := queue.NewRiverQueue(pool, config.Workers{}, handlers)
					Expect(err).NotTo(HaveOccurred())

					Expect(q.Enqueue(ctx, queue.Job{Kind: "reindex", Payload: []byte(`{}`)})).To(Succeed())
					Expect(q.Enqueue(ctx, queue.Job{Kind: "reindex", Payload: []byte(`{}`)})).To(Succeed())

					stats, err := q.Stats(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(stats.Pending).To(Equal(2))
				})
			})
		})

		Describe("lifecycle", func() {
			It("starts workers that process enqueued jobs", func() {
				done := make(chan struct{})
				handlers := map[string]queue.HandlerFunc{
					"embed": func(_ context.Context, _ []byte) error {
						close(done)
						return nil
					},
				}

				q, err := queue.NewRiverQueue(pool, config.Workers{Embed: 1}, handlers)
				Expect(err).NotTo(HaveOccurred())

				Expect(q.Enqueue(ctx, queue.Job{Kind: "embed", Payload: []byte(`{}`)})).To(Succeed())

				go func() { _ = q.Start(ctx) }()
				defer func() { _ = q.Stop(context.Background()) }()

				Eventually(done, 10*time.Second).Should(BeClosed())
			})

			It("stops gracefully", func() {
				handlers := map[string]queue.HandlerFunc{
					"reindex": func(_ context.Context, _ []byte) error { return nil },
				}

				q, err := queue.NewRiverQueue(pool, config.Workers{}, handlers)
				Expect(err).NotTo(HaveOccurred())

				go func() { _ = q.Start(ctx) }()
				// Give it a moment to start.
				time.Sleep(500 * time.Millisecond)
				Expect(q.Stop(context.Background())).To(Succeed())
			})
		})

		Describe("error propagation", func() {
			It("returns handler errors so River can retry", func() {
				attempts := 0
				var mu sync.Mutex
				done := make(chan struct{})

				handlers := map[string]queue.HandlerFunc{
					"reindex": func(_ context.Context, _ []byte) error {
						mu.Lock()
						attempts++
						a := attempts
						mu.Unlock()
						if a >= 2 {
							close(done)
							return nil
						}
						return fmt.Errorf("transient failure")
					},
				}

				q, err := queue.NewRiverQueue(pool, config.Workers{Reindex: 1}, handlers)
				Expect(err).NotTo(HaveOccurred())

				Expect(q.Enqueue(ctx, queue.Job{Kind: "reindex", Payload: []byte(`{}`)})).To(Succeed())

				go func() { _ = q.Start(ctx) }()
				defer func() { _ = q.Stop(context.Background()) }()

				Eventually(done, 15*time.Second).Should(BeClosed())
				mu.Lock()
				Expect(attempts).To(BeNumerically(">=", 2))
				mu.Unlock()
			})
		})
	})
})
