package web

import (
	"slices"
	"testing"
)

func TestBuildCompareShouldTagTheSeriesWinnerAndKeepTheGamesCountPlain(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: series, series_key: r1.s1, team_id: FLA, games: \"5\""),
	)

	v := buildCompare(st, "basti", "r1", compareNow)

	got := cellValueViews(t, v.Table, "Eastern · FLA vs TOR")[0]
	want := []string{compareValueTagCSS, compareValueCSS}
	if !slices.Equal(valueCSS(got), want) {
		t.Errorf("expected the winner tagged and games plain (%v), got %v", want, valueCSS(got))
	}
	if len(got) != 2 || got[0].Text != "FLA" || got[1].Text != "in 5" {
		t.Errorf("expected [FLA, \"in 5\"], got %+v", got)
	}
}

func TestBuildCompareShouldShowSeriesPicksAsWinnerAndGames(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: series, series_key: r1.s1, team_id: FLA, games: \"5\""),
		comparePick("tobbi", "kind: series, series_key: r1.s2, team_id: VGK, games: \"7\""),
		comparePick("basti", "kind: series, series_key: scf.s1, team_id: COL, games: \"4\""),
	)

	v := buildCompare(st, "basti", "r1", compareNow)

	if got, want := cellValues(t, v.Table, "Eastern · FLA vs TOR"), [][]string{{"FLA", "in 5"}, {emptyCellValue}, {emptyCellValue}}; !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Errorf("expected %v, got %v", want, got)
	}
	if got, want := cellValues(t, v.Table, "Western · COL vs VGK"), [][]string{{emptyCellValue}, {emptyCellValue}, {"VGK", "in 7"}}; !slices.EqualFunc(got, want, slices.Equal[[]string]) {
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
