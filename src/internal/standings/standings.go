// Package standings turns internal/scoring's points into the Leaderboard's
// ranked rows: one row per player, sorted by Total. It never derives a
// point value itself, and it caches nothing: every call recomputes from the
// store.
package standings

import (
	"cmp"
	"slices"
	"strings"

	"github.com/sommerfeld-io/fantasy-hockey/internal/scoring"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Row is one player's place in the standings. Rank uses competition
// ranking: equal Totals share a rank and the next rank skips accordingly
// (60, 60, 45 ranks 1, 1, 3). Leader is true for every rank-1 player, but
// only once the top Total is above 0.
type Row struct {
	Rank     int
	PlayerID string
	Name     string
	Points   scoring.Points
	Leader   bool
}

// Rows returns one Row per player in st, sorted by Total descending. Players
// with equal Totals are listed alphabetically by name, ignoring case; that
// order is for display only and never affects a rank.
func Rows(st *store.Store) []Row {
	players := st.Players()
	rows := make([]Row, 0, len(players))
	for _, p := range players {
		rows = append(rows, Row{PlayerID: p.ID, Name: p.Name, Points: scoring.PlayerPoints(st, p.ID)})
	}

	slices.SortStableFunc(rows, compareRows)
	assignRanks(rows)
	return rows
}

// compareRows orders by Total descending, then by name A-Z.
func compareRows(a, b Row) int {
	if c := cmp.Compare(b.Points.Total(), a.Points.Total()); c != 0 {
		return c
	}
	return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

// assignRanks sets competition ranks and the leader flag on rows, which
// must already be sorted by Total descending.
func assignRanks(rows []Row) {
	for i := range rows {
		rows[i].Rank = i + 1
		if i > 0 && rows[i].Points.Total() == rows[i-1].Points.Total() {
			rows[i].Rank = rows[i-1].Rank
		}
		rows[i].Leader = rows[i].Rank == 1 && rows[i].Points.Total() > 0
	}
}
