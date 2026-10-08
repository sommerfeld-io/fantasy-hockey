package web

import (
	"slices"
	"testing"
)

func TestBuildCompareShouldBadgeAwardFinalistsKeyedBySlug(t *testing.T) {
	st := newCompareStore(t, compareDefaultSets, compareDefaultMatchups,
		comparePick("basti", "kind: award, award: hart, finalist_slugs: [mcdavid-connor, mackinnon-nathan]"),
	)

	v := buildCompare(st, "basti", "awards", compareNow)

	got := cellValueViews(t, v.Table, "Hart Trophy finalists")[1]
	want := []string{compareValueTagCSS, compareValueTagCSS}
	if !slices.Equal(valueCSS(got), want) {
		t.Errorf("expected finalists tagged (%v), got %v", want, valueCSS(got))
	}
	if got[0].Match != "mcdavid-connor" || got[1].Match != "mackinnon-nathan" {
		t.Errorf("expected each finalist keyed by its slug, got %+v", got)
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
