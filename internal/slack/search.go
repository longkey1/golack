package slack

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/longkey1/gosla/internal/model"
	"github.com/slack-go/slack"
)

// SearchOptions contains options for searching messages
type SearchOptions struct {
	Author          string
	Mentions        []string
	Channels        []string
	ExcludeChannels []string
	After           time.Time
	Before          time.Time
}

const searchMaxRetries = 5

// SearchMessages searches for messages matching the given options.
//
// When multiple mentions are specified they are treated as OR: each mention
// produces its own query (Slack search has no OR operator for the to: modifier),
// and the results are merged and deduplicated. processedThreads is shared across
// queries so a thread surfaced by more than one mention is fetched only once.
func (c *Client) SearchMessages(opts SearchOptions) ([]model.Message, error) {
	var allMessages []model.Message
	processedThreads := make(map[string]bool)

	for _, query := range c.buildSearchQueries(opts) {
		msgs, err := c.searchByQuery(query, processedThreads)
		if err != nil {
			return nil, err
		}
		allMessages = append(allMessages, msgs...)
	}

	return c.deduplicateMessages(allMessages), nil
}

// searchByQuery runs a single search query, paging through all results.
func (c *Client) searchByQuery(query string, processedThreads map[string]bool) ([]model.Message, error) {
	var messages []model.Message
	params := slack.SearchParameters{
		Count: 100,
		Sort:  "timestamp",
	}

	for {
		var result *slack.SearchMessages
		var err error

		// Retry with exponential backoff for rate limits
		for retry := 0; retry < searchMaxRetries; retry++ {
			result, err = c.api.SearchMessages(query, params)
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

			return nil, fmt.Errorf("search.messages API error: %w", err)
		}

		if err != nil {
			return nil, fmt.Errorf("search.messages API error after retries: %w", err)
		}

		if len(result.Matches) == 0 {
			break
		}

		for _, match := range result.Matches {
			msg := c.convertSearchMatch(match)

			// Check if this is part of a thread
			threadTS := c.extractThreadTS(match)
			if threadTS != "" && threadTS != msg.ID {
				// This message is in a thread
				if !processedThreads[threadTS] {
					// Get the entire thread
					threadMsgs, err := c.GetThreadReplies(match.Channel.ID, threadTS)
					if err != nil {
						// Log error but continue
						fmt.Fprintf(os.Stderr, "[WARN] Failed to get thread %s: %v\n", threadTS, err)
						messages = append(messages, msg)
					} else {
						messages = append(messages, threadMsgs...)
					}
					processedThreads[threadTS] = true
				}
			} else {
				messages = append(messages, msg)
			}
		}

		// Check for more pages
		if result.Paging.Pages <= result.Paging.Page {
			break
		}
		params.Page = result.Paging.Page + 1

		// Rate limit prevention
		time.Sleep(time.Second)
	}

	return messages, nil
}

// buildSearchQueries builds one query per mention so that multiple mentions are
// matched as OR. With no mentions it returns a single query without a mention
// filter.
func (c *Client) buildSearchQueries(opts SearchOptions) []string {
	base := buildBaseQueryParts(opts)

	if len(opts.Mentions) == 0 {
		return []string{strings.Join(base, " ")}
	}

	queries := make([]string, 0, len(opts.Mentions))
	for _, mention := range opts.Mentions {
		parts := make([]string, len(base), len(base)+1)
		copy(parts, base)
		parts = append(parts, mentionQueryPart(mention))
		queries = append(queries, strings.Join(parts, " "))
	}
	return queries
}

// buildBaseQueryParts builds the query modifiers shared by every query (all
// filters except the mention).
func buildBaseQueryParts(opts SearchOptions) []string {
	var parts []string

	if opts.Author != "" {
		parts = append(parts, fmt.Sprintf("from:%s", opts.Author))
	}

	for _, channel := range opts.Channels {
		parts = append(parts, fmt.Sprintf("in:%s", channel))
	}

	for _, channel := range opts.ExcludeChannels {
		parts = append(parts, fmt.Sprintf("-in:%s", channel))
	}

	if !opts.After.IsZero() {
		parts = append(parts, fmt.Sprintf("after:%s", opts.After.Format("2006-01-02")))
	}

	if !opts.Before.IsZero() {
		parts = append(parts, fmt.Sprintf("before:%s", opts.Before.Format("2006-01-02")))
	}

	// Exclude DMs and group DMs
	parts = append(parts, "-is:dm", "-is:mpdm")

	return parts
}

// mentionQueryPart builds the search modifier for a single mention, handling
// both user IDs / @-prefixed names (to:) and bare group names (@name).
func mentionQueryPart(mention string) string {
	if strings.HasPrefix(mention, "@") || strings.HasPrefix(mention, "U") {
		return fmt.Sprintf("to:%s", mention)
	}
	return fmt.Sprintf("@%s", mention)
}

func (c *Client) convertSearchMatch(match slack.SearchMessage) model.Message {
	ts := c.parseTimestamp(match.Timestamp)

	return model.Message{
		ID:             match.Timestamp,
		Type:           "slack_message",
		Content:        match.Text,
		Author:         match.User,
		Timestamp:      ts,
		Channel:        match.Channel.Name,
		ChannelID:      match.Channel.ID,
		Permalink:      match.Permalink,
		Mentions:       c.extractMentions(match.Text),
		AttachedLinks:  c.extractLinks(match),
		ThreadTS:       match.Timestamp,
		IsThreadParent: true,
	}
}

func (c *Client) extractThreadTS(match slack.SearchMessage) string {
	// Try to extract thread_ts from permalink
	if strings.Contains(match.Permalink, "thread_ts=") {
		re := regexp.MustCompile(`thread_ts=([0-9.]+)`)
		matches := re.FindStringSubmatch(match.Permalink)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func (c *Client) parseTimestamp(ts string) time.Time {
	parts := strings.Split(ts, ".")
	if len(parts) == 0 {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

func (c *Client) extractMentions(text string) []string {
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

func (c *Client) extractLinks(match slack.SearchMessage) []string {
	seen := make(map[string]bool)
	var links []string

	// Extract from text
	re := regexp.MustCompile(`https?://[^\s>]+`)
	textLinks := re.FindAllString(match.Text, -1)
	for _, link := range textLinks {
		if !seen[link] {
			links = append(links, link)
			seen[link] = true
		}
	}

	// Extract from attachments
	for _, att := range match.Attachments {
		if att.TitleLink != "" && !seen[att.TitleLink] {
			links = append(links, att.TitleLink)
			seen[att.TitleLink] = true
		}
	}

	return links
}

func (c *Client) deduplicateMessages(messages []model.Message) []model.Message {
	seen := make(map[string]bool)
	var result []model.Message

	for _, msg := range messages {
		if !seen[msg.ID] {
			seen[msg.ID] = true
			result = append(result, msg)
		}
	}

	return result
}
