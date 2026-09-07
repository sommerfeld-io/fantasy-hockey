// Package clock provides the current date and time for the application.
package clock

import "time"

// NowTime returns the current date and time in UTC as a time.Time, for
// callers that need to store or compare timestamps rather than print one.
func NowTime() time.Time {
	return time.Now().UTC()
}
