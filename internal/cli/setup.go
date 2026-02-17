package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivy/kb/internal/store"
)

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run migrations and check prerequisites",
		RunE:  runSetup,
	}
}

func runSetup(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := configFromContext(ctx)

	pgStore, err := store.NewPGStore(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("creating store: %w", err)
	}

	if err := pgStore.Migrate(ctx); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	fmt.Println("setup complete")
	return nil
}
