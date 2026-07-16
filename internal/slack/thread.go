package slack

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/longkey1/golack/internal/model"
	"github.com/slack-go/slack"
)

const maxRetries = 5

// GetThreadReplies fetches all replies in a thread
func (c *Client) GetThreadReplies(channelID, threadTS string) ([]model.Message, error) {
	var allMessages []model.Message
	cursor := ""

	for {
		params := &slack.GetConversationRepliesParameters{
			ChannelID: channelID,
			Timestamp: threadTS,
			Limit:     200,
			Cursor:    cursor,
		}

		var msgs []slack.Message
		var hasMore bool
		var nextCursor string
		var err error

		// Retry with exponential backoff for rate limits
		for retry := 0; retry < maxRetries; retry++ {
			msgs, hasMore, nextCursor, err = c.api.GetConversationReplies(params)
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

			return nil, fmt.Errorf("conversations.replies API error: %w", err)
		}

		if err != nil {
			return nil, fmt.Errorf("conversations.replies API error after retries: %w", err)
		}

		for _, msg := range msgs {
			allMessages = append(allMessages, c.convertReplyMessage(msg, channelID, ""))
		}

		if !hasMore {
			break
		}
		cursor = nextCursor

		// Rate limit prevention
		time.Sleep(time.Second)
	}

	return allMessages, nil
}

// GetThread fetches a complete thread with channel info
func (c *Client) GetThread(channelID, threadTS string) (*model.Thread, error) {
	// Get channel info
	channelName := channelID
	channelInfo, err := c.api.GetConversationInfo(&slack.GetConversationInfoInput{
		ChannelID: channelID,
	})
	if err != nil {
		// Just use channel ID if we can't get the name (might be missing scope)
		fmt.Fprintf(os.Stderr, "[WARN] Could not get channel info: %v\n", err)
	} else {
		channelName = channelInfo.Name
	}

	// Get permalink for the thread
	permalink := ""
	permalinkResp, err := c.api.GetPermalink(&slack.PermalinkParameters{
		Channel: channelID,
		Ts:      threadTS,
	})
	if err == nil {
		permalink = permalinkResp
	}

	// Get thread messages
	messages, err := c.GetThreadReplies(channelID, threadTS)
	if err != nil {
		return nil, err
	}

	// Update channel info in messages
	for i := range messages {
		messages[i].Channel = channelName
		messages[i].ChannelID = channelID
	}

	return &model.Thread{
		ThreadID:        threadTS,
		ThreadPermalink: permalink,
		Channel:         channelName,
		ChannelID:       channelID,
		Messages:        messages,
		MessageCount:    len(messages),
	}, nil
}

func (c *Client) convertReplyMessage(msg slack.Message, channelID, channelName string) model.Message {
	ts := c.parseTimestamp(msg.Timestamp)
	threadTS := msg.ThreadTimestamp
	if threadTS == "" {
		threadTS = msg.Timestamp
	}

	return model.Message{
		ID:             msg.Timestamp,
		Type:           "slack_message",
		Content:        msg.Text,
		Author:         msg.User,
		Timestamp:      ts,
		Channel:        channelName,
		ChannelID:      channelID,
		Mentions:       c.extractMentionsFromText(msg.Text),
		AttachedLinks:  c.extractLinksFromMessage(msg),
		ThreadTS:       threadTS,
		IsThreadParent: threadTS == "" || threadTS == msg.Timestamp,
	}
}

func (c *Client) extractMentionsFromText(text string) []string {
	re := regexp.MustCompile(`<@([^|>]+)\|([^>]+)>`)
	matches := re.FindAllStringSubmatch(text, -1)

	seen := make(map[string]bool)
	var mentions []string
	for _, match := range matches {
		if len(match) > 2 && !seen[match[2]] {
			mentions = append(mentions, match[2])
			seen[match[2]] = true
		}
	}
	return mentions
}

func (c *Client) extractLinksFromMessage(msg slack.Message) []string {
	seen := make(map[string]bool)
	var links []string

	re := regexp.MustCompile(`https?://[^\s>]+`)
	textLinks := re.FindAllString(msg.Text, -1)
	for _, link := range textLinks {
		if !seen[link] {
			links = append(links, link)
			seen[link] = true
		}
	}

	for _, att := range msg.Attachments {
		if att.TitleLink != "" && !seen[att.TitleLink] {
			links = append(links, att.TitleLink)
			seen[att.TitleLink] = true
		}
	}

	return links
}
