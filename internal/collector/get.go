package collector

import (
	"fmt"

	"github.com/longkey1/golack/internal/model"
	"github.com/longkey1/golack/internal/slack"
)

// GetOptions contains options for the get command
type GetOptions struct {
	URL        string
	WithThread bool
}

// Get fetches a message or thread from a Slack URL
func Get(client *slack.Client, opts GetOptions) (*model.Thread, error) {
	urlInfo, err := slack.ParseURL(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Determine the thread timestamp to use
	threadTS := urlInfo.ThreadTS
	if threadTS == "" {
		threadTS = urlInfo.MessageTS
	}

	if opts.WithThread || urlInfo.ThreadTS != "" {
		// Get the entire thread
		return client.GetThread(urlInfo.ChannelID, threadTS)
	}

	// Get single message
	messages, err := client.GetThreadReplies(urlInfo.ChannelID, urlInfo.MessageTS)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	// Filter to just the requested message
	var targetMsg *model.Message
	for _, msg := range messages {
		if msg.ID == urlInfo.MessageTS {
			targetMsg = &msg
			break
		}
	}

	if targetMsg == nil && len(messages) > 0 {
		targetMsg = &messages[0]
	}

	if targetMsg == nil {
		return nil, fmt.Errorf("message not found")
	}

	// Get channel name
	channelName := client.GetChannelName(urlInfo.ChannelID)
	targetMsg.Channel = channelName
	targetMsg.ChannelID = urlInfo.ChannelID

	return &model.Thread{
		ThreadID:     targetMsg.ThreadTS,
		Channel:      channelName,
		ChannelID:    urlInfo.ChannelID,
		Messages:     []model.Message{*targetMsg},
		MessageCount: 1,
	}, nil
}
