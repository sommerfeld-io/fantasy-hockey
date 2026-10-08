package web

import (
	"fmt"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Compare row labels and label fragments for the series category
// (click-dummy wording).
const (
	compareSeriesLabel       = "%s · %s vs %s"
	compareSeriesNoConfLabel = "%s vs %s"
	compareSeriesGamesSuffix = "in %s"
)

// seriesCategories is one row per recorded matchup of setID, showing each
// player's winner badge plus a separate " in <games>" badge, each matching
// independently. teamName/conferenceForMatchup/stanleyCupFinalLabel are defined
// in sheet_series.go, shared with Predict's own series sheet - not
// duplicated here.
func seriesCategories(st *store.Store, setID string) []compareCategory {
	matchups := st.PlayoffMatchups(setID)
	categories := make([]compareCategory, 0, len(matchups))
	for _, m := range matchups {
		seriesKey := store.JoinSeriesKey(setID, m.Key)
		categories = append(categories, compareCategory{
			label: seriesLabel(st, setID, m),
			values: func(playerID string) []compareValueView {
				p, ok := st.FindSeriesPick(playerID, seriesKey)
				if !ok || p.TeamID == "" {
					return nil
				}
				return []compareValueView{tagValue(p.TeamID), tagValue(fmt.Sprintf(compareSeriesGamesSuffix, p.Games))}
			},
		})
	}
	return categories
}

// seriesLabel is "<Conference> · A vs B", with the Final's own label in
// place of a conference and no prefix when the conference is unknown.
func seriesLabel(st *store.Store, setID string, m store.PlayoffMatchup) string {
	conference := stanleyCupFinalLabel
	if setID != store.StanleyCupFinalSetID {
		conference = conferenceForMatchup(st, m)
	}
	if conference == "" {
		return fmt.Sprintf(compareSeriesNoConfLabel, m.TeamA, m.TeamB)
	}
	return fmt.Sprintf(compareSeriesLabel, conference, m.TeamA, m.TeamB)
}
