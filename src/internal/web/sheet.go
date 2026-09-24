package web

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// divisionsSetID is the one Prediction Set id that gets Story 2.4's
// checkbox-chip playoff-teams-and-winners form rather than a single-team
// dropdown - referenced everywhere this file dispatches on it, so the id
// literal can't drift between call sites.
const divisionsSetID = "divisions"

// awardsSetID is the one Prediction Set id that gets Story 2.6's 5-trophy
// finalist-autocomplete form rather than a single-team dropdown or the
// divisions checkbox-chip form - referenced everywhere this file dispatches
// on it, so the id literal can't drift between call sites.
const awardsSetID = "awards"

// pickableSheetKinds is the set of Prediction Set ids that get a real
// pick-entry form instead of the static stub - Story 2.3's single-team
// dropdown for the "cup" and "presidents" ids, Story 3.1's identical
// single-team dropdown for the "playoffcup" id, Story 2.4's checkbox-chip
// form for divisionsSetID, Story 2.6's finalist form for awardsSetID. Every
// other id keeps rendering the stub unchanged.
var pickableSheetKinds = map[string]bool{
	store.KindCupChampion:      true,
	store.KindPresidentsTrophy: true,
	store.KindPlayoffsCup:      true,
	divisionsSetID:             true,
	awardsSetID:                true,
}

// teamDivisionOrder is the fixed division display order for the
// cup/presidents pick sheet's <optgroup> grouping (DESIGN.md's dropdown
// component groups by division).
var teamDivisionOrder = []string{"Atlantic", "Metropolitan", "Central", "Pacific"}

// teamDivisionGroup is one <optgroup> of teamDivisionOrder's dropdown: every
// team sharing one Division, in st.Teams()'s own order.
type teamDivisionGroup struct {
	Division string
	Teams    []store.Team
}

// groupTeamsByDivision groups teams into teamDivisionOrder's fixed order,
// preserving each division's input order. A division with no teams present
// in teams is simply omitted.
func groupTeamsByDivision(teams []store.Team) []teamDivisionGroup {
	byDivision := make(map[string][]store.Team)
	for _, team := range teams {
		byDivision[team.Division] = append(byDivision[team.Division], team)
	}

	var groups []teamDivisionGroup
	for _, division := range teamDivisionOrder {
		if divisionTeams, ok := byDivision[division]; ok {
			groups = append(groups, teamDivisionGroup{Division: division, Teams: divisionTeams})
		}
	}
	return groups
}

// setSubmitted reports whether playerID already has a saved pick for set,
// used both by buildPredictPhases (the Predict list's Submitted status) and
// newSheetData (the sheet's Update-vs-Submit button text). For every id
// other than divisionsSetID/awardsSetID this is Story 2.1's single
// (PlayerID, Kind == set.ID) lookup; divisionsSetID/awardsSetID can't use
// that lookup, since their picks are stored across per-division/per-award
// rows keyed by (PlayerID, Kind, Division)/(PlayerID, Kind, Award) rather
// than one row keyed by Kind == set.ID (AD-28) - submitted there means "has
// saved at least one row for any division/award," matching the reference
// App.jsx's own single "submitted" flag per set rather than one per
// division/award.
func setSubmitted(st *store.Store, set store.PredictionSet, playerID string) bool {
	switch set.ID {
	case divisionsSetID:
		for _, division := range teamDivisionOrder {
			if _, ok := st.FindDivisionPlayoffTeams(playerID, division); ok {
				return true
			}
		}
		return false
	case awardsSetID:
		for _, award := range awardOrder {
			if _, ok := st.FindAwardFinalists(playerID, award); ok {
				return true
			}
		}
		return false
	}

	_, ok := st.FindPrediction(playerID, set.ID)
	return ok
}

// pickView is the cup/presidents sheet's pick-entry state: SelectedTeamID
// pre-fills the dropdown from a saved Prediction (empty for no prior pick);
// Submitted controls the action button's "Update predictions" vs. "Submit
// predictions" text; Error is a non-empty inline caption after a rejected
// submission; Divisions is the season's canonical teams, grouped for the
// <optgroup> markup.
type pickView struct {
	SelectedTeamID string
	Submitted      bool
	Error          string
	Divisions      []teamDivisionGroup
}

// sheetData feeds templates/sheet.html: Title/DeadlineText/Countdown render
// for every Prediction Set id. Closed, Pick, DivisionPick, and AwardsPick
// are only populated for pickableSheetKinds ids (handleSheet/
// handleSheetSubmit) - Pick populates for "cup"/"presidents", DivisionPick
// for divisionsSetID, AwardsPick for awardsSetID; all three stay nil for
// every other id, which keeps rendering the static "not available yet."
// stub (Boundaries & Constraints).
type sheetData struct {
	ID           string
	Title        string
	DeadlineText string
	Countdown    string
	Closed       bool
	Pick         *pickView
	DivisionPick *divisionPickView
	AwardsPick   *awardsPickView
}

// newSheetData builds sheetData for set as seen by playerID at now: the
// common Title/DeadlineText/Countdown/Closed fields every id gets, plus a
// populated Pick for the "cup"/"presidents" ids, a populated DivisionPick
// for divisionsSetID, or a populated AwardsPick for awardsSetID (each built
// from playerID's saved picks). teamIDOverride, when non-nil, replaces the
// saved pick's team id in the rendered cup/presidents form - used by
// renderRejectedPick to re-render the sheet with the rejected submission's
// (invalid) value instead of the last-saved one; it has no effect for
// divisionsSetID/awardsSetID, whose own rejected-resubmission re-renders go
// through renderRejectedDivisionsPick/renderRejectedAwardsPick and
// newDivisionPickView/newAwardsPickView directly instead. err is non-nil
// only when set.DeadlineUTC fails to parse, mirroring newPredictSetView.
func newSheetData(st *store.Store, set store.PredictionSet, playerID string, now time.Time, teamIDOverride *string) (sheetData, error) {
	submitted := setSubmitted(st, set, playerID)

	view, err := newPredictSetView(set, effectiveUpcoming(st, set), submitted, now)
	if err != nil {
		return sheetData{}, err
	}

	data := sheetData{
		ID:           set.ID,
		Title:        view.Title,
		DeadlineText: view.DeadlineText,
		Countdown:    view.Countdown,
		Closed:       view.Status == statusClosed,
	}

	switch {
	case set.ID == divisionsSetID:
		divisionView := newDivisionPickView(st, playerID, submitted, nil, nil)
		data.DivisionPick = &divisionView
	case set.ID == awardsSetID:
		awardsView := newAwardsPickView(st, playerID, submitted, nil)
		data.AwardsPick = &awardsView
	case pickableSheetKinds[set.ID]:
		prediction, _ := st.FindPrediction(playerID, set.ID)
		selectedTeamID := prediction.TeamID
		if teamIDOverride != nil {
			selectedTeamID = *teamIDOverride
		}
		data.Pick = &pickView{
			SelectedTeamID: selectedTeamID,
			Submitted:      submitted,
			Divisions:      groupTeamsByDivision(st.Teams()),
		}
	}

	return data, nil
}

// handleSheet renders the GET /predict/{id} page for the Prediction Set
// matching {id}. For "cup"/"presidents", this is a real pick-entry sheet: a
// single team dropdown (pre-filled from a saved Prediction, if any) grouped
// by division, a Submit/Update action button, and - once the deadline has
// passed - a read-only banner with the select disabled and no action bar
// (FR-8's "no override for anyone"). Every other id keeps rendering the
// static "not available yet." stub. An {id} matching no Prediction Set gets
// a generic http.StatusNotFound response, the same as any other unknown
// path, so it can't be used to probe which ids exist. An Upcoming set gets
// that identical response whatever its kind or deadline, so a set the
// Predict list shows as locked can't be opened by direct URL either. A row
// whose deadline_utc fails to parse is treated the same way, since there's
// nothing sensible to render for it either.
func handleSheet(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		playerID, _ := auth.PlayerIDFromContext(r.Context())

		set, found := findOpenablePredictionSet(st, id)
		if !found {
			http.NotFound(w, r)
			return
		}

		data, err := newSheetData(st, set, playerID, clock.NowTime(), nil)
		if err != nil {
			slog.Error("parse deadline_utc for prediction set", "prediction_set_id", set.ID, "error", err)
			http.NotFound(w, r)
			return
		}

		renderTemplate(w, "sheet.html", data)
	}
}

// findOpenablePredictionSet is findPredictionSetByID with the server-side
// Upcoming gate applied: a set whose effectiveUpcoming is true is reported
// as not found, so both handlers answer it with the same generic 404 as an
// unknown id, before any deadline or form handling. Calling effectiveUpcoming
// - the same helper buildPredictPhases resolves the Predict list's rows with
// - rather than reading set.Upcoming directly means a round-gated id (Story
// 3.2) can't diverge between what the Predict list shows and what a direct
// URL allows.
func findOpenablePredictionSet(st *store.Store, id string) (store.PredictionSet, bool) {
	set, found := findPredictionSetByID(st, id)
	if !found || effectiveUpcoming(st, set) {
		return store.PredictionSet{}, false
	}
	return set, true
}

// findPredictionSetByID returns the Prediction Set matching id, if any -
// the one lookup behind findOpenablePredictionSet, which both handleSheet
// and handleSheetSubmit call, so the lookup can't drift between the two.
func findPredictionSetByID(st *store.Store, id string) (store.PredictionSet, bool) {
	for _, set := range st.PredictionSets() {
		if set.ID == id {
			return set, true
		}
	}
	return store.PredictionSet{}, false
}

// maxSheetFormBytes bounds the POST /predict/{id} body the same way
// maxLoginFormBytes bounds POST /login: a single team id needs only a
// handful of bytes, so this leaves generous headroom while still capping how
// much an unbounded request body can make the server read.
const maxSheetFormBytes = 4096

// invalidTeamErrorText is the inline caption shown when the submitted
// team_id doesn't match any of Store.Teams() (AD-10's server-revalidates
// stance) - an empty, missing, or unknown id all render this identical
// caption, and nothing is saved.
const invalidTeamErrorText = "Pick a team before submitting."

// handleSheetSubmit handles POST /predict/{id}: the cup/presidents pick
// submission (single team_id), the divisionsSetID's division
// playoff-teams-and-winners submission (handleDivisionsSubmit), or the
// awardsSetID's award-finalists submission (handleAwardsSubmit). {id} not
// being a pickableSheetKinds id, or matching no Prediction Set, gets a
// generic http.StatusNotFound - identical to handleSheet's own unknown-id
// handling, so it can't be used to probe which ids exist. An Upcoming set
// gets that same 404 before its deadline or form is even looked at (Upcoming
// wins over Closed, matching predictStatus), so nothing is ever saved for
// it. A closed (past-deadline) set rejects the submission outright with
// http.StatusForbidden, no override for anyone (FR-8), and nothing is
// re-rendered. For cup/presidents, the submitted team_id is re-validated
// against st.Teams() regardless of what the client sent (AD-10); an empty,
// missing, or unknown id re-renders the sheet (200) with an inline error
// caption and the rejected value retained, saving nothing. A valid id is
// saved via st.SavePrediction and redirects to /predict (302).
func handleSheetSubmit(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !pickableSheetKinds[id] {
			http.NotFound(w, r)
			return
		}

		set, found := findOpenablePredictionSet(st, id)
		if !found {
			http.NotFound(w, r)
			return
		}

		now := clock.NowTime()
		deadline, err := time.Parse(time.RFC3339, set.DeadlineUTC)
		if err != nil {
			slog.Error("parse deadline_utc for prediction set", "prediction_set_id", set.ID, "error", err)
			http.NotFound(w, r)
			return
		}
		if !deadline.After(now) {
			http.Error(w, genericErrorBody, http.StatusForbidden)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxSheetFormBytes)
		if err := r.ParseForm(); err != nil {
			slog.Error("parse predict sheet form", "error", err)
			http.Error(w, genericErrorBody, http.StatusInternalServerError)
			return
		}

		playerID, _ := auth.PlayerIDFromContext(r.Context())

		if id == divisionsSetID {
			handleDivisionsSubmit(w, r, st, set, playerID, now)
			return
		}
		if id == awardsSetID {
			handleAwardsSubmit(w, r, st, set, playerID, now)
			return
		}

		teamID := r.FormValue("team_id")
		if !teamRoster(st).has(teamID) {
			renderRejectedPick(w, r, st, set, playerID, now, teamID)
			return
		}

		if err := st.SavePrediction(playerID, id, teamID, now); err != nil {
			slog.Error("save prediction", "player_id", playerID, "kind", id, "error", err)
			http.Error(w, genericErrorBody, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/predict", http.StatusFound)
	}
}

// renderRejectedPick re-renders set's sheet (200) with teamID retained and
// an inline error caption, after handleSheetSubmit rejects it as missing or
// unknown - nothing is saved. err from newSheetData can only come from
// set.DeadlineUTC failing to parse, which handleSheetSubmit's caller already
// parsed successfully moments earlier - kept here (rather than assumed nil)
// so this path stays correct even if that invariant ever changes.
func renderRejectedPick(w http.ResponseWriter, r *http.Request, st *store.Store, set store.PredictionSet, playerID string, now time.Time, teamID string) {
	data, err := newSheetData(st, set, playerID, now, &teamID)
	if err != nil {
		slog.Error("parse deadline_utc for prediction set", "prediction_set_id", set.ID, "error", err)
		http.NotFound(w, r)
		return
	}
	data.Pick.Error = invalidTeamErrorText
	renderTemplate(w, "sheet.html", data)
}
