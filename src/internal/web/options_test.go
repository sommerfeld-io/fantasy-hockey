package web

import (
	"encoding/json"
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// TestNewTeamOptionsShouldConvertEveryTeamIntoTheSharedEmbedShape covers
// AD-19: every entry gets exactly {"id": "<abbr>", "label": "<name>"},
// preserving the input order, so 2.3/2.4's embedding sites can rely on it.
func TestNewTeamOptionsShouldConvertEveryTeamIntoTheSharedEmbedShape(t *testing.T) {
	teams := []store.Team{
		{ID: "TOR", Name: "Toronto Maple Leafs", Conference: "Eastern", Division: "Atlantic"},
		{ID: "VGK", Name: "Vegas Golden Knights", Conference: "Western", Division: "Pacific"},
	}

	options := newTeamOptions(teams)

	out, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `[{"id":"TOR","label":"Toronto Maple Leafs"},{"id":"VGK","label":"Vegas Golden Knights"}]`
	if string(out) != want {
		t.Errorf("expected JSON %q, got %q", want, string(out))
	}
}

func TestNewTeamOptionsShouldReturnAnEmptySliceForAnEmptyInput(t *testing.T) {
	options := newTeamOptions(nil)

	if len(options) != 0 {
		t.Errorf("expected an empty slice, got %v", options)
	}
}

// TestNewAwardFinalistOptionsShouldConvertEveryFinalistIntoTheSharedEmbedShape
// covers AD-19: every entry gets exactly {"id": "<slug>", "label":
// "<display_name>"}, preserving the input order and producing the identical
// shape newTeamOptions does, so 2.6's embedding site can rely on it.
func TestNewAwardFinalistOptionsShouldConvertEveryFinalistIntoTheSharedEmbedShape(t *testing.T) {
	finalists := []store.AwardFinalist{
		{Slug: "mcdavid-connor", DisplayName: "Connor McDavid", Position: store.PositionSkater},
		{Slug: "hellebuyck-connor", DisplayName: "Connor Hellebuyck", Position: store.PositionGoalie},
	}

	options := newAwardFinalistOptions(finalists)

	out, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `[{"id":"mcdavid-connor","label":"Connor McDavid"},{"id":"hellebuyck-connor","label":"Connor Hellebuyck"}]`
	if string(out) != want {
		t.Errorf("expected JSON %q, got %q", want, string(out))
	}
}

func TestNewAwardFinalistOptionsShouldReturnAnEmptySliceForAnEmptyInput(t *testing.T) {
	options := newAwardFinalistOptions(nil)

	if len(options) != 0 {
		t.Errorf("expected an empty slice, got %v", options)
	}
}
