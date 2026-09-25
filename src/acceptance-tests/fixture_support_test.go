package acceptance_test

import (
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// relativeDeadlinePattern parses the Gherkin-friendly deadline phrases the
// features' Given steps accept ("in 5 days", "in 3 hours", "1 day ago"),
// keeping every scenario's deadline relative to the moment it runs rather
// than a hardcoded date that would eventually go stale.
var relativeDeadlinePattern = regexp.MustCompile(`^(?:in (\d+) (hour|hours|day|days)|(\d+) (hour|hours|day|days) ago)$`)

// parseRelativeDeadline resolves phrase against now into an absolute UTC
// deadline.
func parseRelativeDeadline(phrase string, now time.Time) (time.Time, error) {
	m := relativeDeadlinePattern.FindStringSubmatch(phrase)
	if m == nil {
		return time.Time{}, fmt.Errorf("unrecognized relative deadline %q", phrase)
	}

	var amount, unit string
	sign := 1
	if m[1] != "" {
		amount, unit = m[1], m[2]
	} else {
		amount, unit = m[3], m[4]
		sign = -1
	}

	n, err := strconv.Atoi(amount)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse amount %q: %w", amount, err)
	}

	step := time.Hour
	if strings.HasPrefix(unit, "day") {
		step = 24 * time.Hour
	}

	return now.Add(time.Duration(sign*n) * step), nil
}

// seedHeader is the fantasy-hockey.yml preamble every seeded fixture starts
// with: the season plus the one player the scenario signs in as.
func seedHeader(playerID, playerName string) string {
	return fmt.Sprintf("season: \"2026-27\"\nplayers:\n    - id: %s\n      name: %s\n      email: basti@example.com\n", playerID, playerName)
}

// seedSubmittedAt stamps every prediction row a step file seeds; neither
// scoring nor any page reads it.
const seedSubmittedAt = "2026-09-20T10:00:00Z"

// seedConferenceByDivision names each division's own conference, matching
// each seeded team's own Team.Conference field - used only to seed fixtures;
// production code never hardcodes this mapping (see internal/web's
// groupDivisionsByConference).
var seedConferenceByDivision = map[string]string{
	"Atlantic":     "Eastern",
	"Metropolitan": "Eastern",
	"Central":      "Western",
	"Pacific":      "Western",
}

// seedRound is one playoff round's prediction-side set id and the
// results-side round name it maps to.
type seedRound struct {
	setID, resultRound string
}

// seedRounds maps the features' round wording ("round 1") to the ids the
// seeded YAML needs on each side.
var seedRounds = map[string]seedRound{
	"round 1": {store.Round1SetID, "round1"},
	"round 2": {store.Round2SetID, "round2"},
	"round 3": {store.ConferenceFinalsSetID, "round3"},
	"round 4": {store.StanleyCupFinalSetID, "round4"},
}

// mustSeedRound returns seedRounds[name], panicking when name is not a
// known round so a typo can't silently seed empty ids.
func mustSeedRound(name string) seedRound {
	round, ok := seedRounds[name]
	if !ok {
		panic(fmt.Sprintf("unknown round %q - seedRounds has no entry for it", name))
	}
	return round
}

// seedPrediction is one seeded prediction row: its player and its
// kind-specific fields as YAML flow-mapping entries (e.g. "kind: cup,
// team_id: FLA").
type seedPrediction struct {
	playerID, fields string
}

// seedMatchup is one playoff_matchups entry: a series key and its two sides.
type seedMatchup struct {
	key, a, b string
}

// seedDivisionMarks is one division's recorded playoff teams and winner.
type seedDivisionMarks struct {
	playoffs []string
	winner   string
}

// seedSeriesResult is one recorded series outcome; games is written
// unquoted, the way a human hand-edits it.
type seedSeriesResult struct {
	key, winner, games string
}

// seedResults is everything a step file records under results:.
// teamMarks is keyed by division name and series by results-side round name
// ("round1").
type seedResults struct {
	teamMarks  map[string]*seedDivisionMarks
	presidents string
	cupWinner  string
	series     map[string][]seedSeriesResult
}

// isEmpty reports whether r records nothing, so results: can be left out.
func (r seedResults) isEmpty() bool {
	return len(r.teamMarks) == 0 && r.presidents == "" && r.cupWinner == "" && len(r.series) == 0
}

// writeSeedTeams writes teams: with one team per id in sorted order, each
// in the division teamDivisions maps it to and that division's conference.
// It panics on a division seedConferenceByDivision doesn't know.
func writeSeedTeams(b *strings.Builder, teamDivisions map[string]string) {
	b.WriteString("teams:\n")
	for _, id := range slices.Sorted(maps.Keys(teamDivisions)) {
		division := teamDivisions[id]
		conference, ok := seedConferenceByDivision[division]
		if !ok {
			panic(fmt.Sprintf("no fixture conference for division %q - seedConferenceByDivision has drifted from store.Divisions()", division))
		}
		fmt.Fprintf(b, "    - {id: %s, name: Team %s, conference: %s, division: %s}\n", id, id, conference, division)
	}
}

// writeSeedNHLPlayers writes nhl_players: with one skater per slug, sorted.
func writeSeedNHLPlayers(b *strings.Builder, slugs []string) {
	b.WriteString("nhl_players:\n")
	for _, slug := range slices.Sorted(slices.Values(slugs)) {
		fmt.Fprintf(b, "    - {slug: %s, display_name: Player %s, position: skater}\n", slug, slug)
	}
}

// writeSeedMatchups writes playoff_matchups: with every set in sorted order
// and each set's matchups in the order given, or nothing when there are none.
func writeSeedMatchups(b *strings.Builder, matchupsBySet map[string][]seedMatchup) {
	if len(matchupsBySet) == 0 {
		return
	}
	b.WriteString("playoff_matchups:\n")
	for _, setID := range slices.Sorted(maps.Keys(matchupsBySet)) {
		fmt.Fprintf(b, "    %s:\n", setID)
		for _, m := range matchupsBySet[setID] {
			fmt.Fprintf(b, "        - {key: %s, a: %s, b: %s}\n", m.key, m.a, m.b)
		}
	}
}

// writeSeedPredictions writes predictions: with ids p1, p2, ... in the order
// given.
func writeSeedPredictions(b *strings.Builder, predictions []seedPrediction) {
	b.WriteString("predictions:\n")
	for i, p := range predictions {
		fmt.Fprintf(b, "    - {id: p%d, player_id: %s, submitted_at: %q, %s}\n", i+1, p.playerID, seedSubmittedAt, p.fields)
	}
}

// writeSeedResults writes results: (team_marks, trophies, series), or
// nothing when r records nothing.
func writeSeedResults(b *strings.Builder, r seedResults) {
	if r.isEmpty() {
		return
	}
	b.WriteString("results:\n")
	writeSeedTeamMarks(b, r.teamMarks)
	if r.presidents != "" {
		fmt.Fprintf(b, "    presidents_trophy: %s\n", r.presidents)
	}
	if r.cupWinner != "" {
		fmt.Fprintf(b, "    stanley_cup_winner: %s\n", r.cupWinner)
	}
	writeSeedSeriesResults(b, r.series)
}

// writeSeedTeamMarks writes results.team_marks with every division in sorted
// order, or nothing when there are none.
func writeSeedTeamMarks(b *strings.Builder, teamMarks map[string]*seedDivisionMarks) {
	if len(teamMarks) == 0 {
		return
	}
	b.WriteString("    team_marks:\n")
	for _, division := range slices.Sorted(maps.Keys(teamMarks)) {
		marks := teamMarks[division]
		fmt.Fprintf(b, "        %s:\n            playoffs: [%s]\n", strings.ToLower(division), strings.Join(marks.playoffs, ", "))
		if marks.winner != "" {
			fmt.Fprintf(b, "            division_winner: %s\n", marks.winner)
		}
	}
}

// writeSeedSeriesResults writes results.series with every round in sorted
// order and each round's series in the order given, or nothing when there
// are none.
func writeSeedSeriesResults(b *strings.Builder, series map[string][]seedSeriesResult) {
	if len(series) == 0 {
		return
	}
	b.WriteString("    series:\n")
	for _, round := range slices.Sorted(maps.Keys(series)) {
		fmt.Fprintf(b, "        %s:\n", round)
		for _, r := range series[round] {
			fmt.Fprintf(b, "            %s: {winner: %s, games: %s}\n", r.key, r.winner, r.games)
		}
	}
}

// writeSeedFinalists writes award_finalists: with every award in sorted
// order and each award's finalists in the order given, or nothing when there
// are none.
func writeSeedFinalists(b *strings.Builder, finalists map[string][]string) {
	if len(finalists) == 0 {
		return
	}
	b.WriteString("award_finalists:\n")
	for _, award := range slices.Sorted(maps.Keys(finalists)) {
		fmt.Fprintf(b, "    %s:\n", award)
		for _, slug := range finalists[award] {
			fmt.Fprintf(b, "        - {slug: %s, display_name: Player %s}\n", slug, slug)
		}
	}
}

// newScenarioDataFile creates a fresh temp directory named after prefix and
// returns the data file path inside it (not yet written).
func newScenarioDataFile(prefix string) string {
	dir, err := os.MkdirTemp("", "fantasy-hockey-"+prefix+"-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	return filepath.Join(dir, store.DataFileName)
}

// writeSeededStore writes seed to dataFile and opens a store on it, so
// store.New's own parsing runs end-to-end against the seeded document.
func writeSeededStore(dataFile, seed string) (*store.Store, error) {
	if err := os.WriteFile(dataFile, []byte(seed), 0o600); err != nil {
		return nil, fmt.Errorf("seed data file: %w", err)
	}
	st, err := store.New(dataFile)
	if err != nil {
		return nil, fmt.Errorf("store.New: %w", err)
	}
	return st, nil
}

// newSeededStore bootstraps a store backed by seed in a new temp directory
// named after prefix, for scenarios whose fixture is fixed up front. It
// returns the data file's path so the caller can remove its directory with
// removeScenarioDataFile once the scenario ends.
func newSeededStore(prefix, seed string) (*store.Store, string) {
	dataFile := newScenarioDataFile(prefix)
	st, err := writeSeededStore(dataFile, seed)
	if err != nil {
		panic(err.Error())
	}
	return st, dataFile
}

// removeScenarioDataFile best-effort removes dataFile's temp directory.
func removeScenarioDataFile(dataFile string) {
	_ = os.RemoveAll(filepath.Dir(dataFile))
}

// lazyFixture is the embeddable scenario state for features whose Given
// steps keep appending fixtures before the first request: the data file,
// store and server are only built (ensureReady) when a step first needs a
// running server. seedBody supplies everything after seedHeader; afterStore,
// when non-nil, runs once the store exists but before the server starts
// (e.g. to save prior picks). Every request carries playerID's session
// cookie, signed with secret, and never follows a redirect.
type lazyFixture struct {
	dataFile   string
	playerID   string
	playerName string
	secret     string
	seedBody   func() (string, error)
	afterStore func(*store.Store) error

	st           *store.Store
	server       *httptest.Server
	lastStatus   int
	lastBody     string
	lastLocation string
}

// newLazyFixture creates a lazyFixture with its own temp directory named
// after prefix.
func newLazyFixture(prefix, playerID, playerName, secret string, seedBody func() (string, error), afterStore func(*store.Store) error) lazyFixture {
	return lazyFixture{
		dataFile:   newScenarioDataFile(prefix),
		playerID:   playerID,
		playerName: playerName,
		secret:     secret,
		seedBody:   seedBody,
		afterStore: afterStore,
	}
}

// ensureReady writes the seed, opens the store, runs afterStore and starts
// the real production web.NewServer handler - once per scenario.
func (f *lazyFixture) ensureReady() error {
	if f.server != nil {
		return nil
	}

	body, err := f.seedBody()
	if err != nil {
		return err
	}
	st, err := writeSeededStore(f.dataFile, seedHeader(f.playerID, f.playerName)+body)
	if err != nil {
		return err
	}
	f.st = st

	if f.afterStore != nil {
		if err := f.afterStore(st); err != nil {
			return err
		}
	}

	f.server = httptest.NewServer(web.NewServer(st, noopSender, f.secret))
	return nil
}

// close stops the server (if it was ever started) and removes the
// scenario's temp directory.
func (f *lazyFixture) close() {
	if f.server != nil {
		f.server.Close()
	}
	removeScenarioDataFile(f.dataFile)
}

// do requests method+path carrying the signed-in player's session cookie,
// with an optional urlencoded form body, without following any redirect,
// and records the result.
func (f *lazyFixture) do(method, path, body string) error {
	if err := f.ensureReady(); err != nil {
		return err
	}

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, f.server.URL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.AddCookie(auth.IssueSessionCookie(f.playerID, f.secret))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	f.lastStatus = resp.StatusCode
	f.lastLocation = resp.Header.Get("Location")
	f.lastBody = string(respBody)
	return nil
}

// sampleTeamNames maps the team ids that the cup-and-presidents picks and
// playoffs Cup pick Backgrounds declare to their display names. It is a
// small sample, not the full 32-team roster, because those scenarios only
// need a couple of teams to pick from. Series predictions keeps its own
// seriesPredictionsTeams because it also needs each team's conference.
var sampleTeamNames = map[string]string{
	"TOR": "Toronto Maple Leafs",
	"VGK": "Vegas Golden Knights",
}

// requireSeededPlayer fails unless name is exactly want, the one player a
// scenario seeds. The comparison is case-sensitive: a feature has to name
// the seeded player the way the fixture spells it.
func requireSeededPlayer(name, want string) error {
	if name != want {
		return fmt.Errorf("no fixture for player %q; only %q is seeded", name, want)
	}
	return nil
}

// theSignedInPlayerIs validates name against the one player the fixture
// seeds and signs requests in as (see do). It is promoted to every scenario
// state that embeds lazyFixture, whatever step text each registers it under;
// steps without a lazyFixture call requireSeededPlayer directly. It performs
// no sign-in action itself - the session cookie do() attaches is
// unconditional and doesn't depend on this step having run.
func (f *lazyFixture) theSignedInPlayerIs(name string) error {
	return requireSeededPlayer(name, f.playerName)
}

// setRowFragment isolates the single set row for id (the <a>/<div> with
// id="predict-row-{id}" up to its closing tag) in the last response body so
// an assertion about one row can't accidentally match text belonging to a
// different row on the same page.
func (f *lazyFixture) setRowFragment(id string) (string, error) {
	marker := `id="predict-row-` + id + `"`
	start := strings.Index(f.lastBody, marker)
	if start == -1 {
		return "", fmt.Errorf("expected a set row for %q, got %q", id, f.lastBody)
	}
	rest := f.lastBody[start:]
	end := strings.Index(rest, "</a>")
	if divEnd := strings.Index(rest, "</div>"); end == -1 || (divEnd != -1 && divEnd < end) {
		end = divEnd
	}
	if end == -1 {
		return "", fmt.Errorf("could not find the end of the set row for %q", id)
	}
	return rest[:end], nil
}

func TestRequireSeededPlayerShouldAcceptTheExactName(t *testing.T) {
	if err := requireSeededPlayer("Basti", "Basti"); err != nil {
		t.Fatalf("requireSeededPlayer(%q, %q) = %v, want nil", "Basti", "Basti", err)
	}
}

func TestRequireSeededPlayerShouldRejectADifferentCase(t *testing.T) {
	err := requireSeededPlayer("basti", "Basti")
	if err == nil {
		t.Fatalf("requireSeededPlayer(%q, %q) = nil, want an error", "basti", "Basti")
	}
	want := `no fixture for player "basti"; only "Basti" is seeded`
	if err.Error() != want {
		t.Fatalf("requireSeededPlayer error = %q, want %q", err.Error(), want)
	}
}
