package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show KB status",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("status not implemented")
			return nil
		},
	}
}
