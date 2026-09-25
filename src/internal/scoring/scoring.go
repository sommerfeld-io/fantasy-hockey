// Package scoring computes each player's Regular and Playoff points from
// their saved picks and the hand-recorded results in internal/store,
// applying the season's point table. It is the only home of point
// calculation: every call recomputes from the store, nothing is cached and
// nothing is ever written back.
package scoring

import (
	"slices"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Season-1 point table (scoring-rules.md). Recalibrating a season means
// editing only this block and seriesPoints below.
const (
	awardFinalistPoints    = 5
	playoffTeamPoints      = 5
	divisionWinnerPoints   = 15
	presidentsTrophyPoints = 20
	cupChampionPoints      = 20
	playoffsCupPoints      = 20
)

// seriesValue is one round's points for a correct winner and for an exact
// result (winner and game count). They replace each other; never add up.
type seriesValue struct {
	winner, exact int
}

// seriesPoints is the per-round series part of the point table, keyed by
// the round's Prediction Set id.
var seriesPoints = map[string]seriesValue{
	store.Round1SetID:           {winner: 15, exact: 25},
	store.Round2SetID:           {winner: 25, exact: 35},
	store.ConferenceFinalsSetID: {winner: 30, exact: 45},
	store.StanleyCupFinalSetID:  {winner: 30, exact: 50},
}

// Points is one player's score, split into the before-season (Regular) and
// playoff (Playoff) buckets.
type Points struct {
	Regular int
	Playoff int
}

// Total is Regular + Playoff, the value a player is ranked by.
func (p Points) Total() int {
	return p.Regular + p.Playoff
}

// PlayerPoints computes playerID's points from st. A player not in the
// players list, an empty pick, a result not recorded yet or a malformed
// result all simply score 0; scoring never fails.
func PlayerPoints(st *store.Store, playerID string) Points {
	if !slices.ContainsFunc(st.Players(), func(p store.Player) bool { return p.ID == playerID }) {
		return Points{}
	}

	predictions := st.PredictionsForPlayer(playerID)
	points := Points{Regular: divisionsPoints(st, predictions)}
	for _, p := range predictions {
		regular, playoff := predictionPoints(st, p)
		points.Regular += regular
		points.Playoff += playoff
	}
	return points
}

// predictionPoints scores one row that stands on its own. Division rows are
// scored per division by divisionsPoints instead, since a division's
// winner and playoff-team rows depend on each other.
func predictionPoints(st *store.Store, p store.Prediction) (regular, playoff int) {
	switch p.Kind {
	case store.KindAward:
		return awardPoints(p.FinalistSlugs, st.RecordedAwardFinalists(p.Award)), 0
	case store.KindCupChampion:
		return trophyPoints(p.TeamID, st.StanleyCupWinner(), cupChampionPoints), 0
	case store.KindPresidentsTrophy:
		return trophyPoints(p.TeamID, st.PresidentsTrophyWinner(), presidentsTrophyPoints), 0
	case store.KindPlayoffsCup:
		return 0, trophyPoints(p.TeamID, st.StanleyCupWinner(), playoffsCupPoints)
	case store.KindSeries:
		return 0, seriesPickPoints(st, p)
	}
	return 0, 0
}

// awardPoints gives awardFinalistPoints for each distinct pick found
// anywhere in finalists, which a tie can grow past three names.
func awardPoints(picks, finalists []string) int {
	points := 0
	for _, slug := range distinct(picks) {
		if slices.Contains(finalists, slug) {
			points += awardFinalistPoints
		}
	}
	return points
}

// trophyPoints gives value when pick matches the recorded winner.
func trophyPoints(pick, winner string, value int) int {
	if pick == "" || pick != winner {
		return 0
	}
	return value
}

// seriesPickPoints gives the round's exact value when winner and game count
// both match, else its winner value when only the winner matches.
func seriesPickPoints(st *store.Store, p store.Prediction) int {
	setID, _, ok := store.SplitSeriesKey(p.SeriesKey)
	if !ok {
		return 0
	}
	value, ok := seriesPoints[setID]
	if !ok {
		return 0
	}
	winner, games, ok := st.SeriesResult(p.SeriesKey)
	if !ok || p.TeamID != winner {
		return 0
	}
	if p.Games == games {
		return value.exact
	}
	return value.winner
}

// divisionsPoints scores every division's winner pick and playoff-team list.
func divisionsPoints(st *store.Store, predictions []store.Prediction) int {
	points := 0
	for _, division := range store.Divisions() {
		playoffs, winner := st.DivisionResult(division)
		picks := divisionPicks{
			teams:  divisionRow(predictions, store.KindDivisionPlayoffTeams, division).TeamIDs,
			winner: divisionRow(predictions, store.KindDivisionWinner, division).TeamID,
		}
		points += picks.points(playoffs, winner)
	}
	return points
}

// divisionPicks is one player's two picks for one division.
type divisionPicks struct {
	teams  []string
	winner string
}

// points gives divisionWinnerPoints for a correct winner pick and
// playoffTeamPoints for every other listed team that made the playoffs. The
// correctly picked winner never also earns playoffTeamPoints: the 15
// replaces that mark. A recorded division winner always made the playoffs,
// even if the hand-maintained playoffs list leaves it out.
func (d divisionPicks) points(playoffs []string, winner string) int {
	points := 0
	correctWinner := winner != "" && d.winner == winner
	if correctWinner {
		points += divisionWinnerPoints
	}
	for _, team := range distinct(d.teams) {
		if correctWinner && team == winner {
			continue
		}
		if slices.Contains(playoffs, team) || (winner != "" && team == winner) {
			points += playoffTeamPoints
		}
	}
	return points
}

// divisionRow returns the row of kind for division, or a zero row.
func divisionRow(predictions []store.Prediction, kind, division string) store.Prediction {
	for _, p := range predictions {
		if p.Kind == kind && p.Division == division {
			return p
		}
	}
	return store.Prediction{}
}

// distinct drops repeated values so a hand-edited duplicate never scores
// twice.
func distinct(values []string) []string {
	var out []string
	for _, v := range values {
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
