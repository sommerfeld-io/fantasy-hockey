// Package clock provides the current date and time for the application, and
// display helpers for the RFC3339 UTC timestamps the store persists (every
// stored timestamp is UTC; conversion to Europe/Berlin happens only here,
// for display).
package clock

import (
	"fmt"
	"math"
	"time"

	// Embeds the IANA time zone database into the binary so
	// time.LoadLocation("Europe/Berlin") resolves without depending on the
	// host having zoneinfo files on disk - the run image (alpine, no
	// tzdata package) ships none.
	_ "time/tzdata"
)

// displayZone is the fixed zone every deadline is converted to for display -
// timestamps are always stored as RFC3339 UTC (Boundaries & Constraints of
// this story), so this is the single place that conversion happens.
var displayZone = mustLoadLocation("Europe/Berlin")

// mustLoadLocation loads name or panics. A failure here can only come from
// the embedded tzdata (imported above) being missing or corrupt - a
// build-time programmer error, never something runtime input can trigger.
func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("clock: load location %s: %v", name, err))
	}
	return loc
}

// NowTime returns the current date and time in UTC as a time.Time, for
// callers that need to store or compare timestamps rather than print one.
func NowTime() time.Time {
	return time.Now().UTC()
}

// FormatDeadline renders deadline in Europe/Berlin for display, e.g. "Tue,
// Oct 6, 2026, 7:00 PM". deadline is expected to already carry a UTC
// location (as every stored timestamp does); FormatDeadline converts it, it
// never assumes the caller already did.
func FormatDeadline(deadline time.Time) string {
	return deadline.In(displayZone).Format("Mon, Jan 2, 2006, 3:04 PM")
}

// Countdown returns a short, relative-time phrase describing deadline as
// seen from now: "closed" once the deadline has passed (now at or after
// deadline), "today"/"tomorrow" for the two nearest whole days out, and
// "in N days" beyond that. Days are rounded from the raw duration (not
// calendar-day boundaries), matching the UX click-dummy this story's seeded
// Prediction Sets are drawn from.
func Countdown(deadline, now time.Time) string {
	remaining := deadline.Sub(now)
	if remaining <= 0 {
		return "closed"
	}

	days := int(math.Round(remaining.Hours() / 24))
	switch days {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	default:
		return fmt.Sprintf("in %d days", days)
	}
}
