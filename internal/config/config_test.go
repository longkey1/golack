package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeConfig writes a config file with the given content into a fresh
// temp dir and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// isolateEnv points every config source at empty locations so tests never
// pick up the developer's real config file or environment variables.
// t.Setenv to "" is enough for the viper bindings: empty env values are
// treated as unset (AllowEmptyEnv defaults to false).
func isolateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SLACK_API_TOKEN", "")
	t.Setenv("SLACK_AUTHOR", "")
	t.Setenv("SLACK_MENTION", "")
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string            // config file content; used when hasFile is true
		hasFile bool              // whether an explicit config file is written and registered
		env     map[string]string // extra env vars set for the test
		want    Config
		wantErr bool
	}{
		{
			name: "all values from file",
			content: `
token   = "xoxp-file"
author  = "alice"
mention = ["U111", "@john.doe", "@team-name"]
`,
			hasFile: true,
			want: Config{
				Token:   "xoxp-file",
				Author:  "alice",
				Mention: []string{"U111", "@john.doe", "@team-name"},
			},
		},
		{
			name: "env var expansion in file values",
			content: `
token   = "${GOSLA_TEST_TOKEN}"
author  = "${GOSLA_TEST_AUTHOR}"
mention = ["${GOSLA_TEST_MENTION}", "@team-name"]
`,
			hasFile: true,
			env: map[string]string{
				"GOSLA_TEST_TOKEN":   "xoxp-expanded",
				"GOSLA_TEST_AUTHOR":  "bob",
				"GOSLA_TEST_MENTION": "U222",
			},
			want: Config{
				Token:   "xoxp-expanded",
				Author:  "bob",
				Mention: []string{"U222", "@team-name"},
			},
		},
		{
			name: "unset env var reference expands to empty",
			content: `
token = "${GOSLA_TEST_UNSET_VAR}"
`,
			hasFile: true,
			want:    Config{},
		},
		{
			name: "mention entries are trimmed",
			content: `
token   = "xoxp-file"
mention = [" U111 ", "  @team  "]
`,
			hasFile: true,
			want: Config{
				Token:   "xoxp-file",
				Mention: []string{"U111", "@team"},
			},
		},
		{
			name: "env vars only",
			env: map[string]string{
				"SLACK_API_TOKEN": "xoxp-env",
				"SLACK_AUTHOR":    "carol",
				"SLACK_MENTION":   "U333",
			},
			want: Config{
				Token:   "xoxp-env",
				Author:  "carol",
				Mention: []string{"U333"},
			},
		},
		{
			name: "SLACK_MENTION env var is comma-separated",
			env: map[string]string{
				"SLACK_API_TOKEN": "xoxp-env",
				"SLACK_MENTION":   "U333, @john.doe ,@team,",
			},
			want: Config{
				Token:   "xoxp-env",
				Mention: []string{"U333", "@john.doe", "@team"},
			},
		},
		{
			name: "env vars override file values",
			content: `
token  = "xoxp-file"
author = "alice"
`,
			hasFile: true,
			env: map[string]string{
				"SLACK_API_TOKEN": "xoxp-env",
			},
			want: Config{
				Token:  "xoxp-env",
				Author: "alice",
			},
		},
		{
			name: "no file and no env yields empty config",
			want: Config{},
		},
		{
			name:    "invalid toml",
			content: `token = `,
			hasFile: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			if tt.hasFile {
				SetConfigFile(writeConfig(t, tt.content))
			} else {
				SetConfigFile("")
			}
			t.Cleanup(func() { SetConfigFile("") })

			got, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("Load() = %+v, want %+v", *got, tt.want)
			}
		})
	}
}

func TestLoadExplicitFileMissing(t *testing.T) {
	isolateEnv(t)

	SetConfigFile(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	t.Cleanup(func() { SetConfigFile("") })

	if _, err := Load(); err == nil {
		t.Error("Load() with missing explicit config file should return an error")
	}
}

func TestLoadDefaultSearchPath(t *testing.T) {
	isolateEnv(t)

	// A config file placed under $XDG_CONFIG_HOME/gosla is found without an
	// explicit --config path.
	xdg := t.TempDir()
	dir := filepath.Join(xdg, "gosla")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`token = "xoxp-xdg"`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)

	SetConfigFile("")
	t.Cleanup(func() { SetConfigFile("") })

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Token != "xoxp-xdg" {
		t.Errorf("Load() Token = %q, want %q", got.Token, "xoxp-xdg")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "token present",
			cfg:  Config{Token: "xoxp-123"},
		},
		{
			name:    "token missing",
			cfg:     Config{Author: "alice"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
