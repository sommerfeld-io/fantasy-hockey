package scoring

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// fullSeasonSeed is a data file where basti picked everything correctly and
// every result is recorded: four playoff teams per division (the first one
// wins it), five awards, both trophies, the playoffs Cup and every series of
// every round (8/4/2/1) won in 5.
func fullSeasonSeed() string {
	var teams, picks, marks, matchups, series, finalists, nhl strings.Builder
	pickRow := func(fields string) { picks.WriteString(pick(fields)) }

	for _, division := range store.Divisions() {
		ids := make([]string, 4)
		for i := range ids {
			ids[i] = fmt.Sprintf("%c%02d", division[0], i+1)
			fmt.Fprintf(&teams, "    - {id: %s, name: %s, conference: Eastern, division: %s}\n", ids[i], ids[i], division)
		}
		list := strings.Join(ids, ", ")
		pickRow(fmt.Sprintf("kind: division_playoff_teams, division: %s, team_ids: [%s]", division, list))
		pickRow(fmt.Sprintf("kind: division_winner, division: %s, team_id: %s", division, ids[0]))
		fmt.Fprintf(&marks, "        %s: {playoffs: [%s], division_winner: %s}\n", strings.ToLower(division), list, ids[0])
	}

	for _, award := range []string{store.AwardHart, store.AwardNorris, store.AwardVezina, store.AwardArtRoss, store.AwardRocketRichard} {
		slugs := []string{award + "-a", award + "-b", award + "-c"}
		pickRow(fmt.Sprintf("kind: award, award: %s, finalist_slugs: [%s]", award, strings.Join(slugs, ", ")))
		fmt.Fprintf(&finalists, "    %s:\n", award)
		for _, slug := range slugs {
			fmt.Fprintf(&nhl, "    - {slug: %s, display_name: %s, position: skater}\n", slug, slug)
			fmt.Fprintf(&finalists, "        - {slug: %s, display_name: %s}\n", slug, slug)
		}
	}

	for _, kind := range []string{store.KindCupChampion, store.KindPresidentsTrophy, store.KindPlayoffsCup} {
		pickRow("kind: " + kind + ", team_id: A01")
	}

	rounds := []struct {
		setID, round string
		count        int
	}{{store.Round1SetID, "round1", 8}, {store.Round2SetID, "round2", 4}, {store.ConferenceFinalsSetID, "round3", 2}, {store.StanleyCupFinalSetID, "round4", 1}}
	for _, r := range rounds {
		fmt.Fprintf(&matchups, "    %s:\n", r.setID)
		fmt.Fprintf(&series, "        %s:\n", r.round)
		for i := 1; i <= r.count; i++ {
			key := fmt.Sprintf("s%d", i)
			fmt.Fprintf(&matchups, "        - {key: %s, a: A01, b: M01}\n", key)
			fmt.Fprintf(&series, "            %s: {winner: A01, games: 5}\n", key)
			pickRow(fmt.Sprintf("kind: series, series_key: %s, team_id: A01, games: \"5\"", store.JoinSeriesKey(r.setID, key)))
		}
	}

	return "season: \"2026-27\"\nplayers:\n    - {id: basti, name: Basti, email: basti@example.com}\n" +
		"teams:\n" + teams.String() + "nhl_players:\n" + nhl.String() + "playoff_matchups:\n" + matchups.String() +
		"predictions:\n" + picks.String() +
		"results:\n    team_marks:\n" + marks.String() + "    presidents_trophy: A01\n    stanley_cup_winner: A01\n    series:\n" + series.String() +
		"award_finalists:\n" + finalists.String()
}

func TestPlayerPointsShouldReachTheMaximumForAPerfectSeason(t *testing.T) {
	st, _ := openSeededStore(t, fullSeasonSeed())
	if problems := st.ResultProblems(); len(problems) != 0 {
		t.Fatalf("fixture has result problems: %v", problems)
	}

	// Regular: 4 divisions x (15 + 3x5) + 5 awards x 3x5 + 20 + 20.
	// Playoff: 20 + 8x25 + 4x35 + 2x45 + 50.
	want := Points{Regular: 4*(15+3*5) + 5*3*5 + 20 + 20, Playoff: 20 + 8*25 + 4*35 + 2*45 + 50}

	if got := PlayerPoints(st, "basti"); got != want {
		t.Errorf("PlayerPoints = %+v, want %+v", got, want)
	}
}
