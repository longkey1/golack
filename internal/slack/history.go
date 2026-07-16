package slack

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/longkey1/golack/internal/model"
	"github.com/slack-go/slack"
)

// channelIDPattern matches Slack channel IDs (e.g. C0123ABCD, G0123ABCD).
var channelIDPattern = regexp.MustCompile(`^[CGD][A-Z0-9]+$`)

// ChannelRef pairs a resolved channel ID with a display name for output.
type ChannelRef struct {
	ID   string
	Name string
}

// IsChannelID reports whether s looks like a Slack channel ID rather than a name.
func IsChannelID(s string) bool {
	return channelIDPattern.MatchString(s)
}

// ResolveChannels resolves a list of channel names or IDs into ChannelRefs.
// Inputs that look like IDs are used as-is (the display name is filled in via
// conversations.info, falling back to the ID). Inputs that look like names are
// resolved through a single conversations.list lookup; an unknown name is an
// error. The name index is only built when at least one name is present.
func (c *Client) ResolveChannels(inputs []string) ([]ChannelRef, error) {
	needIndex := false
	for _, in := range inputs {
		if !IsChannelID(in) {
			needIndex = true
			break
		}
	}

	var byName map[string]slack.Channel
	if needIndex {
		channels, err := c.ListAllChannels(false)
		if err != nil {
			return nil, err
		}
		byName = make(map[string]slack.Channel, len(channels))
		for _, ch := range channels {
			byName[ch.Name] = ch
		}
	}

	refs := make([]ChannelRef, 0, len(inputs))
	for _, in := range inputs {
		if IsChannelID(in) {
			refs = append(refs, ChannelRef{ID: in, Name: c.GetChannelName(in)})
			continue
		}

		name := strings.TrimPrefix(in, "#")
		ch, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("channel not found: %q", in)
		}
		refs = append(refs, ChannelRef{ID: ch.ID, Name: ch.Name})
	}

	return refs, nil
}

// ListAllChannels lists all public and private channels the token can see,
// paging through every result. Archived channels are skipped when
// excludeArchived is true, which can cut the page count considerably on
// large workspaces.
func (c *Client) ListAllChannels(excludeArchived bool) ([]slack.Channel, error) {
	var all []slack.Channel
	cursor := ""

	for {
		params := &slack.GetConversationsParameters{
			Types:           []string{"public_channel", "private_channel"},
			ExcludeArchived: excludeArchived,
			Limit:           1000,
			Cursor:          cursor,
		}

		var channels []slack.Channel
		var nextCursor string
		var err error

		for retry := 0; retry < maxRetries; retry++ {
			channels, nextCursor, err = c.api.GetConversations(params)
			if err == nil {
				break
			}

			if rateLimitErr, ok := err.(*slack.RateLimitedError); ok {
				waitTime := rateLimitErr.RetryAfter
				if waitTime == 0 {
					waitTime = time.Duration(1<<retry) * time.Second
				}
				time.Sleep(waitTime)
				continue
			}

			return nil, fmt.Errorf("conversations.list API error: %w", err)
		}

		if err != nil {
			return nil, fmt.Errorf("conversations.list API error after retries: %w", err)
		}

		all = append(all, channels...)

		if nextCursor == "" {
			break
		}
		cursor = nextCursor

		// Rate limit prevention
		time.Sleep(time.Second)
	}

	return all, nil
}

// GetChannelHistory fetches every message in a channel within [oldest, latest]
// (inclusive), paging through all results. No filtering is applied.
func (c *Client) GetChannelHistory(channelID, channelName string, oldest, latest time.Time) ([]model.Message, error) {
	var allMessages []model.Message
	cursor := ""

	for {
		params := &slack.GetConversationHistoryParameters{
			ChannelID: channelID,
			Oldest:    strconv.FormatInt(oldest.Unix(), 10),
			Latest:    strconv.FormatInt(latest.Unix(), 10),
			Inclusive: true,
			Limit:     200,
			Cursor:    cursor,
		}

		var resp *slack.GetConversationHistoryResponse
		var err error

		for retry := 0; retry < maxRetries; retry++ {
			resp, err = c.api.GetConversationHistory(params)
			if err == nil {
				break
			}

			if rateLimitErr, ok := err.(*slack.RateLimitedError); ok {
				waitTime := rateLimitErr.RetryAfter
				if waitTime == 0 {
					waitTime = time.Duration(1<<retry) * time.Second
				}
				time.Sleep(waitTime)
				continue
			}

			return nil, fmt.Errorf("conversations.history API error: %w", err)
		}

		if err != nil {
			return nil, fmt.Errorf("conversations.history API error after retries: %w", err)
		}

		for _, msg := range resp.Messages {
			allMessages = append(allMessages, c.convertReplyMessage(msg, channelID, channelName))
		}

		if !resp.HasMore {
			break
		}
		cursor = resp.ResponseMetaData.NextCursor

		// Rate limit prevention
		time.Sleep(time.Second)
	}

	return allMessages, nil
}
