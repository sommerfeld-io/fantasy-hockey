// Package observe is the one home of the app's observability: a dedicated
// Prometheus registry with Go and process collectors, HTTP request metrics,
// the /metrics handler and the audit-log emitter. It is consumed only by
// internal/web and main.go (AD-31, AD-32, AD-33, AD-36).
package observe

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Audit event names. The vocabulary is fixed: every audit line uses one of
// these.
const (
	EventLoginCodeRequested = "login_code_requested"
	EventLoginSucceeded     = "login_succeeded"
	EventLoginFailed        = "login_failed"
	EventLogout             = "logout"
	EventPredictionSaved    = "prediction_saved"
)

// unmatchedRoute is the route label for a request no pattern matched, so a
// raw path can never become a label value.
const unmatchedRoute = "unmatched"

// Observer owns the metrics registry and the audit emitter.
type Observer struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// New builds an Observer with its own registry (never the default
// registerer, so any number of Observers can coexist in one process).
func New() *Observer {
	labels := []string{"route", "status"}
	ob := &Observer{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fantasy_hockey_http_requests_total",
			Help: "HTTP requests served, by route pattern and status code.",
		}, labels),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "fantasy_hockey_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds, by route pattern and status code.",
			Buckets: prometheus.DefBuckets,
		}, labels),
	}
	ob.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		ob.requests,
		ob.duration,
	)
	return ob
}

// Handler serves the registry in the Prometheus text format.
func (o *Observer) Handler() http.Handler {
	return promhttp.HandlerFor(o.registry, promhttp.HandlerOpts{})
}

// Middleware records every request served by next. The route label is read
// from the original request after next returns, since inner handlers may
// replace the request (WithContext) and the mux sets Pattern on the one it
// was handed.
func (o *Observer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = unmatchedRoute
		}
		status := strconv.Itoa(rec.status)
		o.requests.WithLabelValues(route, status).Inc()
		o.duration.WithLabelValues(route, status).Observe(time.Since(start).Seconds())
	})
}

// Audit writes one structured info line for event, naming the acting player
// by id. attrs are extra slog key/value pairs. It is the single audit
// emitter, so counters can later be attached to it without changing callers.
func (o *Observer) Audit(event, playerID string, attrs ...any) {
	args := append([]any{"event", event, "player_id", playerID}, attrs...)
	slog.Info("audit", args...)
}

// statusRecorder remembers the status code written to the response.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
