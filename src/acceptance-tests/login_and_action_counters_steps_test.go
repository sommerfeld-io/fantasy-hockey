package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/observe"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

const (
	counterPlayerID   = "basti"
	counterPlayerMail = "basti@example.com"
	counterCodeBudget = 2 * time.Second
)

// counterSeedTemplate is the data file every action-counter scenario starts
// from; %s is the cup set's deadline.
const counterSeedTemplate = `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
prediction_sets:
    - id: "cup"
      title: "Cup champion"
      subtitle: "Your Stanley Cup winner"
      deadline_utc: %q
      phase: "before_season"
      upcoming: false
teams:
    - id: "TOR"
      name: "Toronto Maple Leafs"
      conference: "Eastern"
      division: "Atlantic"
    - id: "VGK"
      name: "Vegas Golden Knights"
      conference: "Western"
      division: "Pacific"
`

var counterLineRE = regexp.MustCompile(`(?m)^(fantasy_hockey_(?:login_events_total|prediction_saves_total))\{([^}]*)\} (\S+)$`)

// actionCountersScenarioState holds the server and the last /metrics body
// for one login-and-action-counters scenario.
type actionCountersScenarioState struct {
	server   *httptest.Server
	dataFile string
	client   *http.Client
	metrics  string

	mu        sync.Mutex
	emailBody string
}

func newActionCountersScenarioState() *actionCountersScenarioState {
	return &actionCountersScenarioState{
		client: &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *actionCountersScenarioState) close() {
	if s.server != nil {
		s.server.Close()
	}
	if s.dataFile != "" {
		removeScenarioDataFile(s.dataFile)
	}
}

func (s *actionCountersScenarioState) start(deadline time.Time) error {
	st, dataFile := newSeededStore("action-counters", fmt.Sprintf(counterSeedTemplate, deadline.UTC().Format(time.RFC3339)))
	s.dataFile = dataFile
	send := func(_, _, body string) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.emailBody = body
		return nil
	}
	s.server = httptest.NewServer(web.NewServer(st, send, testSessionSecret, observe.New()))
	return nil
}

func (s *actionCountersScenarioState) aServerWithAnOpenCupSet() error {
	return s.start(time.Now().Add(5 * 24 * time.Hour))
}

func (s *actionCountersScenarioState) aServerWithAClosedCupSet() error {
	return s.start(time.Now().Add(-24 * time.Hour))
}

func (s *actionCountersScenarioState) do(method, path string, form url.Values, cookie *http.Cookie) (*http.Response, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, s.server.URL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build %s %s: %w", method, path, err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	_ = resp.Body.Close()
	return resp, nil
}

func (s *actionCountersScenarioState) post(path string, form url.Values, cookie *http.Cookie) error {
	_, err := s.do(http.MethodPost, path, form, cookie)
	return err
}

func (s *actionCountersScenarioState) aVisitorSubmitsTheWrongLoginCode() error {
	return s.post("/login/code", url.Values{"code": {"000000"}}, nil)
}

func (s *actionCountersScenarioState) aVisitorRequestsALoginCodeFor(email string) error {
	return s.post("/login", url.Values{"email": {email}}, nil)
}

func (s *actionCountersScenarioState) theSeededPlayerRequestsALoginCode() error {
	return s.aVisitorRequestsALoginCodeFor(counterPlayerMail)
}

func (s *actionCountersScenarioState) issuedCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return extractSixDigitCode(s.emailBody)
}

func (s *actionCountersScenarioState) theSeededPlayerSubmitsTheIssuedLoginCode() error {
	deadline := time.Now().Add(counterCodeBudget)
	for s.issuedCode() == "" {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for the emailed login code")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return s.post("/login/code", url.Values{"code": {s.issuedCode()}}, nil)
}

func (s *actionCountersScenarioState) theSignedInPlayerLogsOut() error {
	return s.post("/logout", nil, auth.IssueSessionCookie(counterPlayerID, testSessionSecret))
}

func (s *actionCountersScenarioState) aVisitorWithoutASessionLogsOut() error {
	return s.post("/logout", nil, nil)
}

func (s *actionCountersScenarioState) theSignedInPlayerSavesTheCupPick(teamID string) error {
	return s.post("/predict/cup", url.Values{"team_id": {teamID}}, auth.IssueSessionCookie(counterPlayerID, testSessionSecret))
}

func (s *actionCountersScenarioState) theMetricsAreRead() error {
	resp, err := http.Get(s.server.URL + "/metrics")
	if err != nil {
		return fmt.Errorf("GET /metrics: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read /metrics: %w", err)
	}
	s.metrics = string(body)
	return nil
}

func (s *actionCountersScenarioState) requireCounter(metric, label, value, want string) error {
	line := fmt.Sprintf("%s{%s=%q} %s", metric, label, value, want)
	if !strings.Contains(s.metrics, line+"\n") {
		return fmt.Errorf("expected the metrics output to contain %q", line)
	}
	return nil
}

func (s *actionCountersScenarioState) theLoginCounterReads(event, want string) error {
	return s.requireCounter("fantasy_hockey_login_events_total", "event", event, want)
}

func (s *actionCountersScenarioState) theSaveCounterReads(kind, want string) error {
	return s.requireCounter("fantasy_hockey_prediction_saves_total", "kind", kind, want)
}

func (s *actionCountersScenarioState) theCounterSeriesAreLabelledOnlyByEventOrKind() error {
	matches := counterLineRE.FindAllStringSubmatch(s.metrics, -1)
	if len(matches) == 0 {
		return fmt.Errorf("no login or save counter series found")
	}
	for _, m := range matches {
		wantLabel := "event"
		if strings.Contains(m[1], "prediction_saves") {
			wantLabel = "kind"
		}
		if !regexp.MustCompile(`^` + wantLabel + `="[a-z_]+"$`).MatchString(m[2]) {
			return fmt.Errorf("series %s has unexpected labels %q", m[1], m[2])
		}
		for _, forbidden := range []string{counterPlayerID, counterPlayerMail, "predict"} {
			if strings.Contains(m[2], forbidden) {
				return fmt.Errorf("series %s label %q contains %q", m[1], m[2], forbidden)
			}
		}
	}
	return s.metricsHoldNoIdentifiers()
}

// metricsHoldNoIdentifiers fails if the whole /metrics body contains the
// Player id, the Player email or the issued login code.
func (s *actionCountersScenarioState) metricsHoldNoIdentifiers() error {
	for _, forbidden := range []string{counterPlayerID, counterPlayerMail} {
		if strings.Contains(s.metrics, forbidden) {
			return fmt.Errorf("the metrics output contains %q", forbidden)
		}
	}
	if code := s.issuedCode(); code != "" && strings.Contains(s.metrics, code) {
		return fmt.Errorf("the metrics output contains the issued login code")
	}
	return nil
}

// InitializeLoginAndActionCountersScenario registers the login-and-action-
// counters step definitions.
func InitializeLoginAndActionCountersScenario(ctx *godog.ScenarioContext) {
	s := newActionCountersScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a server for the action counters with an open cup set$`, s.aServerWithAnOpenCupSet)
	ctx.Step(`^a server for the action counters with a closed cup set$`, s.aServerWithAClosedCupSet)
	ctx.Step(`^a visitor submits the wrong login code$`, s.aVisitorSubmitsTheWrongLoginCode)
	ctx.Step(`^a visitor asks for a login code for "([^"]*)"$`, s.aVisitorRequestsALoginCodeFor)
	ctx.Step(`^the seeded player requests a login code$`, s.theSeededPlayerRequestsALoginCode)
	ctx.Step(`^the seeded player submits the issued login code$`, s.theSeededPlayerSubmitsTheIssuedLoginCode)
	ctx.Step(`^the signed-in player logs out$`, s.theSignedInPlayerLogsOut)
	ctx.Step(`^a visitor without a session logs out$`, s.aVisitorWithoutASessionLogsOut)
	ctx.Step(`^the signed-in player saves the cup pick "([^"]*)"$`, s.theSignedInPlayerSavesTheCupPick)
	ctx.Step(`^the metrics are read$`, s.theMetricsAreRead)
	ctx.Step(`^the login counter "([^"]*)" reads (\d+)$`, s.theLoginCounterReads)
	ctx.Step(`^the save counter "([^"]*)" reads (\d+)$`, s.theSaveCounterReads)
	ctx.Step(`^the login and save counter series are labelled only by event or kind$`, s.theCounterSeriesAreLabelledOnlyByEventOrKind)
}
