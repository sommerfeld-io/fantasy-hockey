package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// rfc1123Pattern matches the timestamp format the home page renders.
var rfc1123Pattern = regexp.MustCompile(`[A-Za-z]{3}, \d{2} [A-Za-z]{3} \d{4} \d{2}:\d{2}:\d{2} [A-Za-z]{3,4}`)

// homePageScenarioState holds the fixtures and results for one home-page
// scenario. A fresh instance is created per scenario so state never leaks
// between runs.
type homePageScenarioState struct {
	server *httptest.Server
	body   string
}

func newHomePageScenarioState() *homePageScenarioState {
	return &homePageScenarioState{server: httptest.NewServer(web.NewServer())}
}

func (s *homePageScenarioState) close() {
	s.server.Close()
}

func (s *homePageScenarioState) aVisitorOpensTheHomePage() error {
	resp, err := http.Get(s.server.URL + "/")
	if err != nil {
		return fmt.Errorf("get /: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200, got %d", resp.StatusCode)
	}
	s.body = string(body)
	return nil
}

func (s *homePageScenarioState) theResponseShows(text string) error {
	if !strings.Contains(s.body, text) {
		return fmt.Errorf("expected response body to contain %q, got %q", text, s.body)
	}
	return nil
}

func (s *homePageScenarioState) theResponseShowsTheCurrentDateAndTime() error {
	match := rfc1123Pattern.FindString(s.body)
	if match == "" {
		return fmt.Errorf("expected response body to contain a date and time, got %q", s.body)
	}

	got, err := time.Parse(time.RFC1123, match)
	if err != nil {
		return fmt.Errorf("parse timestamp %q: %w", match, err)
	}
	if age := time.Since(got); age < -time.Minute || age > time.Minute {
		return fmt.Errorf("expected the shown timestamp to be close to the current time, got %v (%v ago)", got, age)
	}
	return nil
}

// InitializeHomePageScenario registers the home page step definitions with GoDog.
func InitializeHomePageScenario(ctx *godog.ScenarioContext) {
	s := newHomePageScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a visitor opens the home page$`, s.aVisitorOpensTheHomePage)
	ctx.Step(`^the response shows "([^"]*)"$`, s.theResponseShows)
	ctx.Step(`^the response shows the current date and time$`, s.theResponseShowsTheCurrentDateAndTime)
}
