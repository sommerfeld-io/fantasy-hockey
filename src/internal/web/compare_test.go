package web

import (
	"go/parser"
	"go/token"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// compareNow is the fixed "now" every Compare unit test evaluates against.
var compareNow = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

// compareSeedPlayers declares three players in non-alphabetical file order,
// so column order can only come from st.Players().
const compareSeedPlayers = `season: "2026-27"
players:
    - {id: sadl, name: Sadl, email: sadl@example.com}
    - {id: basti, name: Basti, email: basti@example.com}
    - {id: tobbi, name: Tobbi, email: tobbi@example.com}
teams:
    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}
    - {id: TOR, name: Toronto Maple Leafs, conference: Eastern, division: Atlantic}
    - {id: COL, name: Colorado Avalanche, conference: Western, division: Central}
    - {id: VGK, name: Vegas Golden Knights, conference: Western, division: Pacific}
nhl_players:
    - {slug: mcdavid-connor, display_name: Connor McDavid, position: skater}
    - {slug: mackinnon-nathan, display_name: Nathan MacKinnon, position: skater}
`

// compareSetSeed renders one prediction_sets entry.
func compareSetSeed(id, title, phase, deadline string, upcoming bool) string {
	return "    - {id: " + id + ", title: \"" + title + "\", subtitle: \"\", deadline_utc: \"" + deadline + "\", phase: " + phase + ", upcoming: " + strconv.FormatBool(upcoming) + "}\n"
}

// compareDefaultSets is every Prediction Set kind, with r2 still gated.
var compareDefaultSets = compareSetSeed("cup", "Cup champion", phaseBeforeSeason, "2026-10-06T17:00:00Z", false) +
	compareSetSeed("presidents", "Presidents' Trophy", phaseBeforeSeason, "2026-10-06T17:00:00Z", false) +
	compareSetSeed("divisions", "Division picks", phaseBeforeSeason, "2026-10-06T17:00:00Z", false) +
	compareSetSeed("awards", "Player awards", phaseBeforeSeason, "2026-10-06T17:00:00Z", false) +
	compareSetSeed("playoffcup", "Playoffs Cup pick", phasePlayoffs, "2027-04-15T16:00:00Z", false) +
	compareSetSeed("r1", "Playoff round 1", phasePlayoffs, "2027-04-18T16:00:00Z", false) +
	compareSetSeed("r2", "Playoff round 2", phasePlayoffs, "2027-04-30T16:00:00Z", false) +
	compareSetSeed("scf", "Stanley Cup final", phasePlayoffs, "2027-05-28T16:00:00Z", false)

// compareDefaultMatchups unlocks r1 and scf; r2 stays gated.
const compareDefaultMatchups = `playoff_matchups:
    r1:
        - {key: s1, a: FLA, b: TOR}
        - {key: s2, a: COL, b: VGK}
    scf:
        - {key: s1, a: FLA, b: COL}
`

// comparePick renders one saved prediction row from its kind-specific
// flow-mapping fields.
func comparePick(playerID, fields string) string {
	return "    - {player_id: " + playerID + ", submitted_at: \"" + seedSubmittedAt + "\", " + fields + "}\n"
}

// newCompareStore seeds compareSeedPlayers plus sets, matchups and picks.
func newCompareStore(t *testing.T, sets, matchups string, picks ...string) *store.Store {
	t.Helper()
	st, _ := newSeededStore(t, compareSeedPlayers+"prediction_sets:\n"+sets+matchups+"predictions:\n"+strings.Join(picks, ""))
	return st
}

func chipIDs(chips []compareChipView) []string {
	ids := make([]string, 0, len(chips))
	for _, c := range chips {
		ids = append(ids, c.ID)
	}
	return ids
}

// groupChips returns the chip ids of the group labelled label.
func groupChips(t *testing.T, v compareView, label string) []string {
	t.Helper()
	for _, g := range v.Groups {
		if g.Label == label {
			return chipIDs(g.Chips)
		}
	}
	t.Fatalf("no group labelled %q in %+v", label, v.Groups)
	return nil
}

func selectedChips(v compareView) []string {
	var ids []string
	for _, g := range v.Groups {
		for _, c := range g.Chips {
			if c.Selected {
				ids = append(ids, c.ID)
			}
		}
	}
	return ids
}

func rowLabels(table *compareTableView) []string {
	labels := make([]string, 0, len(table.Rows))
	for _, r := range table.Rows {
		labels = append(labels, r.Label)
	}
	return labels
}

// cellValues returns every player's values for the row labelled label, in
// column order.
func cellValues(t *testing.T, table *compareTableView, label string) [][]string {
	t.Helper()
	for _, r := range table.Rows {
		if r.Label == label {
			values := make([][]string, 0, len(r.Cells))
			for _, c := range r.Cells {
				values = append(values, c.Values)
			}
			return values
		}
	}
	t.Fatalf("no row labelled %q in %v", label, rowLabels(table))
	return nil
}

func TestBuildCompareShouldGroupSelectableSetsIntoTwoLabelledRows(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	v := buildCompare(st, "basti", "", compareNow)

	if got := []string{v.Groups[0].Label, v.Groups[1].Label}; !slices.Equal(got, []string{"Before the season", "Playoffs"}) {
		t.Fatalf("expected the groups Before the season, Playoffs; got %v", got)
	}
	if got := groupChips(t, v, "Before the season"); !slices.Equal(got, []string{"cup", "presidents", "divisions", "awards"}) {
		t.Errorf("unexpected Before the season chips %v", got)
	}
	if got := groupChips(t, v, "Playoffs"); !slices.Equal(got, []string{"playoffcup", "r1", "scf"}) {
		t.Errorf("unexpected Playoffs chips %v", got)
	}
}

func TestBuildCompareShouldNotOfferAChipForAnUpcomingSet(t *testing.T) {
	sets := compareSetSeed("cup", "Cup champion", phaseBeforeSeason, "2026-10-06T17:00:00Z", false) +
		compareSetSeed("playoffcup", "Playoffs Cup pick", phasePlayoffs, "2027-04-15T16:00:00Z", true) +
		compareSetSeed("r2", "Playoff round 2", phasePlayoffs, "2027-04-30T16:00:00Z", false)
	st := newCompareStore(t, sets, "")

	v := buildCompare(st, "basti", "", compareNow)

	if got := groupChips(t, v, "Playoffs"); len(got) != 0 {
		t.Errorf("expected no Playoffs chips for an upcoming set and a gated round, got %v", got)
	}
}

func TestBuildCompareShouldLeaveOutSetsWithABadDeadlineOrUnknownPhase(t *testing.T) {
	sets := compareSetSeed("cup", "Cup champion", phaseBeforeSeason, "2026-10-06T17:00:00Z", false) +
		compareSetSeed("broken", "Broken", phaseBeforeSeason, "not-a-date", false) +
		compareSetSeed("odd", "Odd", "midseason", "2026-10-06T17:00:00Z", false)
	st := newCompareStore(t, sets, "")

	v := buildCompare(st, "basti", "broken", compareNow)

	if got := groupChips(t, v, "Before the season"); !slices.Equal(got, []string{"cup"}) {
		t.Errorf("expected only the cup chip, got %v", got)
	}
	if got := selectedChips(v); !slices.Equal(got, []string{"cup"}) {
		t.Errorf("expected a bad-deadline id to fall back to cup, got %v", got)
	}
}

func TestBuildCompareShouldSelectTheEarliestBeforeSeasonSetByDefault(t *testing.T) {
	sets := compareSetSeed("playoffcup", "Playoffs Cup pick", phasePlayoffs, "2026-09-30T16:00:00Z", false) +
		compareSetSeed("cup", "Cup champion", phaseBeforeSeason, "2026-10-08T17:00:00Z", false) +
		compareSetSeed("presidents", "Presidents' Trophy", phaseBeforeSeason, "2026-10-06T17:00:00Z", false) +
		compareSetSeed("awards", "Player awards", phaseBeforeSeason, "2026-10-06T17:00:00Z", false)
	st := newCompareStore(t, sets, "")

	v := buildCompare(st, "basti", "", compareNow)

	if got := selectedChips(v); !slices.Equal(got, []string{"presidents"}) {
		t.Errorf("expected the earliest-deadline, first-listed before-season set selected, got %v", got)
	}
	if v.Table == nil || v.Table.Title != "Presidents' Trophy" {
		t.Errorf("expected the Presidents' Trophy table, got %+v", v.Table)
	}
}

func TestBuildCompareShouldFallBackToAPlayoffsSetWhenNoBeforeSeasonSetIsSelectable(t *testing.T) {
	sets := compareSetSeed("cup", "Cup champion", phaseBeforeSeason, "2026-10-06T17:00:00Z", true) +
		compareSetSeed("playoffcup", "Playoffs Cup pick", phasePlayoffs, "2027-04-15T16:00:00Z", false)
	st := newCompareStore(t, sets, "")

	v := buildCompare(st, "basti", "", compareNow)

	if got := selectedChips(v); !slices.Equal(got, []string{"playoffcup"}) {
		t.Errorf("expected playoffcup selected, got %v", got)
	}
}

func TestBuildCompareShouldHaveNoTableWhenNoSetIsSelectable(t *testing.T) {
	st := newCompareStore(t, compareSetSeed("cup", "Cup champion", phaseBeforeSeason, "2026-10-06T17:00:00Z", true), "")

	v := buildCompare(st, "basti", "cup", compareNow)

	if v.Table != nil {
		t.Errorf("expected no table, got %+v", v.Table)
	}
	if len(v.Groups) != 2 {
		t.Errorf("expected both labelled groups even when empty, got %+v", v.Groups)
	}
}

func TestBuildCompareShouldSelectTheRequestedSetOnly(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	for _, id := range []string{"presidents", "r1", "scf"} {
		t.Run(id, func(t *testing.T) {
			v := buildCompare(st, "basti", id, compareNow)

			if got := selectedChips(v); !slices.Equal(got, []string{id}) {
				t.Errorf("expected only %q selected, got %v", id, got)
			}
		})
	}
}

func TestBuildCompareShouldFallBackToTheDefaultForAnUnselectableID(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	for _, id := range []string{"nope", "r2", ""} {
		t.Run(id, func(t *testing.T) {
			v := buildCompare(st, "basti", id, compareNow)

			if got := selectedChips(v); !slices.Equal(got, []string{"cup"}) {
				t.Errorf("expected %q to fall back to cup, got %v", id, got)
			}
		})
	}
}

func TestBuildCompareShouldShowTheSelectedSetsDeadline(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	v := buildCompare(st, "basti", "r1", compareNow)

	deadline := time.Date(2027, 4, 18, 16, 0, 0, 0, time.UTC)
	if v.Table.DeadlineText != clock.FormatDeadline(deadline) {
		t.Errorf("expected deadline %q, got %q", clock.FormatDeadline(deadline), v.Table.DeadlineText)
	}
	if v.Table.Countdown != clock.Countdown(deadline, compareNow) {
		t.Errorf("expected countdown %q, got %q", clock.Countdown(deadline, compareNow), v.Table.Countdown)
	}
}

func TestBuildCompareShouldOrderColumnsByStorePlayersAndMarkOnlyTheOwnColumn(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	v := buildCompare(st, "basti", "cup", compareNow)

	var names []string
	var own []string
	for _, c := range v.Table.Columns {
		names = append(names, c.Name)
		if c.Own {
			own = append(own, c.Name)
		}
	}
	if !slices.Equal(names, []string{"Sadl", "Basti", "Tobbi"}) {
		t.Errorf("expected columns in st.Players() order, got %v", names)
	}
	if !slices.Equal(own, []string{"Basti"}) {
		t.Errorf("expected only Basti's column marked own, got %v", own)
	}
	for _, r := range v.Table.Rows {
		for i, c := range r.Cells {
			if c.Own != (i == 1) {
				t.Errorf("row %q cell %d: own=%t, want %t", r.Label, i, c.Own, i == 1)
			}
		}
	}
	if v.Table.ColumnCount != 3 {
		t.Errorf("expected ColumnCount 3, got %d", v.Table.ColumnCount)
	}
}

func TestBuildCompareShouldMarkNoColumnForAStalePlayer(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	v := buildCompare(st, "ghost", "cup", compareNow)

	for _, c := range v.Table.Columns {
		if c.Own {
			t.Errorf("expected no own column for a stale player, got %q", c.Name)
		}
	}
	if len(v.Table.Columns) != 3 {
		t.Errorf("expected all three columns, got %d", len(v.Table.Columns))
	}
}

func TestBuildCompareShouldLabelOneRowPerCategoryOfTheSet(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	tests := []struct {
		setID string
		want  []string
	}{
		{"cup", []string{"Stanley Cup winner"}},
		{"presidents", []string{"Presidents' Trophy"}},
		{"playoffcup", []string{"Stanley Cup winner"}},
		{"divisions", []string{
			"Atlantic — playoff teams", "Atlantic — winner",
			"Metropolitan — playoff teams", "Metropolitan — winner",
			"Central — playoff teams", "Central — winner",
			"Pacific — playoff teams", "Pacific — winner",
		}},
		{"awards", []string{"Hart Trophy finalists", "Norris Trophy finalists", "Vezina Trophy finalists", "Art Ross finalists", "Rocket Richard finalists"}},
		{"r1", []string{"Eastern · FLA vs TOR", "Western · COL vs VGK"}},
		{"scf", []string{"Stanley Cup Final · FLA vs COL"}},
	}
	for _, tt := range tests {
		t.Run(tt.setID, func(t *testing.T) {
			v := buildCompare(st, "basti", tt.setID, compareNow)

			if got := rowLabels(v.Table); !slices.Equal(got, tt.want) {
				t.Errorf("expected rows %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBuildCompareShouldHaveNoRowsForASetOfUnknownKind(t *testing.T) {
	st := newCompareStore(t, compareSetSeed("stub", "Stub", phaseBeforeSeason, "2026-10-06T17:00:00Z", false), "")

	v := buildCompare(st, "basti", "stub", compareNow)

	if v.Table == nil || len(v.Table.Rows) != 0 {
		t.Errorf("expected a table with no rows, got %+v", v.Table)
	}
}

func TestBuildCompareShouldShowEveryPlayersSingleTeamPickByFullName(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: cup, team_id: FLA"),
		comparePick("tobbi", "kind: cup, team_id: XYZ"),
		comparePick("basti", "kind: presidents, team_id: TOR"),
	)

	v := buildCompare(st, "basti", "cup", compareNow)

	want := [][]string{{"Florida Panthers"}, {emptyCellValue}, {"XYZ"}}
	if got := cellValues(t, v.Table, "Stanley Cup winner"); !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestBuildCompareShouldNotMixTheSeasonAndPlayoffsCupPicks(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: cup, team_id: FLA"),
		comparePick("tobbi", "kind: playoffcup, team_id: COL"),
	)

	v := buildCompare(st, "basti", "playoffcup", compareNow)

	want := [][]string{{emptyCellValue}, {emptyCellValue}, {"Colorado Avalanche"}}
	if got := cellValues(t, v.Table, "Stanley Cup winner"); !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestBuildCompareShouldShowDivisionPicksAsAbbreviationsPerDivision(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: division_playoff_teams, division: Atlantic, team_ids: [FLA, TOR]"),
		comparePick("sadl", "kind: division_winner, division: Atlantic, team_id: FLA"),
		comparePick("tobbi", "kind: division_winner, division: Central, team_id: COL"),
	)

	v := buildCompare(st, "basti", "divisions", compareNow)

	checks := map[string][][]string{
		"Atlantic — playoff teams": {{"FLA", "TOR"}, {emptyCellValue}, {emptyCellValue}},
		"Atlantic — winner":        {{"FLA"}, {emptyCellValue}, {emptyCellValue}},
		"Central — winner":         {{emptyCellValue}, {emptyCellValue}, {"COL"}},
		"Central — playoff teams":  {{emptyCellValue}, {emptyCellValue}, {emptyCellValue}},
	}
	for label, want := range checks {
		if got := cellValues(t, v.Table, label); !slices.EqualFunc(got, want, slices.Equal[[]string]) {
			t.Errorf("%s: expected %v, got %v", label, want, got)
		}
	}
}

func TestBuildCompareShouldShowFinalistsByDisplayNameOnePerLine(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("basti", "kind: award, award: hart, finalist_slugs: [mcdavid-connor, mackinnon-nathan, gone-player]"),
	)

	v := buildCompare(st, "basti", "awards", compareNow)

	want := [][]string{{emptyCellValue}, {"Connor McDavid", "Nathan MacKinnon", "gone-player"}, {emptyCellValue}}
	if got := cellValues(t, v.Table, "Hart Trophy finalists"); !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("expected %v, got %v", want, got)
	}
	for _, r := range v.Table.Rows {
		if !r.Stacked {
			t.Errorf("expected award row %q to stack its values", r.Label)
		}
	}
}

func TestBuildCompareShouldNotStackSingleValueRows(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	for _, id := range []string{"cup", "divisions", "r1"} {
		v := buildCompare(st, "basti", id, compareNow)
		for _, r := range v.Table.Rows {
			if r.Stacked {
				t.Errorf("%s: expected row %q not to stack", id, r.Label)
			}
		}
	}
}

func TestBuildCompareShouldShowSeriesPicksAsWinnerAndGames(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: series, series_key: r1.s1, team_id: FLA, games: \"5\""),
		comparePick("tobbi", "kind: series, series_key: r1.s2, team_id: VGK, games: \"7\""),
		comparePick("basti", "kind: series, series_key: scf.s1, team_id: COL, games: \"4\""),
	)

	v := buildCompare(st, "basti", "r1", compareNow)

	if got, want := cellValues(t, v.Table, "Eastern · FLA vs TOR"), [][]string{{"FLA in 5"}, {emptyCellValue}, {emptyCellValue}}; !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("expected %v, got %v", want, got)
	}
	if got, want := cellValues(t, v.Table, "Western · COL vs VGK"), [][]string{{emptyCellValue}, {emptyCellValue}, {"VGK in 7"}}; !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestBuildCompareShouldLabelASeriesWithAnUnknownTeamWithoutAConference(t *testing.T) {
	sets := compareSetSeed("r1", "Playoff round 1", phasePlayoffs, "2027-04-18T16:00:00Z", false)
	st := newCompareStore(t, sets, "playoff_matchups:\n    r1:\n        - {key: s1, a: XXX, b: TOR}\n")

	v := buildCompare(st, "basti", "r1", compareNow)

	if got := rowLabels(v.Table); !slices.Equal(got, []string{"XXX vs TOR"}) {
		t.Errorf("expected a conference-less label, got %v", got)
	}
}

func TestCompareShouldNotImportScoringOrStandings(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "compare.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse compare.go: %v", err)
	}
	forbidden := []string{
		"github.com/sommerfeld-io/fantasy-hockey/internal/scoring",
		"github.com/sommerfeld-io/fantasy-hockey/internal/standings",
	}
	for _, imp := range parsed.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatalf("unquote import %s: %v", imp.Path.Value, err)
		}
		if slices.Contains(forbidden, path) {
			t.Errorf("compare.go imports %s; Compare is display-only over the store", path)
		}
	}
}

// getCompare renders path for playerID and returns the body.
func getCompare(t *testing.T, st *store.Store, playerID, path string) string {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.AddCookie(auth.IssueSessionCookie(playerID, testSecret))
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	return rec.Body.String()
}

func TestCompareRouteShouldRenderTheRequestedSet(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: presidents, team_id: TOR"),
	)

	body := getCompare(t, st, "basti", "/compare?set=presidents")

	for _, want := range []string{
		"Everyone's picks.",
		`<a id="compare-chip-presidents" href="/compare?set=presidents" class="chip compare-chip chip--selected" aria-current="true">`,
		`<a id="compare-chip-cup" href="/compare?set=cup" class="chip compare-chip">`,
		`<span class="cmp-value">Toronto Maple Leafs</span>`,
		`<th scope="col" class="cmp-player cmp-player--own"><span class="cmp-player-name">Basti</span><span class="cmp-you">You</span></th>`,
		`<th scope="col" class="cmp-player"><span class="cmp-player-name">Sadl</span></th>`,
		`<th colspan="3" scope="rowgroup" class="cmp-label">Presidents&#39; Trophy</th>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected the Compare page to contain %q, got %q", want, body)
		}
	}
	if strings.Contains(body, "coming soon") {
		t.Errorf("expected the coming-soon placeholder to be gone, got %q", body)
	}
}

func TestCompareRouteShouldNotWriteToTheStore(t *testing.T) {
	st, dir := newSeededStore(t, compareSeedPlayers+"prediction_sets:\n"+compareDefaultSets+compareDefaultMatchups+"predictions:\n")
	before := readDataFile(t, dir)

	getCompare(t, st, "basti", "/compare?set=divisions")

	if after := readDataFile(t, dir); after != before {
		t.Errorf("expected Compare to leave the data file untouched")
	}
}

// readDataFile returns the seeded data file's contents in dir.
func readDataFile(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, store.DataFileName))
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	return string(data)
}

func TestCompareRouteShouldRenderBothEmptyGroupsAndNoTableWhenNothingIsSelectable(t *testing.T) {
	sets := compareSetSeed("cup", "Cup champion", phaseBeforeSeason, "2026-10-06T17:00:00Z", true) +
		compareSetSeed("playoffcup", "Playoffs Cup pick", phasePlayoffs, "2027-04-15T16:00:00Z", true)
	st := newCompareStore(t, sets, "")

	body := getCompare(t, st, "basti", "/compare")

	for _, want := range []string{
		`<span id="compare-group-before_season" class="compare-group-label">Before the season</span><span class="chip-group compare-chips" role="group" aria-labelledby="compare-group-before_season"><span class="compare-none">Nothing to compare yet.</span></span>`,
		`<span id="compare-group-playoffs" class="compare-group-label">Playoffs</span><span class="chip-group compare-chips" role="group" aria-labelledby="compare-group-playoffs"><span class="compare-none">Nothing to compare yet.</span></span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected the empty group %q, got %q", want, body)
		}
	}
	if strings.Contains(body, "cmp-table") {
		t.Errorf("expected no table when nothing is selectable, got %q", body)
	}
}

func TestCompareRouteShouldShowTheEmptyNoteOnlyUnderAnEmptyPlayoffsGroup(t *testing.T) {
	sets := compareSetSeed("cup", "Cup champion", phaseBeforeSeason, "2026-10-06T17:00:00Z", false) +
		compareSetSeed("playoffcup", "Playoffs Cup pick", phasePlayoffs, "2027-04-15T16:00:00Z", true)
	st := newCompareStore(t, sets, "")

	body := getCompare(t, st, "basti", "/compare")

	want := `aria-labelledby="compare-group-playoffs"><span class="compare-none">Nothing to compare yet.</span></span>`
	if !strings.Contains(body, want) {
		t.Errorf("expected the note under Playoffs, got %q", body)
	}
	if n := strings.Count(body, "Nothing to compare yet."); n != 1 {
		t.Errorf("expected the note exactly once, got %d", n)
	}
	if !strings.Contains(body, `<table class="cmp-table">`) {
		t.Errorf("expected a table for the selectable cup set, got %q", body)
	}
}

func TestCompareRouteShouldStackOnlyAwardRows(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups)

	if awards := getCompare(t, st, "basti", "/compare?set=awards"); strings.Count(awards, `<tr class="cmp-values cmp-values--stacked">`) != len(awardOrder) {
		t.Errorf("expected every award value row stacked, got %q", awards)
	}
	cup := getCompare(t, st, "basti", "/compare?set=cup")
	if strings.Contains(cup, "cmp-values--stacked") {
		t.Errorf("expected no stacked rows for cup, got %q", cup)
	}
	if !strings.Contains(cup, `<tr class="cmp-values">`) {
		t.Errorf("expected a plain value row for cup, got %q", cup)
	}
}

func TestCompareRouteShouldRenderTheCountdown(t *testing.T) {
	deadline := time.Now().UTC().Add(5*24*time.Hour + time.Hour).Truncate(time.Second)
	st := newCompareStore(t, compareSetSeed("cup", "Cup champion", phaseBeforeSeason, deadline.Format(time.RFC3339), false), "")

	body := getCompare(t, st, "basti", "/compare?set=cup")

	want := `<span class="compare-countdown">&nbsp;&middot; ` + clock.Countdown(deadline, time.Now()) + `</span>`
	if !strings.Contains(body, want) {
		t.Errorf("expected the countdown %q, got %q", want, body)
	}
}
