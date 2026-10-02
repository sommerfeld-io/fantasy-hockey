package clock

import (
	"testing"
	"time"
)

func TestNowTimeShouldReturnCurrentTimeInUTC(t *testing.T) {
	before := time.Now().UTC()
	got := NowTime()
	after := time.Now().UTC()

	if got.Location() != time.UTC {
		t.Errorf("expected NowTime() to return a UTC time, got location %v", got.Location())
	}

	if got.Before(before.Add(-time.Second)) || got.After(after.Add(time.Second)) {
		t.Errorf("expected NowTime() to return a timestamp close to the current time, got %v", got)
	}
}

func TestFormatDeadlineShouldConvertToEuropeBerlinForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		utc      time.Time
		expected string
	}{
		// 2026-10-06T17:00:00Z is CEST (UTC+2) in Europe/Berlin.
		{"CEST (summer time)", time.Date(2026, 10, 6, 17, 0, 0, 0, time.UTC), "Tue, Oct 6, 2026, 7:00 PM"},
		// 2026-12-24T17:00:00Z is CET (UTC+1) in Europe/Berlin.
		{"CET (winter time)", time.Date(2026, 12, 24, 17, 0, 0, 0, time.UTC), "Thu, Dec 24, 2026, 6:00 PM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDeadline(tt.utc); got != tt.expected {
				t.Errorf("FormatDeadline(%v) = %q, want %q", tt.utc, got, tt.expected)
			}
		})
	}
}

func TestFormatDeadlineShouldNotMutateAUTCInputInPlace(t *testing.T) {
	utc := time.Date(2026, 10, 6, 17, 0, 0, 0, time.UTC)

	FormatDeadline(utc)

	if utc.Location() != time.UTC {
		t.Errorf("expected the input's location to stay UTC, got %v", utc.Location())
	}
}

func TestCountdownShouldCoverEveryIOMatrixRow(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		deadline time.Time
		expected string
	}{
		{"5 days out", now.Add(5 * 24 * time.Hour), "in 5 days"},
		{"later today", now.Add(3 * time.Hour), "today"},
		{"tomorrow", now.Add(24 * time.Hour), "tomorrow"},
		{"already past", now.Add(-1 * time.Hour), "closed"},
		{"exactly now", now, "closed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Countdown(tt.deadline, now); got != tt.expected {
				t.Errorf("Countdown(%v, %v) = %q, want %q", tt.deadline, now, got, tt.expected)
			}
		})
	}
}

func TestCountdownShouldNotReadTodayForADeadlineThatHasAlreadyPassedToday(t *testing.T) {
	now := time.Date(2026, 9, 15, 20, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)

	if got := Countdown(deadline, now); got != "closed" {
		t.Errorf("expected a same-day but already-past deadline to read \"closed\", got %q", got)
	}
}
