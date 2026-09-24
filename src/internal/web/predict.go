package web

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Prediction Set phases, matching fantasy-hockey.yml's prediction_sets[].phase
// values and the Predict screen's two section labels (DESIGN.md's Section
// header icon component: a target icon for "Before the season," a trophy
// icon for "Playoffs").
const (
	phaseBeforeSeason = "before_season"
	phasePlayoffs     = "playoffs"
)

// Round-gated Prediction Set ids, matching fantasy-hockey.yml's
// prediction_sets[].id values for the three playoff rounds whose Upcoming
// status Story 3.2 computes from playoff_matchups rather than trusting the
// set's own hand-maintained upcoming flag (Design Notes: "r1" stays
// human-gated for this story - Story 3.3's later concern).
const (
	round2SetID           = "r2"
	conferenceFinalsSetID = "cf"
	stanleyCupFinalSetID  = "scf"
)

// roundGatedSetIDs is every Prediction Set id effectiveUpcoming computes
// from playoff_matchups instead of reading Upcoming directly - referenced
// once here so the round2SetID/conferenceFinalsSetID/stanleyCupFinalSetID
// trio can't drift out of sync with effectiveUpcoming's own if-check.
var roundGatedSetIDs = map[string]bool{
	round2SetID:           true,
	conferenceFinalsSetID: true,
	stanleyCupFinalSetID:  true,
}

// effectiveUpcoming reports set's effective "Upcoming" state: for a
// roundGatedSetIDs id, it ignores set.Upcoming entirely and instead reports
// whether any matchup is recorded for that id under playoff_matchups (FR-20)
// - no matchup recorded means still Upcoming, regardless of the id's own
// hand-maintained upcoming value. For every other id, it returns set.Upcoming
// unchanged. Both the Predict list (buildPredictPhases) and the direct-URL
// sheet gate (findOpenablePredictionSet) call this one helper, so the two
// can never disagree about which round is unlocked.
func effectiveUpcoming(st *store.Store, set store.PredictionSet) bool {
	if roundGatedSetIDs[set.ID] {
		return len(st.PlayoffMatchups(set.ID)) == 0
	}
	return set.Upcoming
}

// Prediction Set status labels and their status-pill CSS classes
// (styles.css).
const (
	statusOpen      = "Open"
	statusSubmitted = "Submitted"
	statusClosed    = "Closed"
	statusUpcoming  = "Upcoming"

	statusPillOpen      = "status-pill--open"
	statusPillSubmitted = "status-pill--submitted"
	statusPillClosed    = "status-pill--closed"
	statusPillUpcoming  = "status-pill--upcoming"
)

// predictSetView is one Predict-screen row: every field is already
// presentation-ready (formatted deadline, computed status/pill/accent), so
// shell.html only renders - it never computes status or formats a
// timestamp itself (Boundaries & Constraints: "one reusable helper, not
// duplicated per template").
type predictSetView struct {
	ID             string
	Title          string
	Subtitle       string
	DeadlineText   string
	Countdown      string
	CountdownFaint bool // true for Upcoming rows (DESIGN.md: faint countdown when Upcoming, ice otherwise)
	Status         string
	StatusPillCSS  string
	AccentCSS      string
	Actionable     bool // Open/Submitted/Closed rows link to /predict/{id}; Upcoming rows don't
}

// predictPhases is Predict's two phase-grouped sections.
type predictPhases struct {
	BeforeSeason []predictSetView
	Playoffs     []predictSetView
}

// newPredictSetView derives set's presentation-ready row from its
// hand-maintained YAML fields, evaluating status/countdown against now.
// upcoming is set's effective Upcoming state (effectiveUpcoming's result,
// resolved by the caller) - callers never read set.Upcoming directly, since
// for a round-gated id (Story 3.2) it isn't authoritative. submitted is
// whether the current player already has a saved Prediction row for set
// (st.FindPrediction(playerID, set.ID)); it only ever affects non-upcoming
// sets, since Upcoming always wins (Boundaries & Constraints). An error means
// set.DeadlineUTC isn't valid RFC3339 - a hand-edit mistake in
// fantasy-hockey.yml, not something a caller can recover from per-row, so
// the caller drops the row rather than rendering a broken one.
func newPredictSetView(set store.PredictionSet, upcoming, submitted bool, now time.Time) (predictSetView, error) {
	deadline, err := time.Parse(time.RFC3339, set.DeadlineUTC)
	if err != nil {
		return predictSetView{}, fmt.Errorf("parse deadline_utc %q: %w", set.DeadlineUTC, err)
	}

	status, pillCSS := predictStatus(upcoming, submitted, deadline, now)
	accentCSS := "set-row--open"
	switch {
	case upcoming:
		accentCSS = "set-row--upcoming"
	case submitted:
		accentCSS = "set-row--submitted"
	}

	return predictSetView{
		ID:             set.ID,
		Title:          set.Title,
		Subtitle:       set.Subtitle,
		DeadlineText:   clock.FormatDeadline(deadline),
		Countdown:      clock.Countdown(deadline, now),
		CountdownFaint: upcoming,
		Status:         status,
		StatusPillCSS:  pillCSS,
		AccentCSS:      accentCSS,
		Actionable:     !upcoming,
	}, nil
}

// predictStatus computes a Prediction Set's status label and status-pill CSS
// class from its Upcoming flag, whether the player already has a saved pick
// (submitted), and its deadline. Precedence is Upcoming > Closed > Submitted
// > Open - a deadline exactly at now counts as closed (now is never "still
// open" once it reaches the deadline), and a closed set never shows
// Submitted even if a pick was saved before the deadline passed.
func predictStatus(upcoming, submitted bool, deadline, now time.Time) (label, pillCSS string) {
	switch {
	case upcoming:
		return statusUpcoming, statusPillUpcoming
	case !deadline.After(now):
		return statusClosed, statusPillClosed
	case submitted:
		return statusSubmitted, statusPillSubmitted
	default:
		return statusOpen, statusPillOpen
	}
}

// buildPredictPhases groups st's Prediction Sets into their two Predict
// sections, in the order fantasy-hockey.yml lists them. playerID is the
// logged-in player's id (may be empty for a session lookup miss), used to
// look up each set's saved Prediction (kind == set.ID) so its row can show
// Submitted status - a set with no matching Prediction.Kind (every id other
// than "cup"/"presidents", today) naturally never finds one. A row whose
// deadline_utc fails to parse is logged and skipped rather than failing the
// whole page - one hand-edit mistake shouldn't take down every other
// Prediction Set. A row with a phase other than "before_season" or
// "playoffs" is likewise logged and skipped.
func buildPredictPhases(st *store.Store, playerID string, now time.Time) predictPhases {
	var phases predictPhases
	for _, set := range st.PredictionSets() {
		submitted := setSubmitted(st, set, playerID)
		view, err := newPredictSetView(set, effectiveUpcoming(st, set), submitted, now)
		if err != nil {
			slog.Error("build predict set view", "prediction_set_id", set.ID, "error", err)
			continue
		}

		switch set.Phase {
		case phaseBeforeSeason:
			phases.BeforeSeason = append(phases.BeforeSeason, view)
		case phasePlayoffs:
			phases.Playoffs = append(phases.Playoffs, view)
		default:
			slog.Error("unknown prediction set phase", "prediction_set_id", set.ID, "phase", set.Phase)
		}
	}
	return phases
}
