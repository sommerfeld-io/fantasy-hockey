package acceptance_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/observe"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

const auditMarker = "msg=audit"

// auditScenarioState holds the server and the captured slog output for one
// audit-log-lines scenario.
type auditScenarioState struct {
	server   *httptest.Server
	dataFile string
	client   *http.Client
	prev     *slog.Logger

	mu        sync.Mutex
	logs      bytes.Buffer
	emailBody string
	status    int
}

func newAuditScenarioState() *auditScenarioState {
	return &auditScenarioState{
		client: &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *auditScenarioState) close() {
	if s.server != nil {
		s.server.Close()
	}
	if s.prev != nil {
		slog.SetDefault(s.prev)
	}
	if s.dataFile != "" {
		removeScenarioDataFile(s.dataFile)
	}
}

type lockedWriter struct{ s *auditScenarioState }

func (w lockedWriter) Write(p []byte) (int, error) {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	return w.s.logs.Write(p)
}

func (s *auditScenarioState) start(deadline time.Time) error {
	st, dataFile := newSeededStore("audit-log", fmt.Sprintf(counterSeedTemplate, deadline.UTC().Format(time.RFC3339)))
	s.dataFile = dataFile
	send := func(_, _, body string) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.emailBody = body
		return nil
	}
	s.prev = slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(lockedWriter{s}, nil)))
	s.server = httptest.NewServer(web.NewServer(st, send, testSessionSecret, observe.New()))
	return nil
}

func (s *auditScenarioState) anOpenServer() error {
	return s.start(time.Now().Add(5 * 24 * time.Hour))
}

func (s *auditScenarioState) aClosedServer() error {
	return s.start(time.Now().Add(-24 * time.Hour))
}

func (s *auditScenarioState) post(path string, form url.Values, cookie *http.Cookie) error {
	req, err := http.NewRequest(http.MethodPost, s.server.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build POST %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	s.status = resp.StatusCode
	return resp.Body.Close()
}

func (s *auditScenarioState) responseStatusIs(want int) error {
	if s.status != want {
		return fmt.Errorf("expected response status %d, got %d", want, s.status)
	}
	return nil
}

func (s *auditScenarioState) issuedCode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return extractSixDigitCode(s.emailBody)
}

func (s *auditScenarioState) requestCodeFor(email string) error {
	return s.post("/login", url.Values{"email": {email}}, nil)
}

func (s *auditScenarioState) seededPlayerRequestsCode() error {
	return s.requestCodeFor(counterPlayerMail)
}

func (s *auditScenarioState) seededPlayerSubmitsIssuedCode() error {
	deadline := time.Now().Add(counterCodeBudget)
	for s.issuedCode() == "" {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for the emailed login code")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return s.post("/login/code", url.Values{"code": {s.issuedCode()}}, nil)
}

func (s *auditScenarioState) visitorSubmitsWrongCode() error {
	return s.post("/login/code", url.Values{"code": {"000000"}}, nil)
}

func (s *auditScenarioState) playerLogsOut() error {
	return s.post("/logout", url.Values{}, auth.IssueSessionCookie(counterPlayerID, testSessionSecret))
}

func (s *auditScenarioState) visitorLogsOut() error {
	return s.post("/logout", url.Values{}, nil)
}

func (s *auditScenarioState) playerSavesCupPick(teamID string) error {
	return s.post("/predict/cup", url.Values{"team_id": {teamID}}, auth.IssueSessionCookie(counterPlayerID, testSessionSecret))
}

func (s *auditScenarioState) auditLines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var lines []string
	for _, line := range strings.Split(s.logs.String(), "\n") {
		if strings.Contains(line, auditMarker) {
			lines = append(lines, line)
		}
	}
	return lines
}

func (s *auditScenarioState) noAuditLine() error {
	if lines := s.auditLines(); len(lines) != 0 {
		return fmt.Errorf("expected no audit line, got %q", lines)
	}
	return nil
}

// hasToken reports whether line holds token as a whole space-separated
// attribute, so player_id=basti does not match player_id=bastian.
func hasToken(line, token string) bool {
	return slices.Contains(strings.Fields(line), token)
}

func (s *auditScenarioState) requireOneLine(want ...string) error {
	var lines []string
	for _, line := range s.auditLines() {
		if hasToken(line, want[0]) {
			lines = append(lines, line)
		}
	}
	if len(lines) != 1 {
		return fmt.Errorf("expected exactly one %q audit line, got %d: %q", want[0], len(lines), lines)
	}
	for _, w := range want {
		if !hasToken(lines[0], w) {
			return fmt.Errorf("audit line %q lacks %q", lines[0], w)
		}
	}
	return nil
}

func (s *auditScenarioState) oneLineWithPlayer(event, player string) error {
	return s.requireOneLine("event="+event, "player_id="+player)
}

func (s *auditScenarioState) oneLineWithoutPlayer(event string) error {
	if err := s.requireOneLine("event=" + event); err != nil {
		return err
	}
	for _, line := range s.auditLines() {
		if hasToken(line, "event="+event) && strings.Contains(line, "player_id=") {
			return fmt.Errorf("audit line unexpectedly names a player: %q", line)
		}
	}
	return nil
}

func (s *auditScenarioState) oneSaveLine(event, player, kind, set string) error {
	return s.requireOneLine("event="+event, "player_id="+player, "kind="+kind, "set="+set)
}

func (s *auditScenarioState) noIdentifiers() error {
	lines := s.auditLines()
	if len(lines) == 0 {
		return fmt.Errorf("expected audit lines, got none")
	}
	code := s.issuedCode()
	for _, line := range lines {
		if strings.Contains(line, "@") || strings.Contains(line, counterPlayerMail) {
			return fmt.Errorf("audit line contains an email address: %q", line)
		}
		if code != "" && strings.Contains(line, code) {
			return fmt.Errorf("audit line contains the login code: %q", line)
		}
	}
	return nil
}

// InitializeAuditLogLinesScenario registers the audit-log-lines step
// definitions.
func InitializeAuditLogLinesScenario(ctx *godog.ScenarioContext) {
	s := newAuditScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a server for the audit log with an open cup set$`, s.anOpenServer)
	ctx.Step(`^a server for the audit log with a closed cup set$`, s.aClosedServer)
	ctx.Step(`^the seeded player requests a login code for the audit log$`, s.seededPlayerRequestsCode)
	ctx.Step(`^a visitor asks for a login code for "([^"]*)" for the audit log$`, s.requestCodeFor)
	ctx.Step(`^the seeded player submits the issued login code for the audit log$`, s.seededPlayerSubmitsIssuedCode)
	ctx.Step(`^a visitor submits the wrong login code for the audit log$`, s.visitorSubmitsWrongCode)
	ctx.Step(`^the signed-in player logs out for the audit log$`, s.playerLogsOut)
	ctx.Step(`^a visitor without a session logs out for the audit log$`, s.visitorLogsOut)
	ctx.Step(`^the signed-in player saves the cup pick "([^"]*)" for the audit log$`, s.playerSavesCupPick)
	ctx.Step(`^exactly one audit line has event "([^"]*)" and player id "([^"]*)"$`, s.oneLineWithPlayer)
	ctx.Step(`^exactly one audit line has event "([^"]*)" and no player id$`, s.oneLineWithoutPlayer)
	ctx.Step(`^exactly one audit line has event "([^"]*)", player id "([^"]*)", kind "([^"]*)" and set "([^"]*)"$`, s.oneSaveLine)
	ctx.Step(`^the audit log request was answered with status (\d+)$`, s.responseStatusIs)
	ctx.Step(`^no audit line is written$`, s.noAuditLine)
	ctx.Step(`^no audit line contains an email address or the issued login code$`, s.noIdentifiers)
}
