// Package web is the presentation layer: it serves the home page.
package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
)

// NewServer wires the home page route and returns an http.Handler ready to
// be served.
func NewServer() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleHome)
	return mux
}

// handleHome renders the application name and the current date and time.
func handleHome(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	body := fmt.Sprintf("<h1>Fantasy Hockey</h1>\n<p>%s</p>\n", clock.NowTime().Format(time.RFC1123))
	if _, err := w.Write([]byte(body)); err != nil {
		slog.Error("write home page response", "error", err)
	}
}
