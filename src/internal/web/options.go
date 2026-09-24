package web

import (
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// embedOption is one entry in the shared autocomplete embed shape (AD-19):
// every embedding site (Team via newTeamOptions, AwardFinalist via
// newAwardFinalistOptions, and any future data) renders this identical
// {"id", "label"} JSON object, and the widget always submits ID, never
// Label, into its bound form field.
type embedOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// newTeamOptions converts teams into the shared embed shape, preserving
// input order. Not called from any handler yet - Story 2.3's cup/presidents
// sheet renders native <option>s from Store.Teams() directly instead
// (Boundaries & Constraints: no JSON-embed shape for this story); 2.4/2.6
// embed it once they add their own dropdowns/chips.
func newTeamOptions(teams []store.Team) []embedOption {
	options := make([]embedOption, len(teams))
	for i, team := range teams {
		options[i] = embedOption{ID: team.ID, Label: team.Name}
	}
	return options
}

// newAwardFinalistOptions converts NHL Players (AwardFinalist) into the
// shared embed shape, preserving input order. Not called from any handler
// yet - reserved for Story 2.6's award-finalist autocomplete to embed once
// it adds its own dropdown/chips, exactly like newTeamOptions itself has sat
// unused since Story 2.2.
func newAwardFinalistOptions(finalists []store.AwardFinalist) []embedOption {
	options := make([]embedOption, len(finalists))
	for i, finalist := range finalists {
		options[i] = embedOption{ID: finalist.Slug, Label: finalist.DisplayName}
	}
	return options
}
