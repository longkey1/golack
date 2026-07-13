package collector

import (
	"testing"
	"time"

	"github.com/longkey1/gosla/internal/model"
)

func TestGroupByThread(t *testing.T) {
	t.Parallel()

	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		messages      []model.Message
		wantThreadIDs []string // in expected sorted order
		wantMsgIDs    map[string][]string
	}{
		{
			name:          "empty input",
			messages:      nil,
			wantThreadIDs: nil,
		},
		{
			name: "messages grouped by thread ts",
			messages: []model.Message{
				{ID: "1.0", ThreadTS: "1.0", Timestamp: base},
				{ID: "1.1", ThreadTS: "1.0", Timestamp: base.Add(time.Minute)},
				{ID: "2.0", ThreadTS: "2.0", Timestamp: base.Add(time.Hour)},
			},
			wantThreadIDs: []string{"1.0", "2.0"},
			wantMsgIDs: map[string][]string{
				"1.0": {"1.0", "1.1"},
				"2.0": {"2.0"},
			},
		},
		{
			name: "empty thread ts falls back to message id",
			messages: []model.Message{
				{ID: "3.0", ThreadTS: "", Timestamp: base},
			},
			wantThreadIDs: []string{"3.0"},
			wantMsgIDs: map[string][]string{
				"3.0": {"3.0"},
			},
		},
		{
			name: "messages within a thread sorted by timestamp",
			messages: []model.Message{
				{ID: "1.2", ThreadTS: "1.0", Timestamp: base.Add(2 * time.Minute)},
				{ID: "1.0", ThreadTS: "1.0", Timestamp: base},
				{ID: "1.1", ThreadTS: "1.0", Timestamp: base.Add(time.Minute)},
			},
			wantThreadIDs: []string{"1.0"},
			wantMsgIDs: map[string][]string{
				"1.0": {"1.0", "1.1", "1.2"},
			},
		},
		{
			name: "threads sorted by first message timestamp",
			messages: []model.Message{
				{ID: "9.0", ThreadTS: "9.0", Timestamp: base.Add(3 * time.Hour)},
				{ID: "1.0", ThreadTS: "1.0", Timestamp: base},
			},
			wantThreadIDs: []string{"1.0", "9.0"},
			wantMsgIDs: map[string][]string{
				"1.0": {"1.0"},
				"9.0": {"9.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			threads := groupByThread(tt.messages)

			if len(threads) != len(tt.wantThreadIDs) {
				t.Fatalf("groupByThread() returned %d threads, want %d", len(threads), len(tt.wantThreadIDs))
			}
			for i, wantID := range tt.wantThreadIDs {
				got := threads[i]
				if got.ThreadID != wantID {
					t.Errorf("threads[%d].ThreadID = %q, want %q", i, got.ThreadID, wantID)
					continue
				}
				wantMsgs := tt.wantMsgIDs[wantID]
				if len(got.Messages) != len(wantMsgs) {
					t.Errorf("thread %q has %d messages, want %d", wantID, len(got.Messages), len(wantMsgs))
					continue
				}
				for j, id := range wantMsgs {
					if got.Messages[j].ID != id {
						t.Errorf("thread %q Messages[%d].ID = %q, want %q", wantID, j, got.Messages[j].ID, id)
					}
				}
				if got.ThreadCount != len(wantMsgs) {
					t.Errorf("thread %q ThreadCount = %d, want %d", wantID, got.ThreadCount, len(wantMsgs))
				}
			}
		})
	}
}

func TestGroupByThreadChannelInfo(t *testing.T) {
	t.Parallel()

	msgs := []model.Message{
		{ID: "1.0", ThreadTS: "1.0", Channel: "general", ChannelID: "C123"},
	}
	threads := groupByThread(msgs)
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	if threads[0].Channel != "general" || threads[0].ChannelID != "C123" {
		t.Errorf("thread channel info = (%q, %q), want (%q, %q)",
			threads[0].Channel, threads[0].ChannelID, "general", "C123")
	}
}

func TestDeduplicateMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []model.Message
		wantIDs  []string
	}{
		{
			name:     "empty input",
			messages: nil,
			wantIDs:  nil,
		},
		{
			name: "no duplicates preserves order",
			messages: []model.Message{
				{ID: "a"}, {ID: "b"}, {ID: "c"},
			},
			wantIDs: []string{"a", "b", "c"},
		},
		{
			name: "duplicates keep first occurrence",
			messages: []model.Message{
				{ID: "a", Content: "first"},
				{ID: "b"},
				{ID: "a", Content: "second"},
			},
			wantIDs: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := deduplicateMessages(tt.messages)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("deduplicateMessages() returned %d messages, want %d", len(got), len(tt.wantIDs))
			}
			for i, id := range tt.wantIDs {
				if got[i].ID != id {
					t.Errorf("result[%d].ID = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestDeduplicateMessagesKeepsFirst(t *testing.T) {
	t.Parallel()

	got := deduplicateMessages([]model.Message{
		{ID: "a", Content: "first"},
		{ID: "a", Content: "second"},
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	if got[0].Content != "first" {
		t.Errorf("kept Content = %q, want %q (first occurrence wins)", got[0].Content, "first")
	}
}
