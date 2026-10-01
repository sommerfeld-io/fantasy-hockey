package observe_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/observe"
)

// scrape returns the body of GET /metrics served by ob.
func scrape(t *testing.T, ob *observe.Observer) string {
	t.Helper()
	rec := httptest.NewRecorder()
	ob.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from the metrics handler, got %d", rec.Code)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// serve runs one request through ob.Middleware around a mux with a single
// matched route, so a test controls what the pattern and status are.
func serve(ob *observe.Observer, method, target string) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello/{name}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hi"))
	})
	mux.HandleFunc("GET /teapot", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	ob.Middleware(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, target, nil))
}

func TestHandlerShouldExposeRuntimeAndProcessSeries(t *testing.T) {
	body := scrape(t, observe.New())

	for _, series := range []string{"go_goroutines", "go_memstats_alloc_bytes", "go_gc_duration_seconds", "process_cpu_seconds_total", "process_open_fds"} {
		if !strings.Contains(body, series) {
			t.Errorf("expected the metrics output to contain %q", series)
		}
	}
}

func TestMiddlewareShouldCountAMatchedRouteByPatternAndStatus(t *testing.T) {
	ob := observe.New()

	serve(ob, http.MethodGet, "/hello/world")

	body := scrape(t, ob)
	want := `fantasy_hockey_http_requests_total{route="GET /hello/{name}",status="200"} 1`
	if !strings.Contains(body, want) {
		t.Errorf("expected %q in output, got:\n%s", want, body)
	}
	wantDuration := `fantasy_hockey_http_request_duration_seconds_count{route="GET /hello/{name}",status="200"} 1`
	if !strings.Contains(body, wantDuration) {
		t.Errorf("expected %q in output", wantDuration)
	}
}

func TestMiddlewareShouldNotPutTheRawPathInAnyLabel(t *testing.T) {
	ob := observe.New()

	serve(ob, http.MethodGet, "/hello/secret-name")

	if strings.Contains(scrape(t, ob), "secret-name") {
		t.Error("expected the raw path to appear nowhere in the metrics output")
	}
}

func TestMiddlewareShouldCountAnUnknownPathAsUnmatched(t *testing.T) {
	ob := observe.New()

	serve(ob, http.MethodGet, "/no/such/path/123")

	body := scrape(t, ob)
	want := `fantasy_hockey_http_requests_total{route="unmatched",status="404"} 1`
	if !strings.Contains(body, want) {
		t.Errorf("expected %q in output, got:\n%s", want, body)
	}
	if strings.Contains(body, "/no/such/path/123") {
		t.Error("expected the raw path to appear nowhere in the metrics output")
	}
}

func TestMiddlewareShouldCountAWrongMethodAsUnmatched(t *testing.T) {
	ob := observe.New()

	serve(ob, http.MethodPut, "/teapot")

	want := `fantasy_hockey_http_requests_total{route="unmatched",status="405"} 1`
	if body := scrape(t, ob); !strings.Contains(body, want) {
		t.Errorf("expected %q in output, got:\n%s", want, body)
	}
}

func TestMiddlewareShouldRecordTheExplicitStatus(t *testing.T) {
	ob := observe.New()

	serve(ob, http.MethodGet, "/teapot")

	want := `fantasy_hockey_http_requests_total{route="GET /teapot",status="418"} 1`
	if body := scrape(t, ob); !strings.Contains(body, want) {
		t.Errorf("expected %q in output, got:\n%s", want, body)
	}
}

func TestMiddlewareShouldReadThePatternWhenAnInnerHandlerReplacesTheRequest(t *testing.T) {
	ob := observe.New()
	mux := http.NewServeMux()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusFound) })
	mux.Handle("GET /guarded", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r.WithContext(r.Context()))
	}))

	ob.Middleware(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/guarded", nil))

	want := `fantasy_hockey_http_requests_total{route="GET /guarded",status="302"} 1`
	if body := scrape(t, ob); !strings.Contains(body, want) {
		t.Errorf("expected %q in output, got:\n%s", want, body)
	}
}

func TestNewShouldNotPanicWhenCalledTwice(t *testing.T) {
	first := observe.New()
	second := observe.New()

	serve(first, http.MethodGet, "/teapot")

	if strings.Contains(scrape(t, second), "fantasy_hockey_http_requests_total{") {
		t.Error("expected the second Observer's registry to be independent of the first")
	}
}

func TestAuditShouldWriteOneInfoLineWithEventAndPlayerID(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	observe.New().Audit(observe.EventLoginSucceeded, "basti", "via", "code")

	got := buf.String()
	for _, want := range []string{"level=INFO", "msg=audit", "event=login_succeeded", "player_id=basti", "via=code"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected the audit line to contain %q, got %q", want, got)
		}
	}
	if strings.Count(got, "\n") != 1 {
		t.Errorf("expected exactly one line, got %q", got)
	}
}

func TestAuditEventsShouldUseTheFixedVocabulary(t *testing.T) {
	want := map[string]string{
		observe.EventLoginCodeRequested: "login_code_requested",
		observe.EventLoginSucceeded:     "login_succeeded",
		observe.EventLoginFailed:        "login_failed",
		observe.EventLogout:             "logout",
		observe.EventPredictionSaved:    "prediction_saved",
	}
	for got, exp := range want {
		if got != exp {
			t.Errorf("event constant = %q, want %q", got, exp)
		}
	}
}

func quietLogs(t *testing.T) {
	t.Helper()
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
}

func TestNewShouldPreRegisterEveryLoginEventAtZero(t *testing.T) {
	out := scrape(t, observe.New())
	for _, event := range []string{"login_code_requested", "login_succeeded", "login_failed", "logout"} {
		want := `fantasy_hockey_login_events_total{event="` + event + `"} 0`
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the metrics output", want)
		}
	}
}

func TestPreRegisterKindsShouldExposeEachKindAtZero(t *testing.T) {
	ob := observe.New()
	ob.PreRegisterKinds("cup", "series")
	out := scrape(t, ob)
	for _, kind := range []string{"cup", "series"} {
		want := `fantasy_hockey_prediction_saves_total{kind="` + kind + `"} 0`
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the metrics output", want)
		}
	}
}

func TestAuditShouldIncrementTheLoginEventCounter(t *testing.T) {
	quietLogs(t)
	ob := observe.New()
	ob.Audit(observe.EventLoginFailed, "")
	ob.Audit(observe.EventLoginFailed, "")
	ob.Audit(observe.EventLoginSucceeded, "basti")
	out := scrape(t, ob)
	for _, want := range []string{
		`fantasy_hockey_login_events_total{event="login_failed"} 2`,
		`fantasy_hockey_login_events_total{event="login_succeeded"} 1`,
		`fantasy_hockey_login_events_total{event="logout"} 0`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the metrics output", want)
		}
	}
}

func TestAuditShouldIncrementTheSaveCounterByKindOnly(t *testing.T) {
	quietLogs(t)
	ob := observe.New()
	ob.PreRegisterKinds("cup", "presidents")
	ob.Audit(observe.EventPredictionSaved, "basti", "kind", "cup", "set", "cup")
	out := scrape(t, ob)
	if !strings.Contains(out, `fantasy_hockey_prediction_saves_total{kind="cup"} 1`) {
		t.Errorf("expected the cup save counter at 1, got %q", out)
	}
	if !strings.Contains(out, `fantasy_hockey_prediction_saves_total{kind="presidents"} 0`) {
		t.Errorf("expected the presidents save counter to stay at 0")
	}
	if strings.Contains(out, "basti") {
		t.Errorf("expected no player id in the metrics output")
	}
}

func TestAuditShouldOmitPlayerIDWhenEmpty(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	observe.New().Audit(observe.EventLoginFailed, "")

	if strings.Contains(buf.String(), "player_id") {
		t.Errorf("expected no player_id attribute, got %q", buf.String())
	}
}

func TestAuditShouldNotCountASaveWithoutAKind(t *testing.T) {
	quietLogs(t)
	ob := observe.New()
	ob.Audit(observe.EventPredictionSaved, "basti")
	if strings.Contains(scrape(t, ob), "fantasy_hockey_prediction_saves_total{") {
		t.Errorf("expected no save series without a kind")
	}
}
