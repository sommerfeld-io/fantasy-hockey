package acceptance_test

import (
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
)

// scenarioState holds the value produced by the application for a single GoDog scenario.
// A fresh instance is created per scenario so state does not leak between runs.
type scenarioState struct {
	value string
}

func (s *scenarioState) theApplicationProvidesTheCurrentDateAndTime() error {
	s.value = clock.Now()
	return nil
}

func (s *scenarioState) theValueShouldBeAValidRFC3339Timestamp() error {
	if _, err := time.Parse(time.RFC3339, s.value); err != nil {
		return fmt.Errorf("expected a valid RFC 3339 timestamp, got %q: %w", s.value, err)
	}
	return nil
}

func (s *scenarioState) theValueShouldRepresentAMomentCloseToTheActualCurrentTime() error {
	parsed, err := time.Parse(time.RFC3339, s.value)
	if err != nil {
		return fmt.Errorf("expected a valid RFC 3339 timestamp, got %q: %w", s.value, err)
	}
	if diff := time.Since(parsed); diff < -time.Minute || diff > time.Minute {
		return fmt.Errorf("expected value to be within a minute of the current time, got %q", s.value)
	}
	return nil
}

// InitializeScenario registers all step definitions with GoDog.
// A new scenarioState is created per scenario to prevent state from leaking between scenarios.
func InitializeScenario(ctx *godog.ScenarioContext) {
	s := &scenarioState{}

	ctx.Step(`^the application provides the current date and time$`, s.theApplicationProvidesTheCurrentDateAndTime)
	ctx.Step(`^the value should be a valid RFC 3339 timestamp$`, s.theValueShouldBeAValidRFC3339Timestamp)
	ctx.Step(`^the value should represent a moment close to the actual current time$`, s.theValueShouldRepresentAMomentCloseToTheActualCurrentTime)
}
