package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func reindexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reindex [url]",
		Short: "Trigger reindexing of a source",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				fmt.Printf("reindexing %s: not implemented\n", args[0])
			} else {
				fmt.Println("reindexing all sources: not implemented")
			}
			return nil
		},
	}
}
