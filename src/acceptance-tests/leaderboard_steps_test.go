package acceptance_test

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// leaderboardSubmittedAt stamps every seeded prediction row; neither
// scoring nor the Leaderboard reads it.
const leaderboardSubmittedAt = "2026-09-20T10:00:00Z"

// leaderboardTeams is every team the Leaderboard scenarios pick or record,
// declared under teams: so the store never flags one as unknown.
var leaderboardTeams = []string{"FLA", "TOR"}

// leaderboardRowPattern isolates one rendered Leaderboard row: the player
// id from its element id, and everything up to its closing tag.
var leaderboardRowPattern = regexp.MustCompile(`(?s)<tr id="leaderboard-row-([^"]+)" class="lb-row">(.*?)</tr>`)

// leaderboardCellsPattern reads one row's cells: the rank badge (with its
// optional gold modifier), the name, Regular, Playoff and the Total (with
// its optional gold modifier).
var leaderboardCellsPattern = regexp.MustCompile(`(?s)<span class="rank-badge( rank-badge--gold)?">(\d+)</span>\s*<span class="lb-name">([^<]+)</span>.*?<td class="lb-num">(\d+)</td>\s*<td class="lb-num">(\d+)</td>\s*<td class="lb-total( lb-total--gold)?">(\d+)</td>`)

// leaderboardPick is one seeded single-team prediction row.
type leaderboardPick struct {
	playerID, kind, teamID string
}

// leaderboardRow is one Leaderboard row as rendered.
type leaderboardRow struct {
	playerID, rank, name, regular, playoff, total string
	badgeGold, totalGold                          bool
}

// leaderboardScenarioState seeds the pool's players, picks and results and
// requests /leaderboard over HTTP as one of those players.
type leaderboardScenarioState struct {
	lazyFixture
	poolNames  []string
	picks      []leaderboardPick
	cupWinner  string
	presidents string
}

func newLeaderboardScenarioState() *leaderboardScenarioState {
	s := &leaderboardScenarioState{}
	s.lazyFixture = newLazyFixture("leaderboard", "basti", "Basti", testSessionSecret, s.seedBody, nil)
	return s
}

// leaderboardPlayerID derives a pool player's id from their display name.
func leaderboardPlayerID(name string) string {
	return strings.ToLower(name)
}

// requirePoolPlayer fails unless name is one of the seeded pool players.
func (s *leaderboardScenarioState) requirePoolPlayer(name string) error {
	if !slices.Contains(s.poolNames, name) {
		return fmt.Errorf("no pool player %q; seeded players are %v", name, s.poolNames)
	}
	return nil
}

// seedBody writes every pool player other than the signed-in one (whom
// seedHeader already declares), then the teams, picks and results.
func (s *leaderboardScenarioState) seedBody() (string, error) {
	var b strings.Builder
	for _, name := range s.poolNames {
		id := leaderboardPlayerID(name)
		if id == s.playerID {
			continue
		}
		fmt.Fprintf(&b, "    - id: %s\n      name: %s\n      email: %s@pool.example\n", id, name, id)
	}
	b.WriteString("teams:\n")
	for _, id := range leaderboardTeams {
		fmt.Fprintf(&b, "    - id: %s\n      name: Team %s\n      conference: Eastern\n      division: Atlantic\n", id, id)
	}
	b.WriteString("predictions:\n")
	for i, p := range s.picks {
		fmt.Fprintf(&b, "    - id: p%d\n      player_id: %s\n      submitted_at: %q\n      kind: %s\n      team_id: %s\n",
			i+1, p.playerID, leaderboardSubmittedAt, p.kind, p.teamID)
	}
	b.WriteString("results:\n")
	fmt.Fprintf(&b, "    presidents_trophy: %s\n    stanley_cup_winner: %s\n", s.presidents, s.cupWinner)
	return b.String(), nil
}

func (s *leaderboardScenarioState) thePoolPlayersAre(first, second, third string) error {
	s.poolNames = []string{first, second, third}
	return nil
}

func (s *leaderboardScenarioState) theRecordedStanleyCupWinnerIs(team string) error {
	s.cupWinner = team
	return nil
}

func (s *leaderboardScenarioState) theRecordedPresidentsTrophyWinnerIs(team string) error {
	s.presidents = team
	return nil
}

func (s *leaderboardScenarioState) pickedFor(name, team, kind string) error {
	if err := s.requirePoolPlayer(name); err != nil {
		return err
	}
	s.picks = append(s.picks, leaderboardPick{playerID: leaderboardPlayerID(name), kind: kind, teamID: team})
	return nil
}

// opensTheLeaderboard signs in as name and requests /leaderboard. The first
// request decides which player seedHeader declares; a later one only
// changes whose session cookie is sent.
func (s *leaderboardScenarioState) opensTheLeaderboard(name string) error {
	if err := s.requirePoolPlayer(name); err != nil {
		return err
	}
	s.playerID, s.playerName = leaderboardPlayerID(name), name
	if err := s.do(http.MethodGet, "/leaderboard", ""); err != nil {
		return err
	}
	if s.lastStatus != http.StatusOK {
		return fmt.Errorf("expected status %d for /leaderboard, got %d", http.StatusOK, s.lastStatus)
	}
	return nil
}

// theCupWinnerIsChangedByHandAndTheAppRestarts changes the recorded Cup
// winner and stops the server, so the next request regenerates the whole
// data file from the seed (dropping anything the app wrote) and reopens the
// store on it - standing in for a hand edit plus restart.
func (s *leaderboardScenarioState) theCupWinnerIsChangedByHandAndTheAppRestarts(team string) error {
	s.cupWinner = team
	if s.server != nil {
		s.server.Close()
		s.server = nil
	}
	return nil
}

func (s *leaderboardScenarioState) savesWhileTheAppRuns(name, team, kind string) error {
	if err := s.requirePoolPlayer(name); err != nil {
		return err
	}
	if err := s.ensureReady(); err != nil {
		return err
	}
	return s.st.SavePrediction(leaderboardPlayerID(name), kind, team, time.Now())
}

// renderedRows parses every Leaderboard row in the last response, in order.
func (s *leaderboardScenarioState) renderedRows() ([]leaderboardRow, error) {
	var rows []leaderboardRow
	for _, m := range leaderboardRowPattern.FindAllStringSubmatch(s.lastBody, -1) {
		cells := leaderboardCellsPattern.FindStringSubmatch(m[2])
		if cells == nil {
			return nil, fmt.Errorf("could not read the cells of the Leaderboard row for %q: %q", m[1], m[2])
		}
		rows = append(rows, leaderboardRow{
			playerID:  m[1],
			badgeGold: cells[1] != "",
			rank:      cells[2],
			name:      cells[3],
			regular:   cells[4],
			playoff:   cells[5],
			totalGold: cells[6] != "",
			total:     cells[7],
		})
	}
	return rows, nil
}

func (s *leaderboardScenarioState) theLeaderboardShowsTheseRowsInOrder(table *godog.Table) error {
	rows, err := s.renderedRows()
	if err != nil {
		return err
	}
	want := table.Rows[1:]
	if len(rows) != len(want) {
		return fmt.Errorf("expected %d Leaderboard rows, got %d in %q", len(want), len(rows), s.lastBody)
	}
	for i, w := range want {
		cells := make([]string, len(w.Cells))
		for j, c := range w.Cells {
			cells[j] = c.Value
		}
		if err := compareLeaderboardRow(i+1, rows[i], cells); err != nil {
			return err
		}
	}
	return nil
}

// compareLeaderboardRow checks one rendered row against its expected table
// row (rank, player, regular, playoff, total, gold).
func compareLeaderboardRow(position int, got leaderboardRow, want []string) error {
	gold := want[5] == "yes"
	gotCells := []string{got.rank, got.name, got.regular, got.playoff, got.total}
	if !slices.Equal(gotCells, want[:5]) {
		return fmt.Errorf("row %d: expected rank/player/regular/playoff/total %v, got %v", position, want[:5], gotCells)
	}
	if got.badgeGold != gold || got.totalGold != gold {
		return fmt.Errorf("row %d (%s): expected gold=%t on badge and Total, got badge=%t Total=%t",
			position, got.name, gold, got.badgeGold, got.totalGold)
	}
	return nil
}

func (s *leaderboardScenarioState) noLeaderboardRowIsMarked(marker string) error {
	if strings.Contains(s.lastBody, marker) {
		return fmt.Errorf("expected no %q marker on the Leaderboard, got %q", marker, s.lastBody)
	}
	return nil
}

// rowMarkupFor returns name's rendered row with its id and name replaced by
// placeholders, so two rows can be compared for identical markup.
func (s *leaderboardScenarioState) rowMarkupFor(name string) (string, error) {
	id := leaderboardPlayerID(name)
	for _, m := range leaderboardRowPattern.FindAllStringSubmatch(s.lastBody, -1) {
		if m[1] == id {
			return strings.ReplaceAll(m[0], id, "ID"), nil
		}
	}
	return "", fmt.Errorf("no Leaderboard row for %q in %q", name, s.lastBody)
}

func (s *leaderboardScenarioState) theRowIsMarkedUpLikeTheRowFor(own, other string) error {
	ownRow, err := s.rowMarkupFor(own)
	if err != nil {
		return err
	}
	otherRow, err := s.rowMarkupFor(other)
	if err != nil {
		return err
	}
	ownRow = strings.ReplaceAll(ownRow, own, "NAME")
	otherRow = strings.ReplaceAll(otherRow, other, "NAME")
	if ownRow != otherRow {
		return fmt.Errorf("expected %q's row to be marked up like %q's, got %q vs %q", own, other, ownRow, otherRow)
	}
	return nil
}

func (s *leaderboardScenarioState) theLeaderboardShowsText(text string) error {
	if !strings.Contains(s.lastBody, text) {
		return fmt.Errorf("expected the Leaderboard to show %q, got %q", text, s.lastBody)
	}
	return nil
}

func (s *leaderboardScenarioState) theLeaderboardNoLongerShowsThePlaceholder() error {
	const placeholder = "The leaderboard is coming soon."
	if strings.Contains(s.lastBody, placeholder) {
		return fmt.Errorf("expected the coming-soon placeholder %q to be gone, got %q", placeholder, s.lastBody)
	}
	return nil
}

// InitializeLeaderboardScenario registers the Leaderboard step definitions
// with GoDog.
func InitializeLeaderboardScenario(ctx *godog.ScenarioContext) {
	s := newLeaderboardScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the pool players are "([^"]*)", "([^"]*)" and "([^"]*)"$`, s.thePoolPlayersAre)
	ctx.Step(`^the recorded Stanley Cup winner for the leaderboard is "([^"]*)"$`, s.theRecordedStanleyCupWinnerIs)
	ctx.Step(`^the recorded Presidents' Trophy winner for the leaderboard is "([^"]*)"$`, s.theRecordedPresidentsTrophyWinnerIs)
	ctx.Step(`^"([^"]*)" picked "([^"]*)" for the "([^"]*)" pick$`, s.pickedFor)
	ctx.Step(`^"([^"]*)" opens the Leaderboard$`, s.opensTheLeaderboard)
	ctx.Step(`^the recorded Stanley Cup winner is changed by hand to "([^"]*)" and the app restarts$`, s.theCupWinnerIsChangedByHandAndTheAppRestarts)
	ctx.Step(`^"([^"]*)" saves "([^"]*)" for the "([^"]*)" pick while the app runs$`, s.savesWhileTheAppRuns)
	ctx.Step(`^the Leaderboard shows these rows in order:$`, s.theLeaderboardShowsTheseRowsInOrder)
	ctx.Step(`^no Leaderboard row is marked "([^"]*)"$`, s.noLeaderboardRowIsMarked)
	ctx.Step(`^the Leaderboard row for "([^"]*)" is marked up exactly like the row for "([^"]*)"$`, s.theRowIsMarkedUpLikeTheRowFor)
	ctx.Step(`^the Leaderboard shows the (?:caption|footer) "([^"]*)"$`, s.theLeaderboardShowsText)
	ctx.Step(`^the Leaderboard no longer shows the coming-soon placeholder$`, s.theLeaderboardNoLongerShowsThePlaceholder)
}
