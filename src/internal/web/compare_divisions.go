package web

import (
	"fmt"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Compare row labels for the divisions category (click-dummy wording).
const (
	compareDivisionPlayoffLabel = "%s — playoff teams"
	compareDivisionWinnerLabel  = "%s — winner"
)

// divisionCategories is a playoff-teams row then a winner row per division,
// in store.Divisions() order, with team abbreviations tagged.
func divisionCategories(st *store.Store) []compareCategory {
	var categories []compareCategory
	for _, division := range store.Divisions() {
		categories = append(categories,
			compareCategory{
				label: fmt.Sprintf(compareDivisionPlayoffLabel, division),
				values: func(playerID string) []compareValueView {
					p, ok := st.FindDivisionPlayoffTeams(playerID, division)
					if !ok || len(p.TeamIDs) == 0 {
						return nil
					}
					return tagValues(p.TeamIDs)
				},
			},
			compareCategory{
				label: fmt.Sprintf(compareDivisionWinnerLabel, division),
				values: func(playerID string) []compareValueView {
					p, ok := st.FindDivisionWinner(playerID, division)
					if !ok || p.TeamID == "" {
						return nil
					}
					return []compareValueView{tagValue(p.TeamID)}
				},
			},
		)
	}
	return categories
}
