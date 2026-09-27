package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// handEditedResultsSecret signs session cookies for this scenario's server -
// it only needs to be non-empty and stable within one scenario run.
const handEditedResultsSecret = "hand-edited-results-test-secret"

// handEditedResultsPlayerID/Name is the one seeded player this scenario
// signs in as and submits a pick for.
const (
	handEditedResultsPlayerID   = "basti"
	handEditedResultsPlayerName = "Basti"
)

// handEditedResultsSection is the hand-maintained results:/award_finalists:
// block this scenario seeds verbatim: an inline comment, a flow-style
// team_marks entry, and an award finalist entry whose keys are in the
// opposite order from how internal/store's AwardFinalist struct declares
// them (display_name before slug) - exactly the shapes spec-7-4's AC3
// promises a save never rewrites.
const handEditedResultsSection = `results:
    team_marks:
        atlantic: {playoffs: [FLA, TOR], division_winner: FLA} # division call
    stanley_cup_winner: FLA
award_finalists:
    hart:
        - {display_name: Connor McDavid, slug: mcdavid-connor}
`

// handEditedResultsScenarioState holds the fixture and result for the one
// scenario in this feature. A fresh instance is created per scenario so
// state never leaks between runs. The store and server are built lazily
// (ensureReady) the first time a step needs to make an HTTP call, mirroring
// enter_login_code_steps_test.go's own lazy-server pattern.
type handEditedResultsScenarioState struct {
	dataFile string
	server   *httptest.Server
	status   int
}

func newHandEditedResultsScenarioState() *handEditedResultsScenarioState {
	dir, err := os.MkdirTemp("", "fantasy-hockey-hand-edited-results-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	return &handEditedResultsScenarioState{dataFile: filepath.Join(dir, store.DataFileName)}
}

func (s *handEditedResultsScenarioState) close() {
	if s.server != nil {
		s.server.Close()
	}
	_ = os.RemoveAll(filepath.Dir(s.dataFile)) // best-effort cleanup of the scenario's temp dir
}

// aDataFileWithAHandEditedResultsSection seeds a full data file: one player,
// one open "cup" Prediction Set, two teams, one nhl_player, and
// handEditedResultsSection written exactly as a human would type it.
func (s *handEditedResultsScenarioState) aDataFileWithAHandEditedResultsSection() error {
	deadline := time.Now().UTC().Add(5 * 24 * time.Hour).Format(time.RFC3339)
	seed := fmt.Sprintf(`season: "2026-27"
players:
    - id: %s
      name: %s
      email: basti@example.com
prediction_sets:
    - id: cup
      title: "Cup champion"
      subtitle: "Your Stanley Cup winner"
      deadline_utc: %q
      phase: before_season
      upcoming: false
teams:
    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}
    - {id: TOR, name: Toronto Maple Leafs, conference: Eastern, division: Atlantic}
nhl_players:
    - {slug: mcdavid-connor, display_name: Connor McDavid, position: skater}
`, handEditedResultsPlayerID, handEditedResultsPlayerName, deadline) + handEditedResultsSection

	if err := os.WriteFile(s.dataFile, []byte(seed), 0o600); err != nil {
		return fmt.Errorf("seed data file: %w", err)
	}
	return nil
}

// ensureReady lazily opens the real production store and starts the real
// production web.NewServer handler around the seeded data file, the first
// time a step needs to make an HTTP call.
func (s *handEditedResultsScenarioState) ensureReady() error {
	if s.server != nil {
		return nil
	}
	st, err := store.New(s.dataFile)
	if err != nil {
		return fmt.Errorf("store.New: %w", err)
	}

	send := func(_, _, _ string) error { return nil }
	s.server = httptest.NewServer(web.NewServer(st, send, handEditedResultsSecret))
	return nil
}

// thePlayerSubmitsAValidPickThroughTheWebUI submits a real POST /predict/cup
// carrying the signed-in player's session cookie - the same request path a
// browser would use, so this proves the save round-trips through the
// production write path (internal/store's writeLocked), not a direct store
// call.
func (s *handEditedResultsScenarioState) thePlayerSubmitsAValidPickThroughTheWebUI() error {
	if err := s.ensureReady(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.server.URL+"/predict/cup", strings.NewReader("team_id=TOR"))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(auth.IssueSessionCookie(handEditedResultsPlayerID, handEditedResultsSecret))

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post /predict/cup: %w", err)
	}
	defer resp.Body.Close()

	s.status = resp.StatusCode
	return nil
}

// handEditedResultsExpectedBlock is the whole results:/award_finalists:
// block a save must leave untouched, byte-for-byte, except the one property
// the Design Notes explicitly exempt: a block sequence's indentation
// (empirically confirmed the finalist list item's indentation is the only
// difference from handEditedResultsSection for this exact fixture - it
// shifts from 8 to 6 spaces). Checking the whole block rather than scattered
// fragments means a bug that duplicated, reordered, or inserted content
// elsewhere in this section would be caught, not just a check that specific
// substrings still appear somewhere in the file.
const handEditedResultsExpectedBlock = `results:
    team_marks:
        atlantic: {playoffs: [FLA, TOR], division_winner: FLA} # division call
    stanley_cup_winner: FLA
award_finalists:
    hart:
      - {display_name: Connor McDavid, slug: mcdavid-connor}
`

// thePersistedResultsSectionsAreByteForByteUnchanged reads the data file
// straight off disk after the save and asserts the whole results:/
// award_finalists: block matches handEditedResultsExpectedBlock exactly -
// the comment, the flow style, and the finalist entry's unusual key order
// all survive (AC3) - and that the save itself actually happened, not just
// that the untouched section looks untouched because nothing was written at
// all: the submitted pick must appear under predictions:.
func (s *handEditedResultsScenarioState) thePersistedResultsSectionsAreByteForByteUnchanged() error {
	if s.status != http.StatusFound {
		return fmt.Errorf("expected the pick submission to redirect (status %d), got %d", http.StatusFound, s.status)
	}

	raw, err := os.ReadFile(s.dataFile)
	if err != nil {
		return fmt.Errorf("read data file: %w", err)
	}
	if !strings.Contains(string(raw), handEditedResultsExpectedBlock) {
		return fmt.Errorf("expected the results:/award_finalists: block to match exactly (aside from the documented indentation exception), got:\n%s", raw)
	}
	if !strings.Contains(string(raw), "team_id: TOR") {
		return fmt.Errorf("expected the submitted pick (team_id: TOR) to be persisted under predictions:, got:\n%s", raw)
	}
	return nil
}

// InitializeHandEditedResultsScenario registers the
// hand-edited-results-are-safe-to-edit step definitions with GoDog.
func InitializeHandEditedResultsScenario(ctx *godog.ScenarioContext) {
	s := newHandEditedResultsScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a data file with a hand-edited results and award_finalists section containing a comment, flow style, and an unusual key order$`, s.aDataFileWithAHandEditedResultsSection)
	ctx.Step(`^the player submits a valid pick through the web UI$`, s.thePlayerSubmitsAValidPickThroughTheWebUI)
	ctx.Step(`^the persisted results and award_finalists sections are byte-for-byte unchanged$`, s.thePersistedResultsSectionsAreByteForByteUnchanged)
}
