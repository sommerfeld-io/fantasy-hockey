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
	logins   *prometheus.CounterVec
	saves    *prometheus.CounterVec
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
		logins: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fantasy_hockey_login_events_total",
			Help: "Login events, by event name.",
		}, []string{"event"}),
		saves: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "fantasy_hockey_prediction_saves_total",
			Help: "Prediction rows saved, by prediction kind.",
		}, []string{"kind"}),
	}
	ob.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		ob.requests,
		ob.duration,
		ob.logins,
		ob.saves,
	)
	for _, event := range []string{EventLoginCodeRequested, EventLoginSucceeded, EventLoginFailed, EventLogout} {
		ob.logins.WithLabelValues(event).Add(0)
	}
	return ob
}

// PreRegisterKinds creates one prediction-save series per kind at zero, so
// every series is scrapeable before the first save. observe cannot import
// store, so the caller hands it the kind list.
func (o *Observer) PreRegisterKinds(kinds ...string) {
	for _, kind := range kinds {
		o.saves.WithLabelValues(kind).Add(0)
	}
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

// Audit is the single audit emitter: it writes one structured info line for
// event and increments the matching counter. The login events increment
// fantasy_hockey_login_events_total by event; prediction_saved increments
// fantasy_hockey_prediction_saves_total by the "kind" attr. playerID names the
// acting player and is omitted from the line when empty; it is never a metric
// label. attrs are extra slog key/value pairs; callers never pass an email or
// a login code.
func (o *Observer) Audit(event, playerID string, attrs ...any) {
	args := []any{"event", event}
	if playerID != "" {
		args = append(args, "player_id", playerID)
	}
	args = append(args, attrs...)
	slog.Info("audit", args...)

	switch event {
	case EventLoginCodeRequested, EventLoginSucceeded, EventLoginFailed, EventLogout:
		o.logins.WithLabelValues(event).Inc()
	case EventPredictionSaved:
		if kind, ok := attrString(attrs, "kind"); ok {
			o.saves.WithLabelValues(kind).Inc()
		}
	}
}

// attrString returns the string value following key in a slog key/value list.
func attrString(attrs []any, key string) (string, bool) {
	for i := 0; i+1 < len(attrs); i += 2 {
		if k, ok := attrs[i].(string); ok && k == key {
			v, ok := attrs[i+1].(string)
			return v, ok
		}
	}
	return "", false
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
