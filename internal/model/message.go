package model

import "time"

// Message represents a Slack message
type Message struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Content        string    `json:"content"`
	Author         string    `json:"author"`
	Timestamp      time.Time `json:"timestamp"`
	Channel        string    `json:"channel"`
	ChannelID      string    `json:"channel_id"`
	Permalink      string    `json:"permalink,omitempty"`
	Mentions       []string  `json:"mentions,omitempty"`
	AttachedLinks  []string  `json:"attached_links,omitempty"`
	ThreadTS       string    `json:"thread_ts"`
	IsThreadParent bool      `json:"is_thread_parent"`
}

// Thread represents a Slack thread with its messages
type Thread struct {
	ThreadID        string    `json:"thread_id"`
	ThreadPermalink string    `json:"thread_permalink,omitempty"`
	Channel         string    `json:"channel,omitempty"`
	ChannelID       string    `json:"channel_id,omitempty"`
	Messages        []Message `json:"messages"`
	MessageCount    int       `json:"message_count,omitempty"`
	ThreadCount     int       `json:"thread_count,omitempty"`
}

// SearchResult represents the result of a search operation
type SearchResult struct {
	Threads []Thread `json:"threads,omitempty"`
	Thread  *Thread  `json:"thread,omitempty"`
}
