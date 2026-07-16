package collector

import (
	"testing"
	"time"

	"github.com/longkey1/golack/internal/model"
)

// msg builds a message with the fields relevant to merging.
func msg(id, content string, ts time.Time) model.Message {
	return model.Message{ID: id, Content: content, Timestamp: ts}
}

func TestMerge(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name                 string
		threads              []model.Thread
		wantThreadIDs        []string // in expected sorted order
		wantMessagesByThread map[string][]string
		wantDuplicateThreads int
		wantDuplicateMsgs    int
	}{
		{
			name: "no duplicates",
			threads: []model.Thread{
				{ThreadID: "t1", Messages: []model.Message{msg("m1", "a", base)}},
				{ThreadID: "t2", Messages: []model.Message{msg("m2", "b", base.Add(time.Hour))}},
			},
			wantThreadIDs: []string{"t1", "t2"},
			wantMessagesByThread: map[string][]string{
				"t1": {"m1"},
				"t2": {"m2"},
			},
		},
		{
			name: "duplicate threads are merged",
			threads: []model.Thread{
				{ThreadID: "t1", Messages: []model.Message{msg("m1", "a", base)}},
				{ThreadID: "t1", Messages: []model.Message{msg("m2", "b", base.Add(time.Minute))}},
			},
			wantThreadIDs: []string{"t1"},
			wantMessagesByThread: map[string][]string{
				"t1": {"m1", "m2"},
			},
			wantDuplicateThreads: 1,
		},
		{
			name: "duplicate messages keep the latest timestamp",
			threads: []model.Thread{
				{ThreadID: "t1", Messages: []model.Message{msg("m1", "old", base)}},
				{ThreadID: "t1", Messages: []model.Message{msg("m1", "new", base.Add(time.Hour))}},
			},
			wantThreadIDs: []string{"t1"},
			wantMessagesByThread: map[string][]string{
				"t1": {"m1"},
			},
			wantDuplicateThreads: 1,
			wantDuplicateMsgs:    1,
		},
		{
			name: "threads sorted by first message timestamp",
			threads: []model.Thread{
				{ThreadID: "late", Messages: []model.Message{msg("m2", "b", base.Add(2*time.Hour))}},
				{ThreadID: "early", Messages: []model.Message{msg("m1", "a", base)}},
			},
			wantThreadIDs: []string{"early", "late"},
			wantMessagesByThread: map[string][]string{
				"early": {"m1"},
				"late":  {"m2"},
			},
		},
		{
			name: "messages within a merged thread sorted by timestamp",
			threads: []model.Thread{
				{ThreadID: "t1", Messages: []model.Message{msg("m2", "second", base.Add(time.Minute))}},
				{ThreadID: "t1", Messages: []model.Message{msg("m1", "first", base)}},
			},
			wantThreadIDs: []string{"t1"},
			wantMessagesByThread: map[string][]string{
				"t1": {"m1", "m2"},
			},
			wantDuplicateThreads: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := Merge(MergeOptions{Threads: tt.threads})

			if len(result.Threads) != len(tt.wantThreadIDs) {
				t.Fatalf("Merge() returned %d threads, want %d", len(result.Threads), len(tt.wantThreadIDs))
			}
			for i, want := range tt.wantThreadIDs {
				got := result.Threads[i]
				if got.ThreadID != want {
					t.Errorf("Threads[%d].ThreadID = %q, want %q", i, got.ThreadID, want)
					continue
				}
				wantMsgs := tt.wantMessagesByThread[want]
				if len(got.Messages) != len(wantMsgs) {
					t.Errorf("thread %q has %d messages, want %d", want, len(got.Messages), len(wantMsgs))
					continue
				}
				for j, wantID := range wantMsgs {
					if got.Messages[j].ID != wantID {
						t.Errorf("thread %q Messages[%d].ID = %q, want %q", want, j, got.Messages[j].ID, wantID)
					}
				}
				if got.MessageCount != len(wantMsgs) {
					t.Errorf("thread %q MessageCount = %d, want %d", want, got.MessageCount, len(wantMsgs))
				}
			}

			if result.OriginalThreadCount != len(tt.threads) {
				t.Errorf("OriginalThreadCount = %d, want %d", result.OriginalThreadCount, len(tt.threads))
			}
			if result.DuplicateThreads != tt.wantDuplicateThreads {
				t.Errorf("DuplicateThreads = %d, want %d", result.DuplicateThreads, tt.wantDuplicateThreads)
			}
			if result.DuplicateMessages != tt.wantDuplicateMsgs {
				t.Errorf("DuplicateMessages = %d, want %d", result.DuplicateMessages, tt.wantDuplicateMsgs)
			}
		})
	}
}

func TestMergeKeepsLatestMessageContent(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	result := Merge(MergeOptions{Threads: []model.Thread{
		{ThreadID: "t1", Messages: []model.Message{msg("m1", "new", base.Add(time.Hour))}},
		{ThreadID: "t1", Messages: []model.Message{msg("m1", "old", base)}},
	}})

	if len(result.Threads) != 1 || len(result.Threads[0].Messages) != 1 {
		t.Fatalf("unexpected result shape: %+v", result.Threads)
	}
	if got := result.Threads[0].Messages[0].Content; got != "new" {
		t.Errorf("kept message Content = %q, want %q (latest timestamp wins)", got, "new")
	}
}

func TestMergeDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	original := []model.Thread{
		{ThreadID: "t1", Messages: []model.Message{msg("m1", "a", base)}},
		{ThreadID: "t1", Messages: []model.Message{msg("m2", "b", base.Add(time.Minute))}},
	}

	Merge(MergeOptions{Threads: original})

	if len(original[0].Messages) != 1 {
		t.Errorf("input thread messages were mutated: got %d messages, want 1", len(original[0].Messages))
	}
	if original[0].Messages[0].ID != "m1" {
		t.Errorf("input message was mutated: ID = %q, want %q", original[0].Messages[0].ID, "m1")
	}
}
