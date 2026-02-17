package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/store"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show KB status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFromContext(ctx)

			pgStore, err := store.NewPGStore(ctx, cfg.Database)
			if err != nil {
				return fmt.Errorf("creating store: %w", err)
			}

			sources, err := pgStore.ListSources(ctx)
			if err != nil {
				return fmt.Errorf("listing sources: %w", err)
			}

			q, err := queue.NewRiverQueue(pgStore.Pool(), cfg.Workers, nil)
			if err != nil {
				return fmt.Errorf("creating queue: %w", err)
			}

			stats, err := q.Stats(ctx)
			if err != nil {
				return fmt.Errorf("getting queue stats: %w", err)
			}

			fmt.Printf("Sources: %d\n", len(sources))
			fmt.Printf("Queue: %d pending, %d running, %d failed\n", stats.Pending, stats.Running, stats.Failed)
			return nil
		},
	}
}
