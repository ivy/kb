package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	gitRef    = "unknown"
	buildTag  = "unknown"
	buildDate = "unknown"
)

func versionCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		// Skip config loading for version command
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]string{
					"version":    version,
					"git_ref":    gitRef,
					"build_tag":  buildTag,
					"build_date": buildDate,
				})
			}
			fmt.Printf("kb %s (%s) built %s at %s\n", version, gitRef, buildTag, buildDate)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}
