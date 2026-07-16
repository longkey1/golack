package collector

import (
	"time"

	"github.com/longkey1/golack/internal/model"
	"github.com/longkey1/golack/internal/slack"
)

// HistoryOptions contains options for collecting channel history for a day.
type HistoryOptions struct {
	Date       time.Time
	Channels   []slack.ChannelRef
	WithThread bool
}

// History collects every message in the given channels for a specific day.
// Unlike List, no author/mention/channel filtering is applied: the full day's
// messages are returned as-is.
func History(client *slack.Client, opts HistoryOptions) (*DayResult, error) {
	// The date is parsed as a calendar day (UTC); interpret its boundaries in
	// the local timezone so the day matches the operator's wall clock (e.g. a
	// JST 00:00-23:59 window) rather than a UTC window.
	dayStart := time.Date(opts.Date.Year(), opts.Date.Month(), opts.Date.Day(), 0, 0, 0, 0, time.Local)
	dayEnd := dayStart.AddDate(0, 0, 1).Add(-time.Second)

	var messages []model.Message
	for _, ch := range opts.Channels {
		msgs, err := client.GetChannelHistory(ch.ID, ch.Name, dayStart, dayEnd)
		if err != nil {
			return &DayResult{Date: opts.Date, Error: err}, err
		}
		messages = append(messages, msgs...)
	}

	// If thread option is enabled, expand each thread to include its replies.
	if opts.WithThread {
		var err error
		messages, err = fetchThreads(client, messages)
		if err != nil {
			return &DayResult{Date: opts.Date, Error: err}, err
		}
	}

	threads := groupByThread(messages)

	return &DayResult{
		Date:     opts.Date,
		Threads:  threads,
		Messages: messages,
	}, nil
}
