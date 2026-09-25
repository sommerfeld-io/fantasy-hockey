package standings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/scoring"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Picks every test combines: with FLA recorded as Cup winner and TOR as
// Presidents' Trophy winner, cup and presidents each score 20 Regular and
// playoffcup scores 20 Playoff.
const (
	cupFLA        = "kind: cup, team_id: FLA"
	presidentsTOR = "kind: presidents, team_id: TOR"
	playoffCupFLA = "kind: playoffcup, team_id: FLA"
	cupTOR        = "kind: cup, team_id: TOR"
)

// standingsFixtureResults records the results every pick above is scored
// against.
const standingsFixtureResults = `results:
    presidents_trophy: TOR
    stanley_cup_winner: FLA
`

// fixturePlayer is one seeded player and the picks they saved.
type fixturePlayer struct {
	id, name string
	picks    []string
}

// seedSubmittedAt stamps every seeded Prediction row; standings never reads
// it.
const seedSubmittedAt = "2026-09-20T10:00:00Z"

// newStandingsStore opens a store holding players in the given file order,
// their picks and standingsFixtureResults.
func newStandingsStore(t *testing.T, players []fixturePlayer) *store.Store {
	t.Helper()
	var b strings.Builder
	b.WriteString("season: \"2026-27\"\nplayers:\n")
	for _, p := range players {
		fmt.Fprintf(&b, "    - {id: %s, name: %s, email: %s@example.com}\n", p.id, p.name, p.id)
	}
	b.WriteString("teams:\n")
	b.WriteString("    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}\n")
	b.WriteString("    - {id: TOR, name: Toronto Maple Leafs, conference: Eastern, division: Atlantic}\n")
	b.WriteString("predictions:\n")
	for _, p := range players {
		for _, pick := range p.picks {
			fmt.Fprintf(&b, "    - {player_id: %s, submitted_at: %q, %s}\n", p.id, seedSubmittedAt, pick)
		}
	}
	b.WriteString(standingsFixtureResults)

	path := filepath.Join(t.TempDir(), store.DataFileName)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New returned error: %v", err)
	}
	if problems := st.ResultProblems(); len(problems) != 0 {
		t.Fatalf("fixture has result problems: %v", problems)
	}
	return st
}

func row(rank int, id, name string, regular, playoff int, leader bool) Row {
	return Row{Rank: rank, PlayerID: id, Name: name, Points: scoring.Points{Regular: regular, Playoff: playoff}, Leader: leader}
}

func TestRowsShouldRankAndOrderEveryPlayer(t *testing.T) {
	tests := []struct {
		name    string
		players []fixturePlayer
		want    []Row
	}{
		{
			name: "should rank a clear leader first and not keep file order",
			players: []fixturePlayer{
				{"tobbi", "Tobbi", []string{cupFLA}},
				{"sadl", "Sadl", []string{cupFLA, presidentsTOR}},
				{"basti", "Basti", []string{cupFLA, presidentsTOR, playoffCupFLA}},
			},
			want: []Row{
				row(1, "basti", "Basti", 40, 20, true),
				row(2, "sadl", "Sadl", 40, 0, false),
				row(3, "tobbi", "Tobbi", 20, 0, false),
			},
		},
		{
			name: "should share a rank below the top and list the tie alphabetically, not by file order",
			players: []fixturePlayer{
				{"basti", "Basti", []string{cupFLA, presidentsTOR, playoffCupFLA}},
				{"tobbi", "Tobbi", []string{cupFLA, playoffCupFLA}},
				{"sadl", "Sadl", []string{cupFLA, presidentsTOR}},
			},
			want: []Row{
				row(1, "basti", "Basti", 40, 20, true),
				row(2, "sadl", "Sadl", 40, 0, false),
				row(2, "tobbi", "Tobbi", 20, 20, false),
			},
		},
		{
			name: "should make every player tied at the top a leader and skip the next rank",
			players: []fixturePlayer{
				{"tobbi", "Tobbi", []string{cupFLA}},
				{"sadl", "Sadl", []string{cupFLA, playoffCupFLA}},
				{"basti", "Basti", []string{cupFLA, presidentsTOR}},
			},
			want: []Row{
				row(1, "basti", "Basti", 40, 0, true),
				row(1, "sadl", "Sadl", 20, 20, true),
				row(3, "tobbi", "Tobbi", 20, 0, false),
			},
		},
		{
			name: "should not make anyone a leader before anything scores",
			players: []fixturePlayer{
				{"tobbi", "Tobbi", nil},
				{"basti", "Basti", []string{cupTOR}},
				{"sadl", "Sadl", nil},
			},
			want: []Row{
				row(1, "basti", "Basti", 0, 0, false),
				row(1, "sadl", "Sadl", 0, 0, false),
				row(1, "tobbi", "Tobbi", 0, 0, false),
			},
		},
		{
			name: "should list a player with no picks with zero points, not leave them out",
			players: []fixturePlayer{
				{"basti", "Basti", []string{cupFLA}},
				{"tobbi", "Tobbi", nil},
			},
			want: []Row{
				row(1, "basti", "Basti", 20, 0, true),
				row(2, "tobbi", "Tobbi", 0, 0, false),
			},
		},
		{
			name: "should order a tie alphabetically regardless of letter case",
			players: []fixturePlayer{
				{"bea", "bea", []string{cupFLA}},
				{"adam", "Adam", []string{cupFLA}},
			},
			want: []Row{
				row(1, "adam", "Adam", 20, 0, true),
				row(1, "bea", "bea", 20, 0, true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newStandingsStore(t, tt.players)

			got := Rows(st)

			assertRows(t, got, tt.want)
		})
	}
}

func assertRows(t *testing.T, got, want []Row) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d rows, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: expected %+v, got %+v", i+1, want[i], got[i])
		}
	}
}

func TestRowsShouldReturnNoRowsForAStoreWithoutPlayers(t *testing.T) {
	st := newStandingsStore(t, nil)

	got := Rows(st)

	if len(got) != 0 {
		t.Errorf("expected no rows, got %+v", got)
	}
}

func TestRowsShouldTakeEveryPointValueFromScoring(t *testing.T) {
	st := newStandingsStore(t, []fixturePlayer{
		{"basti", "Basti", []string{cupFLA, playoffCupFLA}},
		{"sadl", "Sadl", []string{presidentsTOR}},
		{"tobbi", "Tobbi", []string{cupTOR}},
	})

	for _, r := range Rows(st) {
		if want := scoring.PlayerPoints(st, r.PlayerID); r.Points != want {
			t.Errorf("%s: expected scoring's %+v, got %+v", r.PlayerID, want, r.Points)
		}
	}
}

func TestRowsShouldReflectAPickSavedAfterAnEarlierCallAndNotReuseStaleRows(t *testing.T) {
	st := newStandingsStore(t, []fixturePlayer{
		{"basti", "Basti", []string{cupFLA}},
		{"sadl", "Sadl", nil},
	})
	before := Rows(st)

	if err := st.SavePrediction("sadl", store.KindPlayoffsCup, "FLA", time.Now()); err != nil {
		t.Fatalf("SavePrediction: %v", err)
	}
	after := Rows(st)

	assertRows(t, before, []Row{
		row(1, "basti", "Basti", 20, 0, true),
		row(2, "sadl", "Sadl", 0, 0, false),
	})
	assertRows(t, after, []Row{
		row(1, "basti", "Basti", 20, 0, true),
		row(1, "sadl", "Sadl", 0, 20, true),
	})
}
