package cmd

import (
	"github.com/longkey1/gosla/internal/config"
	"github.com/spf13/cobra"
)

var (
	token      string
	configPath string
	resolveIDs bool
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "gosla",
		Short: "Slack Log Collector CLI",
		Long: `gosla is a CLI tool for collecting Slack messages.

It supports:
  - Fetching messages by URL
  - Collecting messages for date ranges
  - Filtering by author and mentions
  - Thread expansion`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			config.SetConfigFile(configPath)
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "Slack API token (overrides SLACK_API_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: $XDG_CONFIG_HOME/gosla/config.toml)")
	rootCmd.PersistentFlags().BoolVar(&resolveIDs, "resolve-ids", false, "Resolve Slack user/channel IDs in message content to human-readable names")

	// Add subcommands
	rootCmd.AddCommand(newGetCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newHistoryCmd())
	rootCmd.AddCommand(newMergeCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd
}

// Execute runs the root command
func Execute() error {
	return NewRootCmd().Execute()
}
