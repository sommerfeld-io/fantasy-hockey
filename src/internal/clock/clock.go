// Package clock provides the current date and time for the application.
package clock

import "time"

// Now returns the current date and time formatted as RFC 3339.
func Now() string {
	return time.Now().Format(time.RFC3339)
}
