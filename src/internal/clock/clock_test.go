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
