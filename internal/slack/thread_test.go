package slack

import (
	"reflect"
	"testing"
	"time"

	"github.com/slack-go/slack"
)

func TestConvertReplyMessage(t *testing.T) {
	t.Parallel()

	c := &Client{}

	tests := []struct {
		name             string
		msg              slack.Message
		wantThreadTS     string
		wantThreadParent bool
	}{
		{
			name: "message without thread ts is a parent",
			msg: slack.Message{Msg: slack.Msg{
				Timestamp: "1716192523.567890",
				Text:      "hello",
				User:      "U123",
			}},
			wantThreadTS:     "1716192523.567890",
			wantThreadParent: true,
		},
		{
			name: "thread parent has thread ts equal to its own ts",
			msg: slack.Message{Msg: slack.Msg{
				Timestamp:       "1716192523.567890",
				ThreadTimestamp: "1716192523.567890",
			}},
			wantThreadTS:     "1716192523.567890",
			wantThreadParent: true,
		},
		{
			name: "thread reply is not a parent",
			msg: slack.Message{Msg: slack.Msg{
				Timestamp:       "1716192600.000100",
				ThreadTimestamp: "1716192523.567890",
			}},
			wantThreadTS:     "1716192523.567890",
			wantThreadParent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := c.convertReplyMessage(tt.msg, "C123", "general")

			if got.ID != tt.msg.Timestamp {
				t.Errorf("ID = %q, want %q", got.ID, tt.msg.Timestamp)
			}
			if got.Type != "slack_message" {
				t.Errorf("Type = %q, want %q", got.Type, "slack_message")
			}
			if got.Content != tt.msg.Text {
				t.Errorf("Content = %q, want %q", got.Content, tt.msg.Text)
			}
			if got.Author != tt.msg.User {
				t.Errorf("Author = %q, want %q", got.Author, tt.msg.User)
			}
			if got.Channel != "general" || got.ChannelID != "C123" {
				t.Errorf("channel = (%q, %q), want (general, C123)", got.Channel, got.ChannelID)
			}
			if got.ThreadTS != tt.wantThreadTS {
				t.Errorf("ThreadTS = %q, want %q", got.ThreadTS, tt.wantThreadTS)
			}
			if got.IsThreadParent != tt.wantThreadParent {
				t.Errorf("IsThreadParent = %v, want %v", got.IsThreadParent, tt.wantThreadParent)
			}
			if want := time.Unix(1716192523, 0); tt.name == "message without thread ts is a parent" && !got.Timestamp.Equal(want) {
				t.Errorf("Timestamp = %v, want %v", got.Timestamp, want)
			}
		})
	}
}

func TestExtractMentionsFromText(t *testing.T) {
	t.Parallel()

	c := &Client{}

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "no mentions",
			text: "plain text",
			want: nil,
		},
		{
			name: "named mentions deduplicated",
			text: "<@U123|alice> hi <@U456|bob> and again <@U123|alice>",
			want: []string{"alice", "bob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := c.extractMentionsFromText(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractMentionsFromText(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestExtractLinksFromMessage(t *testing.T) {
	t.Parallel()

	c := &Client{}

	tests := []struct {
		name string
		msg  slack.Message
		want []string
	}{
		{
			name: "no links",
			msg:  slack.Message{Msg: slack.Msg{Text: "no links here"}},
			want: nil,
		},
		{
			name: "links extracted from text",
			msg: slack.Message{Msg: slack.Msg{
				Text: "see https://example.com/a and <https://example.com/b>",
			}},
			want: []string{"https://example.com/a", "https://example.com/b"},
		},
		{
			name: "links from attachments and deduplication",
			msg: slack.Message{Msg: slack.Msg{
				Text: "see https://example.com/a",
				Attachments: []slack.Attachment{
					{TitleLink: "https://example.com/a"},
					{TitleLink: "https://example.com/att"},
					{TitleLink: ""},
				},
			}},
			want: []string{"https://example.com/a", "https://example.com/att"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := c.extractLinksFromMessage(tt.msg)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractLinksFromMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
