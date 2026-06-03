package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Token   string
	Author  string
	Mention []string
}

// configFile is set by the root command from the --config flag.
var configFile string

// SetConfigFile registers an explicit config file path to be used by Load.
// An empty string falls back to the default search paths.
func SetConfigFile(path string) {
	configFile = path
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigType("toml")

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName("config")
		for _, dir := range defaultConfigPaths() {
			v.AddConfigPath(dir)
		}
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// When the user explicitly specified --config, a missing file is an error.
		if configFile != "" {
			return nil, fmt.Errorf("failed to read config file %q: %w", configFile, err)
		}
	}

	_ = v.BindEnv("token", "SLACK_API_TOKEN")
	_ = v.BindEnv("author", "SLACK_AUTHOR")
	_ = v.BindEnv("mention", "SLACK_MENTION")

	cfg := &Config{
		Token:  os.ExpandEnv(v.GetString("token")),
		Author: os.ExpandEnv(v.GetString("author")),
	}

	if raw := v.GetStringSlice("mention"); len(raw) > 0 {
		cfg.Mention = expandSlice(raw)
	} else if s := v.GetString("mention"); s != "" {
		// SLACK_MENTION env var is a comma-separated string.
		cfg.Mention = expandSlice(strings.Split(s, ","))
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("slack API token is required (set SLACK_API_TOKEN, 'token' in config file, or use --token flag)")
	}
	return nil
}

func expandSlice(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = os.ExpandEnv(strings.TrimSpace(s))
	}
	return out
}

// defaultConfigPaths returns the directories searched for config.toml when no
// explicit --config is given, in priority order.
func defaultConfigPaths() []string {
	var paths []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "gosla"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "gosla"))
	}
	return paths
}
