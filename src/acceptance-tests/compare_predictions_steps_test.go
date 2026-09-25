package acceptance_test

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// compareTeams maps every team the Compare scenarios pick or match up to its
// division, declared under teams: so every id resolves to a canonical team.
var compareTeams = map[string]string{
	"FLA": "Atlantic", "TOR": "Atlantic", "BOS": "Atlantic", "TBL": "Atlantic",
	"COL": "Central", "VGK": "Pacific",
}

// compareNHLPlayers is every finalist slug the Compare scenarios pick.
var compareNHLPlayers = []string{"mcdavid-connor", "mackinnon-nathan", "kucherov-nikita"}

// compareYouLabel is the own column's text marker.
const compareYouLabel = `<span class="cmp-you">You</span>`

// compareStalePlayerID is a session player id no seeded player has.
const compareStalePlayerID = "ghost"

// Markup patterns for the rendered Compare section: selector groups and
// their chips, the deadline, the player header and the category rows.
var (
	compareGroupPattern      = regexp.MustCompile(`(?s)<div class="compare-group"><span id="[^"]*" class="compare-group-label">([^<]+)</span>(.*?)</div>`)
	compareChipPattern       = regexp.MustCompile(`<a id="compare-chip-([^"]+)" href="([^"]+)" class="chip compare-chip( chip--selected)?"[^>]*>([^<]+)</a>`)
	compareDeadlinePattern   = regexp.MustCompile(`<span class="compare-deadline-text">([^<]+)</span>`)
	compareHeaderPattern     = regexp.MustCompile(`<th scope="col" class="cmp-player( cmp-player--own)?"><span class="cmp-player-name">([^<]+)</span>`)
	compareLabelPattern      = regexp.MustCompile(`<th colspan="\d+" scope="rowgroup" class="cmp-label">([^<]+)</th>`)
	compareValueRowPattern   = regexp.MustCompile(`(?s)<tr class="cmp-values[^"]*">(.*?)</tr>`)
	compareCellPattern       = regexp.MustCompile(`(?s)<td class="cmp-cell( cmp-cell--own)?">(.*?)</td>`)
	compareCellValuePattern  = regexp.MustCompile(`<span class="cmp-value">([^<]*)</span>`)
	compareSectionHeaderText = regexp.MustCompile(`<div class="section-header">.*?<span>([^<]+)</span></div>`)
)

// compareSeedSet is one seeded prediction_sets entry. deadline is written
// verbatim, so a scenario can seed an unparseable value.
type compareSeedSet struct {
	id, title, phase, deadline string
	upcoming                   bool
}

// compareCell is one rendered table cell: its values and own-column flag.
type compareCell struct {
	values []string
	own    bool
}

// compareScenarioState seeds the pool's players, Prediction Sets, matchups
// and picks, and requests /compare over HTTP as one of those players.
type compareScenarioState struct {
	lazyFixture
	poolNames []string
	sets      []compareSeedSet
	deadlines map[string]time.Time
	matchups  map[string][]seedMatchup
	picks     []seedPrediction
}

func newCompareScenarioState() *compareScenarioState {
	s := &compareScenarioState{deadlines: map[string]time.Time{}, matchups: map[string][]seedMatchup{}}
	s.lazyFixture = newLazyFixture("compare", "basti", "Basti", testSessionSecret, s.seedBody, nil)
	return s
}

// comparePlayerID derives a pool player's id from their display name.
func comparePlayerID(name string) string {
	return strings.ToLower(name)
}

func (s *compareScenarioState) requirePoolPlayer(name string) error {
	if !slices.Contains(s.poolNames, name) {
		return fmt.Errorf("no compare pool player %q; seeded players are %v", name, s.poolNames)
	}
	return nil
}

// seedBody writes every pool player other than the signed-in one (whom
// seedHeader already declares), then the sets, teams, players, matchups and
// picks.
func (s *compareScenarioState) seedBody() (string, error) {
	var b strings.Builder
	for _, name := range s.poolNames {
		id := comparePlayerID(name)
		if id == s.playerID {
			continue
		}
		fmt.Fprintf(&b, "    - id: %s\n      name: %s\n      email: %s@pool.example\n", id, name, id)
	}
	b.WriteString("prediction_sets:\n")
	for _, set := range s.sets {
		fmt.Fprintf(&b, "    - {id: %s, title: %q, subtitle: \"\", deadline_utc: %q, phase: %s, upcoming: %t}\n",
			set.id, set.title, set.deadline, set.phase, set.upcoming)
	}
	writeSeedTeams(&b, compareTeams)
	writeSeedNHLPlayers(&b, compareNHLPlayers)
	writeSeedMatchups(&b, s.matchups)
	writeSeedPredictions(&b, s.picks)
	return b.String(), nil
}

func (s *compareScenarioState) thePoolPlayersAre(first, second, third string) error {
	s.poolNames = []string{first, second, third}
	return nil
}

func (s *compareScenarioState) addSet(id, title, phase, phrase string, upcoming bool) error {
	deadline, err := parseRelativeDeadline(phrase, time.Now())
	if err != nil {
		return err
	}
	deadline = deadline.UTC().Truncate(time.Second)
	s.deadlines[id] = deadline
	s.sets = append(s.sets, compareSeedSet{id: id, title: title, phase: phase, deadline: deadline.Format(time.RFC3339), upcoming: upcoming})
	return nil
}

func (s *compareScenarioState) aPredictionSet(id, title, phase, phrase string) error {
	return s.addSet(id, title, phase, phrase, false)
}

func (s *compareScenarioState) anUpcomingPredictionSet(id, title, phase, phrase string) error {
	return s.addSet(id, title, phase, phrase, true)
}

func (s *compareScenarioState) aPredictionSetWithRawDeadline(id, title, phase, raw string) error {
	s.sets = append(s.sets, compareSeedSet{id: id, title: title, phase: phase, deadline: raw})
	return nil
}

// compareMatchupPattern reads one "s1: FLA vs TOR" matchup phrase.
var compareMatchupPattern = regexp.MustCompile(`^(\S+): (\S+) vs (\S+)$`)

func (s *compareScenarioState) theRoundMatchupsAre(round, first, second string) error {
	setID := mustSeedRound(round).setID
	for _, phrase := range []string{first, second} {
		m := compareMatchupPattern.FindStringSubmatch(phrase)
		if m == nil {
			return fmt.Errorf("unrecognized matchup %q; want \"s1: FLA vs TOR\"", phrase)
		}
		s.matchups[setID] = append(s.matchups[setID], seedMatchup{key: m[1], a: m[2], b: m[3]})
	}
	return nil
}

func (s *compareScenarioState) addPick(name, fields string) error {
	if err := s.requirePoolPlayer(name); err != nil {
		return err
	}
	s.picks = append(s.picks, seedPrediction{playerID: comparePlayerID(name), fields: fields})
	return nil
}

func (s *compareScenarioState) pickedFor(name, team, kind string) error {
	return s.addPick(name, fmt.Sprintf("kind: %s, team_id: %s", kind, team))
}

func (s *compareScenarioState) pickedPlayoffTeams(name, list, division string) error {
	return s.addPick(name, fmt.Sprintf("kind: %s, division: %s, team_ids: [%s]", store.KindDivisionPlayoffTeams, division, strings.Join(splitList(list), ", ")))
}

func (s *compareScenarioState) pickedDivisionWinner(name, team, division string) error {
	return s.addPick(name, fmt.Sprintf("kind: %s, division: %s, team_id: %s", store.KindDivisionWinner, division, team))
}

func (s *compareScenarioState) pickedFinalists(name, list, award string) error {
	return s.addPick(name, fmt.Sprintf("kind: %s, award: %s, finalist_slugs: [%s]", store.KindAward, award, strings.Join(splitList(list), ", ")))
}

func (s *compareScenarioState) pickedRound1Series(name, team string, games int, key string) error {
	return s.addPick(name, fmt.Sprintf("kind: %s, series_key: %s, team_id: %s, games: \"%d\"", store.KindSeries, store.JoinSeriesKey(store.Round1SetID, key), team, games))
}

// open signs in as name (the first request decides whom seedHeader
// declares) and requests path, expecting a 200.
func (s *compareScenarioState) open(name, path string) error {
	if err := s.requirePoolPlayer(name); err != nil {
		return err
	}
	s.playerID, s.playerName = comparePlayerID(name), name
	return s.get(path)
}

func (s *compareScenarioState) get(path string) error {
	if err := s.do(http.MethodGet, path, ""); err != nil {
		return err
	}
	if s.lastStatus != http.StatusOK {
		return fmt.Errorf("expected status %d for %s, got %d", http.StatusOK, path, s.lastStatus)
	}
	return nil
}

func (s *compareScenarioState) opensCompare(name string) error {
	return s.open(name, "/compare")
}

func (s *compareScenarioState) opensCompareWithTheSet(name, setID string) error {
	return s.open(name, "/compare?set="+setID)
}

// aStalePlayerOpensCompare seeds the fixture as usual, then requests
// /compare with a session whose player id no seeded player has.
func (s *compareScenarioState) aStalePlayerOpensCompare() error {
	if err := s.ensureReady(); err != nil {
		return err
	}
	s.playerID = compareStalePlayerID
	return s.get("/compare")
}

// chip is one rendered selector chip.
type chip struct {
	id, href, title string
	selected        bool
}

func parseChips(markup string) []chip {
	var chips []chip
	for _, m := range compareChipPattern.FindAllStringSubmatch(markup, -1) {
		chips = append(chips, chip{id: m[1], href: html.UnescapeString(m[2]), selected: m[3] != "", title: html.UnescapeString(m[4])})
	}
	return chips
}

func (s *compareScenarioState) selectsTheChip(name, title string) error {
	for _, c := range parseChips(s.lastBody) {
		if c.title == title {
			return s.open(name, c.href)
		}
	}
	return fmt.Errorf("no Compare chip titled %q in %q", title, s.lastBody)
}

func (s *compareScenarioState) theSectionHeaderReads(text string) error {
	for _, m := range compareSectionHeaderText.FindAllStringSubmatch(s.lastBody, -1) {
		if html.UnescapeString(m[1]) == text {
			return nil
		}
	}
	return fmt.Errorf("expected a section header reading %q, got %q", text, s.lastBody)
}

func (s *compareScenarioState) theRowShowsTheChips(label, list string) error {
	for _, m := range compareGroupPattern.FindAllStringSubmatch(s.lastBody, -1) {
		if html.UnescapeString(m[1]) != label {
			continue
		}
		var got []string
		for _, c := range parseChips(m[2]) {
			got = append(got, c.title)
		}
		if want := strings.Split(list, ", "); !slices.Equal(got, want) {
			return fmt.Errorf("expected the %q row to show the chips %v, got %v", label, want, got)
		}
		return nil
	}
	return fmt.Errorf("no Compare selector row labelled %q in %q", label, s.lastBody)
}

func (s *compareScenarioState) onlyTheChipIsSelected(title string) error {
	var selected []string
	for _, c := range parseChips(s.lastBody) {
		if c.selected {
			selected = append(selected, c.title)
		}
	}
	if !slices.Equal(selected, []string{title}) {
		return fmt.Errorf("expected only the %q chip selected, got %v", title, selected)
	}
	return nil
}

func (s *compareScenarioState) theDeadlineIsTheDeadlineOf(setID string) error {
	deadline, ok := s.deadlines[setID]
	if !ok {
		return fmt.Errorf("no seeded deadline for %q", setID)
	}
	m := compareDeadlinePattern.FindStringSubmatch(s.lastBody)
	if m == nil {
		return fmt.Errorf("no Compare deadline in %q", s.lastBody)
	}
	if want := clock.FormatDeadline(deadline); html.UnescapeString(m[1]) != want {
		return fmt.Errorf("expected the Compare deadline %q, got %q", want, m[1])
	}
	return nil
}

// renderedHeader returns the player header's names and which are own.
func (s *compareScenarioState) renderedHeader() (names []string, own []bool) {
	for _, m := range compareHeaderPattern.FindAllStringSubmatch(s.lastBody, -1) {
		names = append(names, html.UnescapeString(m[2]))
		own = append(own, m[1] != "")
	}
	return names, own
}

// renderedRows returns every category row: its label and its cells.
func (s *compareScenarioState) renderedRows() (labels []string, rows [][]compareCell) {
	for _, m := range compareLabelPattern.FindAllStringSubmatch(s.lastBody, -1) {
		labels = append(labels, html.UnescapeString(m[1]))
	}
	for _, m := range compareValueRowPattern.FindAllStringSubmatch(s.lastBody, -1) {
		var cells []compareCell
		for _, c := range compareCellPattern.FindAllStringSubmatch(m[1], -1) {
			var values []string
			for _, v := range compareCellValuePattern.FindAllStringSubmatch(c[2], -1) {
				values = append(values, html.UnescapeString(v[1]))
			}
			cells = append(cells, compareCell{values: values, own: c[1] != ""})
		}
		rows = append(rows, cells)
	}
	return labels, rows
}

// renderedTable flattens the header and category rows into the same shape
// as a feature table: a "category" header row, then one row per category
// with multiple values joined by ", ".
func (s *compareScenarioState) renderedTable() [][]string {
	names, _ := s.renderedHeader()
	table := [][]string{append([]string{"category"}, names...)}
	labels, rows := s.renderedRows()
	for i, label := range labels {
		row := []string{label}
		if i < len(rows) {
			for _, c := range rows[i] {
				row = append(row, strings.Join(c.values, ", "))
			}
		}
		table = append(table, row)
	}
	return table
}

func (s *compareScenarioState) theTableShows(table *godog.Table) error {
	want := make([][]string, len(table.Rows))
	for i, row := range table.Rows {
		for _, c := range row.Cells {
			want[i] = append(want[i], c.Value)
		}
	}
	got := s.renderedTable()
	if !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		return fmt.Errorf("expected the Compare table\n%v\ngot\n%v", want, got)
	}
	return nil
}

// ownColumns returns the names whose header is marked own, and fails if a
// row's cell count differs from the header's or any value cell's own flag
// disagrees with its column's header.
func (s *compareScenarioState) ownColumns() ([]string, error) {
	names, own := s.renderedHeader()
	var marked []string
	for i, name := range names {
		if own[i] {
			marked = append(marked, name)
		}
	}
	_, rows := s.renderedRows()
	for r, cells := range rows {
		if len(cells) != len(names) {
			return nil, fmt.Errorf("row %d has %d cells but the header has %d columns", r+1, len(cells), len(names))
		}
		for i, c := range cells {
			if c.own != own[i] {
				return nil, fmt.Errorf("row %d, column %q: cell own=%t but header own=%t", r+1, names[i], c.own, own[i])
			}
		}
	}
	return marked, nil
}

func (s *compareScenarioState) onlyTheColumnIsOwn(name string) error {
	marked, err := s.ownColumns()
	if err != nil {
		return err
	}
	if !slices.Equal(marked, []string{name}) {
		return fmt.Errorf("expected only %q's column marked as own, got %v", name, marked)
	}
	if !strings.Contains(s.lastBody, compareYouLabel) {
		return fmt.Errorf("expected the own column to carry a text marker, not colour alone, got %q", s.lastBody)
	}
	return nil
}

func (s *compareScenarioState) noColumnIsOwn() error {
	marked, err := s.ownColumns()
	if err != nil {
		return err
	}
	if len(marked) != 0 {
		return fmt.Errorf("expected no column marked as own, got %v", marked)
	}
	if strings.Contains(s.lastBody, compareYouLabel) {
		return fmt.Errorf("expected no %q label when no column is own, got %q", compareYouLabel, s.lastBody)
	}
	if names, _ := s.renderedHeader(); len(names) == 0 {
		return fmt.Errorf("expected the Compare table to render player columns, got %q", s.lastBody)
	}
	return nil
}

// InitializeCompareScenario registers the Compare step definitions with
// GoDog.
func InitializeCompareScenario(ctx *godog.ScenarioContext) {
	s := newCompareScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the compare pool players are "([^"]*)", "([^"]*)" and "([^"]*)"$`, s.thePoolPlayersAre)
	ctx.Step(`^a compare Prediction Set "([^"]*)" titled "([^"]*)" in phase "([^"]*)" with a deadline "([^"]*)"$`, s.aPredictionSet)
	ctx.Step(`^an upcoming compare Prediction Set "([^"]*)" titled "([^"]*)" in phase "([^"]*)" with a deadline "([^"]*)"$`, s.anUpcomingPredictionSet)
	ctx.Step(`^a compare Prediction Set "([^"]*)" titled "([^"]*)" in phase "([^"]*)" with the raw deadline "([^"]*)"$`, s.aPredictionSetWithRawDeadline)
	ctx.Step(`^the (round \d) matchups for compare are "([^"]*)" and "([^"]*)"$`, s.theRoundMatchupsAre)
	ctx.Step(`^"([^"]*)" picked "([^"]*)" for the compare "([^"]*)" pick$`, s.pickedFor)
	ctx.Step(`^"([^"]*)" picked "([^"]*)" as the compare "([^"]*)" playoff teams$`, s.pickedPlayoffTeams)
	ctx.Step(`^"([^"]*)" picked "([^"]*)" as the compare "([^"]*)" division winner$`, s.pickedDivisionWinner)
	ctx.Step(`^"([^"]*)" picked "([^"]*)" as the compare "([^"]*)" finalists$`, s.pickedFinalists)
	ctx.Step(`^"([^"]*)" picked "([^"]*)" in (\d+) games for compare round 1 series "([^"]*)"$`, s.pickedRound1Series)
	ctx.Step(`^"([^"]*)" opens Compare$`, s.opensCompare)
	ctx.Step(`^"([^"]*)" opens Compare with the set "([^"]*)"$`, s.opensCompareWithTheSet)
	ctx.Step(`^a player whose account no longer exists opens Compare$`, s.aStalePlayerOpensCompare)
	ctx.Step(`^"([^"]*)" selects the "([^"]*)" chip on Compare$`, s.selectsTheChip)
	ctx.Step(`^the Compare section header reads "([^"]*)"$`, s.theSectionHeaderReads)
	ctx.Step(`^the Compare "([^"]*)" row shows the chips "([^"]*)"$`, s.theRowShowsTheChips)
	ctx.Step(`^only the "([^"]*)" chip is selected on Compare$`, s.onlyTheChipIsSelected)
	ctx.Step(`^the Compare deadline is the deadline of "([^"]*)"$`, s.theDeadlineIsTheDeadlineOf)
	ctx.Step(`^the Compare table shows:$`, s.theTableShows)
	ctx.Step(`^only the Compare column for "([^"]*)" is marked as the signed-in player's own$`, s.onlyTheColumnIsOwn)
	ctx.Step(`^no Compare column is marked as the signed-in player's own$`, s.noColumnIsOwn)
}
