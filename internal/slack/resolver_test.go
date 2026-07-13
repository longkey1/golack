package slack

import (
	"testing"

	"github.com/longkey1/gosla/internal/model"
)

// newTestResolver builds a Resolver with pre-populated caches so that no
// Slack API call is ever made during tests (every lookup is a cache hit).
func newTestResolver() *Resolver {
	return &Resolver{
		userCache:      map[string]string{"U123": "alice", "U456": "bob"},
		channelCache:   map[string]string{"C123": "general"},
		usergroupCache: map[string]string{"S123": "@backend"},
	}
}

func TestResolveContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "plain text unchanged",
			text: "hello world",
			want: "hello world",
		},
		{
			name: "user mention with inline name",
			text: "hey <@U999|carol>",
			want: "hey @carol",
		},
		{
			name: "user mention resolved from cache",
			text: "hey <@U123>",
			want: "hey @alice",
		},
		{
			name: "channel reference with inline name",
			text: "see <#C999|random>",
			want: "see #random",
		},
		{
			name: "channel reference resolved from cache",
			text: "see <#C123>",
			want: "see #general",
		},
		{
			name: "usergroup with inline label",
			text: "cc <!subteam^S999|@frontend>",
			want: "cc @frontend",
		},
		{
			name: "usergroup resolved from cache",
			text: "cc <!subteam^S123>",
			want: "cc @backend",
		},
		{
			name: "here broadcast",
			text: "<!here> please review",
			want: "@here please review",
		},
		{
			name: "channel broadcast",
			text: "<!channel> heads up",
			want: "@channel heads up",
		},
		{
			name: "everyone broadcast",
			text: "<!everyone> announcement",
			want: "@everyone announcement",
		},
		{
			name: "multiple tokens in one message",
			text: "<@U123> posted in <#C123>: <!here>",
			want: "@alice posted in #general: @here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newTestResolver()
			if got := r.resolveContent(tt.text); got != tt.want {
				t.Errorf("resolveContent(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestResolveThreads(t *testing.T) {
	t.Parallel()

	r := newTestResolver()
	threads := []model.Thread{
		{
			ThreadID: "1.0",
			Messages: []model.Message{
				{ID: "1.0", Author: "U123", Content: "ping <@U456>"},
				{ID: "1.1", Author: "U456", Content: "pong"},
			},
		},
	}

	got := r.ResolveThreads(threads)

	if len(got) != 1 || len(got[0].Messages) != 2 {
		t.Fatalf("unexpected shape: %+v", got)
	}
	if got[0].Messages[0].Author != "alice" {
		t.Errorf("Messages[0].Author = %q, want %q", got[0].Messages[0].Author, "alice")
	}
	if got[0].Messages[0].Content != "ping @bob" {
		t.Errorf("Messages[0].Content = %q, want %q", got[0].Messages[0].Content, "ping @bob")
	}
	if got[0].Messages[1].Author != "bob" {
		t.Errorf("Messages[1].Author = %q, want %q", got[0].Messages[1].Author, "bob")
	}

	// The input threads must not be mutated.
	if threads[0].Messages[0].Author != "U123" {
		t.Errorf("input was mutated: Author = %q, want %q", threads[0].Messages[0].Author, "U123")
	}
}
