package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the KB server",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("starting server...")
			return nil
		},
	}
}
