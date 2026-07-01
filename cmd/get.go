package cmd

import (
	"fmt"

	"github.com/longkey1/gosla/internal/collector"
	"github.com/longkey1/gosla/internal/config"
	"github.com/longkey1/gosla/internal/model"
	"github.com/longkey1/gosla/internal/output"
	"github.com/longkey1/gosla/internal/slack"
	"github.com/spf13/cobra"
)

var getWithThread bool

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <url>",
		Short: "Get a message or thread from a Slack URL",
		Long: `Get a message or thread from a Slack URL and output as JSON.

Examples:
  gosla get "https://xxx.slack.com/archives/C123/p456"
  gosla get "https://xxx.slack.com/archives/C123/p456" --thread`,
		Args: cobra.ExactArgs(1),
		RunE: runGet,
	}

	cmd.Flags().BoolVar(&getWithThread, "thread", false, "Get the entire thread")

	return cmd
}

func runGet(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override token from flag if provided
	if token != "" {
		cfg.Token = token
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	// Create Slack client
	client := slack.NewClient(cfg.Token)

	// Get message/thread
	opts := collector.GetOptions{
		URL:        url,
		WithThread: getWithThread,
	}

	result, err := collector.Get(client, opts)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	if resolveIDs {
		resolver := slack.NewResolver(client)
		resolved := resolver.ResolveThreads([]model.Thread{*result})
		result = &resolved[0]
	}

	// Output to stdout
	writer := output.NewStdoutWriter()
	return writer.Write(result)
}
