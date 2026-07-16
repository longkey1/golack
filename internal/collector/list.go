package collector

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/longkey1/golack/internal/model"
	"github.com/longkey1/golack/internal/slack"
)

// ListOptions contains options for the list command
type ListOptions struct {
	Date            time.Time
	Author          string
	Mentions        []string
	Channels        []string
	ExcludeChannels []string
	WithThread      bool
}

// DayResult contains the result of collecting messages for a day
type DayResult struct {
	Date     time.Time
	Threads  []model.Thread
	Messages []model.Message
	Error    error
}

// List collects messages for a specific day
func List(client *slack.Client, opts ListOptions) (*DayResult, error) {
	// Calculate date range for search (day before and day after for accurate filtering)
	prevDate := opts.Date.AddDate(0, 0, -1)
	nextDate := opts.Date.AddDate(0, 0, 1)

	searchOpts := slack.SearchOptions{
		Author:          opts.Author,
		Mentions:        opts.Mentions,
		Channels:        opts.Channels,
		ExcludeChannels: opts.ExcludeChannels,
		After:           prevDate,
		Before:          nextDate,
	}

	messages, err := client.SearchMessages(searchOpts)
	if err != nil {
		return &DayResult{
			Date:  opts.Date,
			Error: err,
		}, err
	}

	// If thread option is enabled, fetch full threads
	if opts.WithThread {
		messages, err = fetchThreads(client, messages)
		if err != nil {
			return &DayResult{
				Date:  opts.Date,
				Error: err,
			}, err
		}
	}

	// Group messages by thread
	threads := groupByThread(messages)

	return &DayResult{
		Date:     opts.Date,
		Threads:  threads,
		Messages: messages,
	}, nil
}

func fetchThreads(client *slack.Client, messages []model.Message) ([]model.Message, error) {
	processedThreads := make(map[string]bool)
	var allMessages []model.Message

	for _, msg := range messages {
		threadTS := msg.ThreadTS
		if threadTS == "" {
			threadTS = msg.ID
		}

		if processedThreads[threadTS] {
			continue
		}

		// Get the entire thread
		threadMsgs, err := client.GetThreadReplies(msg.ChannelID, threadTS)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to get thread %s: %v\n", threadTS, err)
			allMessages = append(allMessages, msg)
			continue
		}

		// Update channel info
		for i := range threadMsgs {
			if threadMsgs[i].Channel == "" {
				threadMsgs[i].Channel = msg.Channel
			}
			if threadMsgs[i].ChannelID == "" {
				threadMsgs[i].ChannelID = msg.ChannelID
			}
		}

		allMessages = append(allMessages, threadMsgs...)
		processedThreads[threadTS] = true
	}

	return deduplicateMessages(allMessages), nil
}

func groupByThread(messages []model.Message) []model.Thread {
	threadMap := make(map[string]*model.Thread)

	for _, msg := range messages {
		threadTS := msg.ThreadTS
		if threadTS == "" {
			threadTS = msg.ID
		}

		if thread, exists := threadMap[threadTS]; exists {
			thread.Messages = append(thread.Messages, msg)
			thread.ThreadCount = len(thread.Messages)
		} else {
			threadMap[threadTS] = &model.Thread{
				ThreadID:    threadTS,
				Channel:     msg.Channel,
				ChannelID:   msg.ChannelID,
				Messages:    []model.Message{msg},
				ThreadCount: 1,
			}
		}
	}

	// Convert map to slice and sort
	var threads []model.Thread
	for _, thread := range threadMap {
		// Sort messages within thread by timestamp
		sort.Slice(thread.Messages, func(i, j int) bool {
			return thread.Messages[i].Timestamp.Before(thread.Messages[j].Timestamp)
		})
		threads = append(threads, *thread)
	}

	// Sort threads by first message timestamp
	sort.Slice(threads, func(i, j int) bool {
		if len(threads[i].Messages) == 0 || len(threads[j].Messages) == 0 {
			return false
		}
		return threads[i].Messages[0].Timestamp.Before(threads[j].Messages[0].Timestamp)
	})

	return threads
}

func deduplicateMessages(messages []model.Message) []model.Message {
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
