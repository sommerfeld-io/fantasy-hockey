package web

import (
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// rosterTestTeams is a small mixed-division team fixture for the roster
// tests, including one entry with an empty ID to prove an empty id never
// becomes a member.
var rosterTestTeams = []store.Team{
	{ID: "TOR", Name: "Toronto Maple Leafs", Conference: "Eastern", Division: "Atlantic"},
	{ID: "BOS", Name: "Boston Bruins", Conference: "Eastern", Division: "Atlantic"},
	{ID: "EDM", Name: "Edmonton Oilers", Conference: "Western", Division: "Pacific"},
	{ID: "", Name: "Nameless", Conference: "Western", Division: "Pacific"},
}

func TestRosterShouldAnswerMembershipAndLookupForEveryCase(t *testing.T) {
	atlanticOnly := func(team store.Team) bool { return team.Division == "Atlantic" }

	tests := []struct {
		name     string
		keep     func(store.Team) bool
		id       string
		wantHas  bool
		wantName string
	}{
		{name: "has and gets a member", keep: nil, id: "TOR", wantHas: true, wantName: "Toronto Maple Leafs"},
		{name: "has and gets a member from another division without a filter", keep: nil, id: "EDM", wantHas: true, wantName: "Edmonton Oilers"},
		{name: "does not have a missing id", keep: nil, id: "XYZ", wantHas: false},
		{name: "does not have an empty id", keep: nil, id: "", wantHas: false},
		{name: "keeps an item the filter accepts", keep: atlanticOnly, id: "BOS", wantHas: true, wantName: "Boston Bruins"},
		{name: "drops an item the filter rejects", keep: atlanticOnly, id: "EDM", wantHas: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRoster(rosterTestTeams, teamKey, tt.keep)

			has := r.has(tt.id)
			team, ok := r.get(tt.id)

			if has != tt.wantHas {
				t.Errorf("has(%q) = %v, want %v", tt.id, has, tt.wantHas)
			}
			if ok != tt.wantHas {
				t.Errorf("get(%q) ok = %v, want %v", tt.id, ok, tt.wantHas)
			}
			if team.Name != tt.wantName {
				t.Errorf("get(%q) name = %q, want %q", tt.id, team.Name, tt.wantName)
			}
		})
	}
}

func TestRosterShouldKeepTheFirstItemForADuplicateID(t *testing.T) {
	teams := []store.Team{
		{ID: "TOR", Name: "Toronto Maple Leafs"},
		{ID: "TOR", Name: "Duplicate Toronto"},
	}

	team, ok := newRoster(teams, teamKey, nil).get("TOR")

	if !ok {
		t.Fatal("expected TOR to be a member")
	}
	if team.Name != "Toronto Maple Leafs" {
		t.Errorf("expected the first-seen item to win, got %q", team.Name)
	}
}

func TestRosterShouldHaveNoMembersForAnEmptyInput(t *testing.T) {
	r := newRoster(nil, teamKey, nil)

	if r.has("TOR") {
		t.Errorf("expected an empty roster to have no members")
	}
	if _, ok := r.get("TOR"); ok {
		t.Errorf("expected get on an empty roster to report not found")
	}
}

func TestStoreRostersShouldReflectTheStoresCanonicalLists(t *testing.T) {
	st := newTestStoreWithAwardsRoster(t, "")
	teams := newTestStoreWithPredictionSetsAndTeams(t, "")

	if !teamRoster(teams).has("TOR") {
		t.Errorf("expected teamRoster to contain TOR")
	}
	if teamRoster(teams).has("XYZ") {
		t.Errorf("expected teamRoster not to contain an unknown id")
	}
	if !divisionTeamRoster(teams, "Atlantic").has("TOR") {
		t.Errorf("expected the Atlantic roster to contain TOR")
	}
	if divisionTeamRoster(teams, "Pacific").has("TOR") {
		t.Errorf("expected the Pacific roster not to contain an Atlantic team")
	}
	if _, ok := playerRoster(st).get("mcdavid-connor"); !ok {
		t.Errorf("expected playerRoster to contain mcdavid-connor")
	}
	if playerRoster(st).has("") {
		t.Errorf("expected playerRoster never to contain an empty slug")
	}
}
