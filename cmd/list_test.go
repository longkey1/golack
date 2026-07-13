package cmd

import (
	"testing"
	"time"
)

func TestParseDateRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		day       string
		month     string
		from      string
		to        string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "single day",
			day:       "2025-01-15",
			wantStart: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "month",
			month:     "2025-01",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "custom range",
			from:      "2025-01-01",
			to:        "2025-01-15",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "no options",
			wantErr: true,
		},
		{
			name:    "day and month are mutually exclusive",
			day:     "2025-01-15",
			month:   "2025-01",
			wantErr: true,
		},
		{
			name:    "day and from/to are mutually exclusive",
			day:     "2025-01-15",
			from:    "2025-01-01",
			to:      "2025-01-15",
			wantErr: true,
		},
		{
			name:    "from without to",
			from:    "2025-01-01",
			wantErr: true,
		},
		{
			name:    "to without from",
			to:      "2025-01-15",
			wantErr: true,
		},
		{
			name:    "invalid day format",
			day:     "01-15-2025",
			wantErr: true,
		},
		{
			name:    "invalid month format",
			month:   "2025/01",
			wantErr: true,
		},
		{
			name:    "range end before start",
			from:    "2025-01-15",
			to:      "2025-01-01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseDateRange(tt.day, tt.month, tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDateRange() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !got.Start.Equal(tt.wantStart) {
				t.Errorf("parseDateRange() Start = %v, want %v", got.Start, tt.wantStart)
			}
			if !got.End.Equal(tt.wantEnd) {
				t.Errorf("parseDateRange() End = %v, want %v", got.End, tt.wantEnd)
			}
		})
	}
}
