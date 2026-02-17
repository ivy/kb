package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ivy/kb/internal/store"
)

func sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage content sources",
	}

	cmd.AddCommand(sourcesAddCmd(), sourcesListCmd(), sourcesRemoveCmd())
	return cmd
}

func sourcesAddCmd() *cobra.Command {
	var kind string

	cmd := &cobra.Command{
		Use:   "add [url]",
		Short: "Add a content source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFromContext(ctx)

			pgStore, err := store.NewPGStore(ctx, cfg.Database)
			if err != nil {
				return fmt.Errorf("creating store: %w", err)
			}

			if err := pgStore.AddSource(ctx, args[0], kind, json.RawMessage("{}")); err != nil {
				return fmt.Errorf("adding source: %w", err)
			}

			fmt.Printf("added source: %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&kind, "kind", "gitlab", "source kind (gitlab)")
	return cmd
}

func sourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List content sources",
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

			if len(sources) == 0 {
				fmt.Println("no sources configured")
				return nil
			}

			for _, s := range sources {
				fmt.Printf("%s (%s)\n", s.URL, s.Kind)
			}
			return nil
		},
	}
}

func sourcesRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [url]",
		Short: "Remove a content source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := configFromContext(ctx)

			pgStore, err := store.NewPGStore(ctx, cfg.Database)
			if err != nil {
				return fmt.Errorf("creating store: %w", err)
			}

			if err := pgStore.RemoveSource(ctx, args[0]); err != nil {
				return fmt.Errorf("removing source: %w", err)
			}

			fmt.Printf("removed source: %s\n", args[0])
			return nil
		},
	}
}
