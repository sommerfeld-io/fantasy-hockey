package web

import (
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// awardCategories is one stacked row per award, in awardOrder, with each
// finalist's display name, plain. awardOrder/awardTitle/displayNameForSlug
// are defined in sheet_awards.go, shared with Predict's own award sheet -
// not duplicated here.
func awardCategories(st *store.Store) []compareCategory {
	categories := make([]compareCategory, 0, len(awardOrder))
	for _, award := range awardOrder {
		categories = append(categories, compareCategory{
			label:   awardTitle[award],
			stacked: true,
			values: func(playerID string) []compareValueView {
				p, ok := st.FindAwardFinalists(playerID, award)
				if !ok {
					return nil
				}
				values := make([]compareValueView, 0, len(p.FinalistSlugs))
				for _, slug := range p.FinalistSlugs {
					values = append(values, plainValue(displayNameForSlug(st, slug)))
				}
				return values
			},
		})
	}
	return categories
}
