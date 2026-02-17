package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage content sources",
	}

	addCmd := &cobra.Command{
		Use:   "add [url]",
		Short: "Add a content source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("adding source %s: not implemented\n", args[0])
			return nil
		},
	}
	addCmd.Flags().String("kind", "gitlab", "source kind (gitlab)")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List content sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("sources list not implemented")
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove [url]",
		Short: "Remove a content source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("removing source %s: not implemented\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(addCmd, listCmd, removeCmd)
	return cmd
}
