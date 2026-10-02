package web

import (
	"github.com/sommerfeld-io/fantasy-hockey/internal/standings"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Leaderboard CSS classes (styles.css). Gold is reserved for a leader's
// rank badge and Total (DESIGN.md), so only those two carry a modifier.
const (
	rankBadgeCSS       = "rank-badge"
	rankBadgeLeaderCSS = "rank-badge rank-badge--gold"
	totalCSS           = "lb-total"
	totalLeaderCSS     = "lb-total lb-total--gold"
)

// leaderboardRowView is one presentation-ready Leaderboard row: its points
// come straight from standings.Row and its CSS classes are precomputed, so
// shell.html computes no points, ranks or leader state itself.
type leaderboardRowView struct {
	PlayerID string
	Name     string
	Rank     int
	Regular  int
	Playoff  int
	Total    int
	BadgeCSS string
	TotalCSS string
}

// leaderboardView is the Leaderboard tab's content.
type leaderboardView struct {
	Rows []leaderboardRowView
}

// buildLeaderboard renders standings.Rows(st) into view rows. It is called
// on every request, so nothing is ever cached.
func buildLeaderboard(st *store.Store) leaderboardView {
	rows := standings.Rows(st)
	views := make([]leaderboardRowView, 0, len(rows))
	for _, r := range rows {
		views = append(views, newLeaderboardRowView(r))
	}
	return leaderboardView{Rows: views}
}

func newLeaderboardRowView(r standings.Row) leaderboardRowView {
	v := leaderboardRowView{
		PlayerID: r.PlayerID,
		Name:     r.Name,
		Rank:     r.Rank,
		Regular:  r.Points.Regular,
		Playoff:  r.Points.Playoff,
		Total:    r.Points.Total(),
		BadgeCSS: rankBadgeCSS,
		TotalCSS: totalCSS,
	}
	if r.Leader {
		v.BadgeCSS, v.TotalCSS = rankBadgeLeaderCSS, totalLeaderCSS
	}
	return v
}
