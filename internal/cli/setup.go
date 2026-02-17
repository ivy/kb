package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run migrations and check prerequisites",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("setup not implemented")
			return nil
		},
	}
}
