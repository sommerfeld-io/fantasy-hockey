package clock

import (
	"testing"
	"time"
)

func TestNowShouldReturnParsableRFC3339Timestamp(t *testing.T) {
	got := Now()

	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Errorf("expected Now() to return a valid RFC 3339 timestamp, got %q: %v", got, err)
	}
}

func TestNowShouldNotReturnEmptyString(t *testing.T) {
	got := Now()

	if got == "" {
		t.Error("expected Now() to return a non-empty timestamp")
	}
}

func TestNowShouldReturnCurrentTime(t *testing.T) {
	before := time.Now()
	got := Now()
	after := time.Now()

	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("expected Now() to return a valid RFC 3339 timestamp, got %q: %v", got, err)
	}

	if parsed.Before(before.Add(-time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("expected Now() to return a timestamp close to the current time, got %q", got)
	}
}
