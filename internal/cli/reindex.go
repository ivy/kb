package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivy/kb/internal/index"
	"github.com/ivy/kb/internal/queue"
	"github.com/ivy/kb/internal/store"
)

func reindexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reindex [url]",
		Short: "Trigger reindexing of a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFromContext(ctx)

			pgStore, err := store.NewPGStore(ctx, cfg.Database)
			if err != nil {
				return fmt.Errorf("creating store: %w", err)
			}

			q, err := queue.NewRiverQueue(pgStore.Pool(), cfg.Workers, nil)
			if err != nil {
				return fmt.Errorf("creating queue: %w", err)
			}

			payload, err := json.Marshal(index.ReindexJob{SourceURL: args[0]})
			if err != nil {
				return fmt.Errorf("marshaling job: %w", err)
			}

			if err := q.Enqueue(ctx, queue.Job{Kind: "reindex", Payload: payload}); err != nil {
				return fmt.Errorf("enqueuing reindex: %w", err)
			}

			fmt.Printf("reindex job enqueued for %s\n", args[0])
			return nil
		},
	}
}
