package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/ivy/kb/internal/config"
)

type contextKey string

const configKey contextKey = "config"

func configFromContext(ctx context.Context) *config.Config {
	return ctx.Value(configKey).(*config.Config)
}

// Execute runs the root command and returns an exit code.
func Execute() int {
	if err := rootCmd().Execute(); err != nil {
		return 1
	}
	return 0
}

func rootCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "kb",
		Short: "KB — knowledge base for your team",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			// CLI flags override config file values
			if v, _ := cmd.Flags().GetString("log-level"); v != "" {
				cfg.Log.Level = v
			}
			if v, _ := cmd.Flags().GetString("log-format"); v != "" {
				cfg.Log.Format = v
			}
			setupLogging(cfg.Log)
			cmd.SetContext(context.WithValue(cmd.Context(), configKey, cfg))
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path (default: kb.yaml)")
	cmd.PersistentFlags().String("log-level", "", "log level (debug, info, warn, error)")
	cmd.PersistentFlags().String("log-format", "", "log format (text, journal)")

	cmd.AddCommand(
		versionCmd(),
		serveCmd(),
		setupCmd(),
		sourcesCmd(),
		reindexCmd(),
		statusCmd(),
	)

	return cmd
}

func setupLogging(cfg config.Log) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
