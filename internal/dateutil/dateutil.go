package dateutil

import (
	"fmt"
	"time"
)

// DateRange represents a range of dates
type DateRange struct {
	Start time.Time
	End   time.Time
}

// ParseDay parses a date string in YYYY-MM-DD format
func ParseDay(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format (expected YYYY-MM-DD): %s", s)
	}
	return t, nil
}

// ParseMonth parses a month string in YYYY-MM format and returns the date range
func ParseMonth(s string) (DateRange, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return DateRange{}, fmt.Errorf("invalid month format (expected YYYY-MM): %s", s)
	}

	start := t
	end := t.AddDate(0, 1, -1) // Last day of the month

	return DateRange{Start: start, End: end}, nil
}

// DayRange returns a DateRange for a single day
func DayRange(day time.Time) DateRange {
	return DateRange{
		Start: day,
		End:   day,
	}
}

// CustomRange creates a DateRange from two date strings
func CustomRange(from, to string) (DateRange, error) {
	start, err := ParseDay(from)
	if err != nil {
		return DateRange{}, fmt.Errorf("invalid from date: %w", err)
	}

	end, err := ParseDay(to)
	if err != nil {
		return DateRange{}, fmt.Errorf("invalid to date: %w", err)
	}

	if end.Before(start) {
		return DateRange{}, fmt.Errorf("end date must be after start date")
	}

	return DateRange{Start: start, End: end}, nil
}

// Days returns all days in the range
func (dr DateRange) Days() []time.Time {
	var days []time.Time
	current := dr.Start
	for !current.After(dr.End) {
		days = append(days, current)
		current = current.AddDate(0, 0, 1)
	}
	return days
}

// FormatDate formats a time as YYYY-MM-DD
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}
