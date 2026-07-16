package collector

import (
	"sort"

	"github.com/longkey1/golack/internal/model"
)

// MergeOptions specifies options for merging threads
type MergeOptions struct {
	Threads []model.Thread
}

// MergeResult contains the merged threads and statistics
type MergeResult struct {
	Threads              []model.Thread
	OriginalThreadCount  int
	MergedThreadCount    int
	OriginalMessageCount int
	MergedMessageCount   int
	DuplicateThreads     int
	DuplicateMessages    int
}

// Merge merges multiple threads, deduplicating by ThreadID and Message ID
func Merge(opts MergeOptions) *MergeResult {
	result := &MergeResult{}

	// Count original threads and messages
	for _, t := range opts.Threads {
		result.OriginalThreadCount++
		result.OriginalMessageCount += len(t.Messages)
	}

	// Merge threads by ThreadID
	mergedThreads := mergeThreads(opts.Threads)

	// Deduplicate messages within each thread
	for i := range mergedThreads {
		mergedThreads[i].Messages = deduplicateMessagesKeepLatest(mergedThreads[i].Messages)
		mergedThreads[i].MessageCount = len(mergedThreads[i].Messages)
	}

	// Sort threads by the first message's timestamp
	sort.Slice(mergedThreads, func(i, j int) bool {
		if len(mergedThreads[i].Messages) == 0 {
			return true
		}
		if len(mergedThreads[j].Messages) == 0 {
			return false
		}
		return mergedThreads[i].Messages[0].Timestamp.Before(mergedThreads[j].Messages[0].Timestamp)
	})

	// Count merged threads and messages
	for _, t := range mergedThreads {
		result.MergedThreadCount++
		result.MergedMessageCount += len(t.Messages)
	}

	result.DuplicateThreads = result.OriginalThreadCount - result.MergedThreadCount
	result.DuplicateMessages = result.OriginalMessageCount - result.MergedMessageCount
	result.Threads = mergedThreads

	return result
}

// mergeThreads groups threads by ThreadID and merges their messages
func mergeThreads(threads []model.Thread) []model.Thread {
	threadMap := make(map[string]*model.Thread)

	for _, t := range threads {
		if existing, ok := threadMap[t.ThreadID]; ok {
			// Merge messages
			existing.Messages = append(existing.Messages, t.Messages...)
			// Update counts
			if t.ThreadCount > existing.ThreadCount {
				existing.ThreadCount = t.ThreadCount
			}
		} else {
			// Create a copy to avoid modifying the original
			threadCopy := model.Thread{
				ThreadID:        t.ThreadID,
				ThreadPermalink: t.ThreadPermalink,
				Channel:         t.Channel,
				ChannelID:       t.ChannelID,
				Messages:        make([]model.Message, len(t.Messages)),
				MessageCount:    t.MessageCount,
				ThreadCount:     t.ThreadCount,
			}
			copy(threadCopy.Messages, t.Messages)
			threadMap[t.ThreadID] = &threadCopy
		}
	}

	// Convert map to slice
	result := make([]model.Thread, 0, len(threadMap))
	for _, t := range threadMap {
		result = append(result, *t)
	}

	return result
}

// deduplicateMessagesKeepLatest deduplicates messages by ID, keeping the one with the latest timestamp
func deduplicateMessagesKeepLatest(messages []model.Message) []model.Message {
	messageMap := make(map[string]model.Message)

	for _, m := range messages {
		if existing, ok := messageMap[m.ID]; ok {
			// Keep the one with the latest timestamp
			if m.Timestamp.After(existing.Timestamp) {
				messageMap[m.ID] = m
			}
		} else {
			messageMap[m.ID] = m
		}
	}

	// Convert map to slice
	result := make([]model.Message, 0, len(messageMap))
	for _, m := range messageMap {
		result = append(result, m)
	}

	// Sort by timestamp
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}
