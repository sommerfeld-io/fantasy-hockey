package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/observe"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

const (
	metricsPath           = "/metrics"
	metricsPlayerEmail    = "basti@example.com"
	prometheusTextType    = "text/plain"
	metricsCodeWaitBudget = 2 * time.Second
)

// metricsScenarioState holds the server, the last /metrics response and the
// emailed login code for one metrics-endpoint scenario. A fresh instance is
// created per scenario so state never leaks between runs.
type metricsScenarioState struct {
	server      *httptest.Server
	client      *http.Client
	status      int
	contentType string
	body        string

	mu        sync.Mutex
	emailBody string
}

func newMetricsScenarioState() *metricsScenarioState {
	s := &metricsScenarioState{
		client: &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	send := func(_, _, body string) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.emailBody = body
		return nil
	}
	s.server = httptest.NewServer(web.NewServer(newAppShellStore(), send, testSessionSecret, observe.New()))
	return s
}

func (s *metricsScenarioState) close() {
	s.server.Close()
}

func (s *metricsScenarioState) fetch(method, path string) error {
	req, err := http.NewRequest(method, s.server.URL+path, nil)
	if err != nil {
		return fmt.Errorf("build %s %s: %w", method, path, err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s %s: %w", method, path, err)
	}
	s.status = resp.StatusCode
	s.contentType = resp.Header.Get("Content-Type")
	s.body = string(body)
	return nil
}

func (s *metricsScenarioState) anAnonymousClientRequests(path string) error {
	return s.fetch(http.MethodGet, path)
}

func (s *metricsScenarioState) anAnonymousClientHasSentAPUTTo(path string) error {
	return s.fetch(http.MethodPut, path)
}

func (s *metricsScenarioState) theSeededPlayerHasRequestedALoginCode() error {
	resp, err := http.PostForm(s.server.URL+"/login", url.Values{"email": {metricsPlayerEmail}})
	if err != nil {
		return fmt.Errorf("POST /login: %w", err)
	}
	_ = resp.Body.Close()
	return s.waitForEmailedCode()
}

func (s *metricsScenarioState) waitForEmailedCode() error {
	deadline := time.Now().Add(metricsCodeWaitBudget)
	for time.Now().Before(deadline) {
		if s.emailedCode() != "" {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for the emailed login code")
}

func (s *metricsScenarioState) emailedCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return extractSixDigitCode(s.emailBody)
}

func (s *metricsScenarioState) theMetricsResponseStatusIs(want int) error {
	if s.status != want {
		return fmt.Errorf("expected status %d, got %d", want, s.status)
	}
	return nil
}

func (s *metricsScenarioState) theMetricsResponseIsInThePrometheusTextFormat() error {
	if !strings.HasPrefix(s.contentType, prometheusTextType) {
		return fmt.Errorf("expected a %s content type, got %q", prometheusTextType, s.contentType)
	}
	if !strings.Contains(s.body, "# TYPE ") {
		return fmt.Errorf("expected Prometheus # TYPE lines in the body")
	}
	return nil
}

func (s *metricsScenarioState) theMetricsOutputContainsTheSeries(name string) error {
	if !strings.Contains(s.body, "\n"+name) && !strings.HasPrefix(s.body, name) {
		return fmt.Errorf("expected the metrics output to contain series %q", name)
	}
	return nil
}

func (s *metricsScenarioState) theMetricsOutputContainsARequestCountFor(route, status string) error {
	return s.requireLine("fantasy_hockey_http_requests_total", route, status)
}

func (s *metricsScenarioState) theMetricsOutputContainsARequestDurationFor(route, status string) error {
	return s.requireLine("fantasy_hockey_http_request_duration_seconds_count", route, status)
}

func (s *metricsScenarioState) requireLine(metric, route, status string) error {
	want := fmt.Sprintf(`%s{route=%q,status=%q}`, metric, route, status)
	if !strings.Contains(s.body, want) {
		return fmt.Errorf("expected the metrics output to contain %s", want)
	}
	return nil
}

func (s *metricsScenarioState) theMetricsOutputDoesNotContain(text string) error {
	if strings.Contains(s.body, text) {
		return fmt.Errorf("expected the metrics output not to contain %q", text)
	}
	return nil
}

func (s *metricsScenarioState) theMetricsOutputDoesNotContainTheIssuedLoginCode() error {
	code := s.emailedCode()
	if code == "" {
		return fmt.Errorf("no login code was issued, so its absence proves nothing")
	}
	return s.theMetricsOutputDoesNotContain(code)
}

// InitializeMetricsEndpointScenario registers the metrics-endpoint step
// definitions.
func InitializeMetricsEndpointScenario(ctx *godog.ScenarioContext) {
	s := newMetricsScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^an anonymous client (?:requests|has requested) "([^"]*)"$`, s.anAnonymousClientRequests)
	ctx.Step(`^an anonymous client has sent a PUT to "([^"]*)"$`, s.anAnonymousClientHasSentAPUTTo)
	ctx.Step(`^the seeded player has requested a login code$`, s.theSeededPlayerHasRequestedALoginCode)
	ctx.Step(`^the metrics response status is (\d+)$`, s.theMetricsResponseStatusIs)
	ctx.Step(`^the metrics response is in the Prometheus text format$`, s.theMetricsResponseIsInThePrometheusTextFormat)
	ctx.Step(`^the metrics output contains the series "([^"]*)"$`, s.theMetricsOutputContainsTheSeries)
	ctx.Step(`^the metrics output contains a request count for route "([^"]*)" with status "([^"]*)"$`, s.theMetricsOutputContainsARequestCountFor)
	ctx.Step(`^the metrics output contains a request duration for route "([^"]*)" with status "([^"]*)"$`, s.theMetricsOutputContainsARequestDurationFor)
	ctx.Step(`^the metrics output does not contain "([^"]*)"$`, s.theMetricsOutputDoesNotContain)
	ctx.Step(`^the metrics output does not contain the issued login code$`, s.theMetricsOutputDoesNotContainTheIssuedLoginCode)
}
