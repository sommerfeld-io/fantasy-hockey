package web

import (
	"slices"
	"testing"
)

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

func TestBuildCompareShouldTagDivisionPlayoffTeamsAndWinner(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: division_playoff_teams, division: Atlantic, team_ids: [FLA, TOR]"),
		comparePick("sadl", "kind: division_winner, division: Atlantic, team_id: FLA"),
	)

	v := buildCompare(st, "basti", "divisions", compareNow)

	teams := cellValueViews(t, v.Table, "Atlantic — playoff teams")[0]
	if want := []string{compareValueTagCSS, compareValueTagCSS}; !slices.Equal(valueCSS(teams), want) {
		t.Errorf("expected both playoff teams tagged (%v), got %v", want, valueCSS(teams))
	}
	winner := cellValueViews(t, v.Table, "Atlantic — winner")[0]
	if want := []string{compareValueTagCSS}; !slices.Equal(valueCSS(winner), want) {
		t.Errorf("expected the division winner tagged (%v), got %v", want, valueCSS(winner))
	}
}

// TestBuildCompareShouldTagTheOwnColumnsValueAndKeepOwnStyling closes an
// epic-5 retrospective gap (item 40): every existing division tag test above
// reads a non-signed-in player's pick into a non-own column - nothing proved
// the signed-in player's own column/cell still renders a tagged value
// correctly alongside its own own-styling.
func TestBuildCompareShouldTagTheOwnColumnsValueAndKeepOwnStyling(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("basti", "kind: division_playoff_teams, division: Atlantic, team_ids: [FLA, TOR]"),
	)

	v := buildCompare(st, "basti", "divisions", compareNow)

	if !v.Table.Columns[1].Own || v.Table.Columns[1].Name != "Basti" {
		t.Fatalf("expected column 1 to be Basti's own column, got %+v", v.Table.Columns[1])
	}
	if v.Table.Columns[1].CSS != comparePlayerOwnCSS {
		t.Errorf("expected the own column's CSS to be %q, got %q", comparePlayerOwnCSS, v.Table.Columns[1].CSS)
	}

	var ownCell *compareCellView
	for _, r := range v.Table.Rows {
		if r.Label == "Atlantic — playoff teams" {
			ownCell = &r.Cells[1]
			break
		}
	}
	if ownCell == nil {
		t.Fatal("no \"Atlantic — playoff teams\" row found")
	}
	if !ownCell.Own {
		t.Error("expected the signed-in player's own cell to be marked own")
	}
	if ownCell.CSS != compareCellOwnCSS {
		t.Errorf("expected the own cell's CSS to be %q, got %q", compareCellOwnCSS, ownCell.CSS)
	}
	if want := []string{compareValueTagCSS, compareValueTagCSS}; !slices.Equal(valueCSS(ownCell.Values), want) {
		t.Errorf("expected the signed-in player's own playoff-teams pick tagged (%v), got %v", want, valueCSS(ownCell.Values))
	}
}

func TestBuildCompareShouldShowAnEmptyDashForASavedButEmptyDivisionPlayoffTeamsPick(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("sadl", "kind: division_playoff_teams, division: Atlantic, team_ids: []"),
	)

	v := buildCompare(st, "basti", "divisions", compareNow)

	got := cellValueViews(t, v.Table, "Atlantic — playoff teams")[0]
	if len(got) != 1 || got[0].Text != emptyCellValue || got[0].CSS != compareValueEmptyCSS || got[0].Match != "" {
		t.Errorf("expected a faint empty value for a saved-but-empty pick, got %+v", got)
	}
}
