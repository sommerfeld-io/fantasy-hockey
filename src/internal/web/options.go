package web

import (
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// embedOption is one entry in the shared autocomplete embed shape (AD-19):
// every embedding site (today only AwardFinalist via
// newAwardFinalistOptions; newTeamOptions has no production caller) renders
// this identical {"id", "label"} JSON object, and the widget always submits
// ID, never Label, into its bound form field.
type embedOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// newTeamOptions converts teams into the shared embed shape, preserving
// input order. It has no production caller: every team picker (the
// cup/presidents sheet, the division chips and the series roster) renders
// from Store.Teams() directly, so the 2.4/2.6 embed it was kept for never
// happened. Only its tests call it.
func newTeamOptions(teams []store.Team) []embedOption {
	options := make([]embedOption, len(teams))
	for i, team := range teams {
		options[i] = embedOption{ID: team.ID, Label: team.Name}
	}
	return options
}

// newAwardFinalistOptions converts NHL Players (AwardFinalist) into the
// shared embed shape, preserving input order. The awards sheet embeds it
// as each award's autocomplete options (marshalAwardOptions).
func newAwardFinalistOptions(finalists []store.AwardFinalist) []embedOption {
	options := make([]embedOption, len(finalists))
	for i, finalist := range finalists {
		options[i] = embedOption{ID: finalist.Slug, Label: finalist.DisplayName}
	}
	return options
}
