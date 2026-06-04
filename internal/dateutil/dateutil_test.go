package dateutil

import (
	"testing"
	"time"
)

func TestParseDay(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "valid date",
			input:   "2025-01-15",
			want:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "01-15-2025",
			wantErr: true,
		},
		{
			name:    "invalid date",
			input:   "2025-13-01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDay(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDay() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseDay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMonth(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "january",
			input:     "2025-01",
			wantStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:      "february non-leap year",
			input:     "2025-02",
			wantStart: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:    "invalid format",
			input:   "2025/01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMonth(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMonth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !got.Start.Equal(tt.wantStart) {
					t.Errorf("ParseMonth() Start = %v, want %v", got.Start, tt.wantStart)
				}
				if !got.End.Equal(tt.wantEnd) {
					t.Errorf("ParseMonth() End = %v, want %v", got.End, tt.wantEnd)
				}
			}
		})
	}
}

func TestDateRange_Days(t *testing.T) {
	dr := DateRange{
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC),
	}

	days := dr.Days()
	if len(days) != 3 {
		t.Errorf("Days() returned %d days, want 3", len(days))
	}

	expected := []time.Time{
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC),
	}

	for i, day := range days {
		if !day.Equal(expected[i]) {
			t.Errorf("Days()[%d] = %v, want %v", i, day, expected[i])
		}
	}
}
