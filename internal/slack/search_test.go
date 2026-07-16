package slack

import (
	"reflect"
	"testing"
	"time"

	"github.com/longkey1/golack/internal/model"
	"github.com/slack-go/slack"
)

func TestBuildBaseQueryParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts SearchOptions
		want []string
	}{
		{
			name: "empty options only exclude dms",
			opts: SearchOptions{},
			want: []string{"-is:dm", "-is:mpdm"},
		},
		{
			name: "author filter",
			opts: SearchOptions{Author: "alice"},
			want: []string{"from:alice", "-is:dm", "-is:mpdm"},
		},
		{
			name: "channel and exclude channel filters",
			opts: SearchOptions{
				Channels:        []string{"general", "random"},
				ExcludeChannels: []string{"announcements"},
			},
			want: []string{"in:general", "in:random", "-in:announcements", "-is:dm", "-is:mpdm"},
		},
		{
			name: "date range",
			opts: SearchOptions{
				After:  time.Date(2025, 1, 14, 0, 0, 0, 0, time.UTC),
				Before: time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC),
			},
			want: []string{"after:2025-01-14", "before:2025-01-16", "-is:dm", "-is:mpdm"},
		},
		{
			name: "all filters combined",
			opts: SearchOptions{
				Author:          "alice",
				Channels:        []string{"general"},
				ExcludeChannels: []string{"bot-logs"},
				After:           time.Date(2025, 1, 14, 0, 0, 0, 0, time.UTC),
				Before:          time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC),
			},
			want: []string{
				"from:alice", "in:general", "-in:bot-logs",
				"after:2025-01-14", "before:2025-01-16",
				"-is:dm", "-is:mpdm",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildBaseQueryParts(tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildBaseQueryParts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMentionQueryPart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mention string
		want    string
	}{
		{"U12345678", "to:U12345678"},
		{"@john.doe", "to:@john.doe"},
		{"@team-name", "to:@team-name"},
		{"team-name", "@team-name"},
	}

	for _, tt := range tests {
		t.Run(tt.mention, func(t *testing.T) {
			t.Parallel()

			if got := mentionQueryPart(tt.mention); got != tt.want {
				t.Errorf("mentionQueryPart(%q) = %q, want %q", tt.mention, got, tt.want)
			}
		})
	}
}

func TestBuildSearchQueries(t *testing.T) {
	t.Parallel()

	c := &Client{}

	tests := []struct {
		name string
		opts SearchOptions
		want []string
	}{
		{
			name: "no mentions yields a single query",
			opts: SearchOptions{Author: "alice"},
			want: []string{"from:alice -is:dm -is:mpdm"},
		},
		{
			name: "one query per mention",
			opts: SearchOptions{
				Author:   "alice",
				Mentions: []string{"U111", "@team"},
			},
			want: []string{
				"from:alice -is:dm -is:mpdm to:U111",
				"from:alice -is:dm -is:mpdm to:@team",
			},
		},
		{
			name: "bare group name uses @ form",
			opts: SearchOptions{Mentions: []string{"backend"}},
			want: []string{"-is:dm -is:mpdm @backend"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := c.buildSearchQueries(tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildSearchQueries() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	t.Parallel()

	c := &Client{}

	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "seconds with microseconds",
			input: "1716192523.567890",
			want:  time.Unix(1716192523, 0),
		},
		{
			name:  "seconds only",
			input: "1716192523",
			want:  time.Unix(1716192523, 0),
		},
		{
			name:  "invalid input yields zero time",
			input: "not-a-timestamp",
			want:  time.Time{},
		},
		{
			name:  "empty input yields zero time",
			input: "",
			want:  time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := c.parseTimestamp(tt.input); !got.Equal(tt.want) {
				t.Errorf("parseTimestamp(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractThreadTS(t *testing.T) {
	t.Parallel()

	c := &Client{}

	tests := []struct {
		name      string
		permalink string
		want      string
	}{
		{
			name:      "permalink with thread_ts",
			permalink: "https://example.slack.com/archives/C123/p1716192523567890?thread_ts=1716192500.123456&cid=C123",
			want:      "1716192500.123456",
		},
		{
			name:      "permalink without thread_ts",
			permalink: "https://example.slack.com/archives/C123/p1716192523567890",
			want:      "",
		},
		{
			name:      "empty permalink",
			permalink: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			match := slack.SearchMessage{Permalink: tt.permalink}
			if got := c.extractThreadTS(match); got != tt.want {
				t.Errorf("extractThreadTS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractMentions(t *testing.T) {
	t.Parallel()

	c := &Client{}

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "no mentions",
			text: "hello world",
			want: nil,
		},
		{
			name: "single mention",
			text: "hey <@U123|alice>, ping",
			want: []string{"alice"},
		},
		{
			name: "multiple mentions deduplicated",
			text: "<@U123|alice> <@U456|bob> <@U123|alice>",
			want: []string{"alice", "bob"},
		},
		{
			name: "mention without name is ignored",
			text: "hey <@U123>",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := c.extractMentions(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractMentions(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestDeduplicateSearchMessages(t *testing.T) {
	t.Parallel()

	c := &Client{}

	got := c.deduplicateMessages([]model.Message{
		{ID: "1.0", Content: "first"},
		{ID: "2.0"},
		{ID: "1.0", Content: "second"},
	})

	if len(got) != 2 {
		t.Fatalf("deduplicateMessages() returned %d messages, want 2", len(got))
	}
	if got[0].ID != "1.0" || got[1].ID != "2.0" {
		t.Errorf("result IDs = [%q, %q], want [1.0, 2.0]", got[0].ID, got[1].ID)
	}
	if got[0].Content != "first" {
		t.Errorf("kept Content = %q, want %q (first occurrence wins)", got[0].Content, "first")
	}
}
