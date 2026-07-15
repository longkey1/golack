package cmd

import (
	"reflect"
	"testing"

	"github.com/longkey1/gosla/internal/slack"
)

func TestValidateTypeFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{name: "empty", in: "", wantErr: false},
		{name: "user", in: "user", wantErr: false},
		{name: "channel", in: "channel", wantErr: false},
		{name: "usergroup", in: "usergroup", wantErr: false},
		{name: "invalid", in: "bot", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateTypeFilter(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTypeFilter(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestClassifyQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        string
		typeFilter string
		want       resolveQuery
		wantErr    bool
	}{
		{
			name: "user ID",
			raw:  "U0123ABCD",
			want: resolveQuery{raw: "U0123ABCD", kind: kindID, types: []string{slack.TypeUser}, value: "U0123ABCD"},
		},
		{
			name: "enterprise user ID",
			raw:  "W0123ABCD",
			want: resolveQuery{raw: "W0123ABCD", kind: kindID, types: []string{slack.TypeUser}, value: "W0123ABCD"},
		},
		{
			name: "channel ID",
			raw:  "C0123ABCD",
			want: resolveQuery{raw: "C0123ABCD", kind: kindID, types: []string{slack.TypeChannel}, value: "C0123ABCD"},
		},
		{
			name: "usergroup ID",
			raw:  "S0123ABCD",
			want: resolveQuery{raw: "S0123ABCD", kind: kindID, types: []string{slack.TypeUsergroup}, value: "S0123ABCD"},
		},
		{
			name: "channel name with hash",
			raw:  "#general",
			want: resolveQuery{raw: "#general", kind: kindName, types: []string{slack.TypeChannel}, value: "general"},
		},
		{
			name: "at-name searches users and usergroups",
			raw:  "@john.doe",
			want: resolveQuery{raw: "@john.doe", kind: kindName, types: []string{slack.TypeUser, slack.TypeUsergroup}, value: "john.doe"},
		},
		{
			name: "email",
			raw:  "john.doe@example.com",
			want: resolveQuery{raw: "john.doe@example.com", kind: kindEmail, types: []string{slack.TypeUser}, value: "john.doe@example.com"},
		},
		{
			name: "bare name searches all types",
			raw:  "general",
			want: resolveQuery{raw: "general", kind: kindName, types: []string{slack.TypeUser, slack.TypeChannel, slack.TypeUsergroup}, value: "general"},
		},
		{
			name:       "bare name with type filter",
			raw:        "general",
			typeFilter: "channel",
			want:       resolveQuery{raw: "general", kind: kindName, types: []string{slack.TypeChannel}, value: "general"},
		},
		{
			name:       "at-name narrowed by type filter",
			raw:        "@backend",
			typeFilter: "usergroup",
			want:       resolveQuery{raw: "@backend", kind: kindName, types: []string{slack.TypeUsergroup}, value: "backend"},
		},
		{
			name:       "channel ID treated as user name under --type user",
			raw:        "C0123ABCD",
			typeFilter: "user",
			want:       resolveQuery{raw: "C0123ABCD", kind: kindName, types: []string{slack.TypeUser}, value: "C0123ABCD"},
		},
		{
			name:       "hash name contradicts --type user",
			raw:        "#general",
			typeFilter: "user",
			wantErr:    true,
		},
		{
			name:       "at-name contradicts --type channel",
			raw:        "@john.doe",
			typeFilter: "channel",
			wantErr:    true,
		},
		{
			name:       "email contradicts --type channel",
			raw:        "john.doe@example.com",
			typeFilter: "channel",
			wantErr:    true,
		},
		{
			name:    "empty query is an error",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "bare hash is an error",
			raw:     "#",
			wantErr: true,
		},
		{
			name:    "bare at is an error",
			raw:     "@",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := classifyQuery(tt.raw, tt.typeFilter)
			if (err != nil) != tt.wantErr {
				t.Fatalf("classifyQuery(%q, %q) error = %v, wantErr %v", tt.raw, tt.typeFilter, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("classifyQuery(%q, %q) = %+v, want %+v", tt.raw, tt.typeFilter, got, tt.want)
			}
		})
	}
}
