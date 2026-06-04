package cmd

import (
	"fmt"
	"strings"

	"github.com/longkey1/gosla/internal/config"
	"github.com/spf13/cobra"
)

// configKeys lists the valid config keys in a stable, documented order.
var configKeys = []string{"token", "author", "mention"}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show configuration values",
		Long: `Show the effective configuration values.

Values are resolved from the config file, environment variables, and flags
(in increasing order of precedence), matching what other commands use.`,
	}

	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigListCmd())

	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single configuration value",
		Long: `Get the effective value of a single configuration key.

Valid keys: token, author, mention

Examples:
  gosla config get token
  gosla config get mention`,
		Args: cobra.ExactArgs(1),
		RunE: runConfigGet,
	}
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long:  `List all effective configuration values as key=value lines.`,
		Args:  cobra.NoArgs,
		RunE:  runConfigList,
	}
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	values, err := configValues()
	if err != nil {
		return err
	}

	key := args[0]
	value, ok := values[key]
	if !ok {
		return fmt.Errorf("unknown config key %q (valid keys: %s)", key, strings.Join(configKeys, ", "))
	}

	fmt.Println(value)
	return nil
}

func runConfigList(cmd *cobra.Command, args []string) error {
	values, err := configValues()
	if err != nil {
		return err
	}

	for _, key := range configKeys {
		fmt.Printf("%s=%s\n", key, values[key])
	}
	return nil
}

// configValues loads the effective configuration and returns it keyed by name.
func configValues() (map[string]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Override token from flag if provided, matching the other commands.
	if token != "" {
		cfg.Token = token
	}

	return map[string]string{
		"token":   cfg.Token,
		"author":  cfg.Author,
		"mention": strings.Join(cfg.Mention, ","),
	}, nil
}
