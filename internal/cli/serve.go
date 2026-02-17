package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ivy/kb/internal/agent"
	"github.com/ivy/kb/internal/embed"
	"github.com/ivy/kb/internal/index"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/server"
	"github.com/ivy/kb/internal/session"
	"github.com/ivy/kb/internal/store"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the KB server",
		RunE:  runServe,
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := configFromContext(cmd.Context())

	// 1. Store
	pgStore, err := store.NewPGStore(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("creating store: %w", err)
	}

	// 2. Migrations
	if err := pgStore.Migrate(ctx); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	// 3. Queue — two-phase init for ReindexHandler → Queue cycle
	//    First pass: create handlers map with nil queue for reindex
	var q queue.Queue
	handlers := map[string]queue.HandlerFunc{
		"reindex": func(ctx context.Context, payload []byte) error {
			// Closure captures q, which is set after NewRiverQueue returns
			return index.ReindexHandler(pgStore, q, cfg.GitLab)(ctx, payload)
		},
		"embed": embed.EmbedHandler(pgStore, cfg.Embedding),
	}

	q, err = queue.NewRiverQueue(pgStore.Pool(), cfg.Workers, handlers)
	if err != nil {
		return fmt.Errorf("creating queue: %w", err)
	}

	// 4. Sessions
	sessions := session.NewLRUManager(cfg.Sessions)

	// 5. Agent
	ag := agent.New(pgStore, pgStore, cfg.Agent)

	// 6. Server
	srv := server.New(ag, sessions, q, cfg.Tailscale)

	slog.Info("starting kb server")

	// Start queue workers in background
	errCh := make(chan error, 2)
	go func() {
		if err := q.Start(ctx); err != nil {
			errCh <- fmt.Errorf("queue: %w", err)
		}
	}()

	// Start HTTP server (blocks until ctx cancels)
	go func() {
		if err := srv.Serve(ctx); err != nil {
			errCh <- fmt.Errorf("server: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case err := <-errCh:
		cancel()
		return err
	}

	// Graceful shutdown: stop queue first (drain in-flight jobs)
	if err := q.Stop(context.Background()); err != nil {
		slog.Error("queue shutdown error", "err", err)
	}

	return nil
}
