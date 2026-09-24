package web

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// divisionConferenceGroup is one of the two Conference sections
// ("Eastern"/"Western") the divisions sheet renders, each holding its own
// two teamDivisionGroups.
type divisionConferenceGroup struct {
	Conference string
	Divisions  []teamDivisionGroup
}

// groupDivisionsByConference splits groups (teamDivisionOrder's fixed
// division order) into their two Conferences, preserving teamDivisionOrder's
// own order within each conference. Each conference id is derived from its
// first group's first team's own Conference field (Never: "never a second
// hardcoded division->conference map"). groups must come from
// groupTeamsByDivision, whose own contract guarantees every group has at
// least one team - never a group with an empty Teams slice.
func groupDivisionsByConference(groups []teamDivisionGroup) []divisionConferenceGroup {
	var conferences []divisionConferenceGroup
	indexByConference := make(map[string]int)

	for _, group := range groups {
		conference := group.Teams[0].Conference

		i, ok := indexByConference[conference]
		if !ok {
			i = len(conferences)
			indexByConference[conference] = i
			conferences = append(conferences, divisionConferenceGroup{Conference: conference})
		}
		conferences[i].Divisions = append(conferences[i].Divisions, group)
	}
	return conferences
}

// maxTeamsPerDivision is the most playoff teams a player may check within
// one division (DESIGN.md's Division-picks progress indicator: "a small n/5
// counter per division").
const maxTeamsPerDivision = 5

// requiredTeamsPerConference is the exact total of playoff teams a
// conference's two divisions must sum to (either a 4/4 split or a 5/3 split)
// before that conference counts as valid.
const requiredTeamsPerConference = 8

// divisionPlayoffTeamsFieldName is division's checkbox-group form field
// name for its playoff-teams pick.
func divisionPlayoffTeamsFieldName(division string) string {
	return "playoff_teams_" + strings.ToLower(division)
}

// divisionWinnerFieldName is division's <select> form field name for its
// winner pick.
func divisionWinnerFieldName(division string) string {
	return "winner_" + strings.ToLower(division)
}

// divisionTeamPick is one team chip in a division's playoff-teams checkbox
// list: Checked reflects the player's currently-saved (or a rejected
// resubmission's own retained) pick.
type divisionTeamPick struct {
	ID      string
	Name    string
	Checked bool
}

// divisionGroupPick is one division's rendered chip group and winner
// select: Teams are the division's full roster as checkbox chips;
// WinnerTeams is the same roster again for the winner <select> - never
// filtered to the checked playoff-team picks (Design Notes: matches the
// reference App.jsx's own unfiltered winner Select). PlayoffTeamsField/
// WinnerField are this division's two submitted form field names.
// SelectedCount is the division's live n/5 counter.
type divisionGroupPick struct {
	Division          string
	PlayoffTeamsField string
	WinnerField       string
	Teams             []divisionTeamPick
	SelectedCount     int
	Winner            string
	WinnerTeams       []store.Team
}

// divisionConferencePick is one conference's rendered section: two division
// groups, the live n/8 Total across both, and whether Total is exactly
// requiredTeamsPerConference.
type divisionConferencePick struct {
	Conference string
	Divisions  []divisionGroupPick
	Total      int
	Valid      bool
}

// divisionPickView is the "divisions" sheet's whole pick-entry state:
// Conferences holds both rendered sections; Submitted/Error mirror pickView's
// own fields. AllValid is the AND of every conference's Valid (vacuously
// true when Conferences is empty - a hand-edit misconfiguration with no
// teams at all, matching this codebase's own established precedent for
// rejecting hand-edit-reachable-only findings), gating the submit button's
// disabled attribute both on initial server render and live via
// divisions.js's updateSubmitState.
type divisionPickView struct {
	Conferences []divisionConferencePick
	Submitted   bool
	AllValid    bool
	Error       string
}

// selectedDivisionTeams returns the subset of selected that belongs to
// group's own roster, as a set for O(1) Checked lookups (and to dedup any
// repeated id). Filtering to the division's own roster first - rather than
// trusting selected's raw length - means a rejected "wrong division"
// resubmission's re-rendered SelectedCount/its conference's Total can't be
// inflated by a team id submitted under a different division's field.
func selectedDivisionTeams(group teamDivisionGroup, selected []string) map[string]bool {
	members := newRoster(group.Teams, teamKey, nil)

	set := make(map[string]bool)
	for _, id := range selected {
		if members.has(id) {
			set[id] = true
		}
	}
	return set
}

// newDivisionPickView builds sheetData's DivisionPick field for playerID:
// every division group from groupTeamsByDivision(st.Teams()), each chip's
// Checked state and each division's Winner reflecting either playerID's
// saved picks, or - when playoffTeamsOverride/winnersOverride are non-nil -
// a rejected resubmission's own (invalid) values retained for re-rendering
// (renderRejectedDivisionsPick).
func newDivisionPickView(st *store.Store, playerID string, submitted bool, playoffTeamsOverride map[string][]string, winnersOverride map[string]string) divisionPickView {
	groups := groupTeamsByDivision(st.Teams())

	picksByDivision := make(map[string]divisionGroupPick, len(groups))
	for _, group := range groups {
		picksByDivision[group.Division] = newDivisionGroupPick(st, playerID, group, playoffTeamsOverride, winnersOverride)
	}

	conferences := buildDivisionConferencePicks(groupDivisionsByConference(groups), picksByDivision)

	return divisionPickView{Conferences: conferences, Submitted: submitted, AllValid: allConferencesValid(conferences)}
}

// selectedTeamsForDivision resolves group's currently-selected playoff-team
// ids: override's own entry when override is non-nil (a rejected
// resubmission's retained value), otherwise playerID's saved pick.
func selectedTeamsForDivision(st *store.Store, playerID string, group teamDivisionGroup, override map[string][]string) []string {
	if override != nil {
		return override[group.Division]
	}
	if prediction, ok := st.FindDivisionPlayoffTeams(playerID, group.Division); ok {
		return prediction.TeamIDs
	}
	return nil
}

// winnerForDivision resolves division's currently-selected winner the same
// way selectedTeamsForDivision resolves playoff teams.
func winnerForDivision(st *store.Store, playerID, division string, override map[string]string) string {
	if override != nil {
		return override[division]
	}
	if prediction, ok := st.FindDivisionWinner(playerID, division); ok {
		return prediction.TeamID
	}
	return ""
}

// newDivisionGroupPick builds one division's rendered chip group and winner
// select - factored out of newDivisionPickView purely to keep its own
// cyclomatic complexity low (gocyclo).
func newDivisionGroupPick(st *store.Store, playerID string, group teamDivisionGroup, playoffTeamsOverride map[string][]string, winnersOverride map[string]string) divisionGroupPick {
	selectedSet := selectedDivisionTeams(group, selectedTeamsForDivision(st, playerID, group, playoffTeamsOverride))

	teams := make([]divisionTeamPick, len(group.Teams))
	for i, team := range group.Teams {
		teams[i] = divisionTeamPick{ID: team.ID, Name: team.Name, Checked: selectedSet[team.ID]}
	}

	return divisionGroupPick{
		Division:          group.Division,
		PlayoffTeamsField: divisionPlayoffTeamsFieldName(group.Division),
		WinnerField:       divisionWinnerFieldName(group.Division),
		Teams:             teams,
		SelectedCount:     len(selectedSet),
		Winner:            winnerForDivision(st, playerID, group.Division, winnersOverride),
		WinnerTeams:       group.Teams,
	}
}

// buildDivisionConferencePicks assembles each conference's rendered section
// (its two division groups plus their live n/8 Total/Valid) from
// picksByDivision - factored out of newDivisionPickView purely to keep its
// own cyclomatic complexity low (gocyclo).
func buildDivisionConferencePicks(confGroups []divisionConferenceGroup, picksByDivision map[string]divisionGroupPick) []divisionConferencePick {
	var conferences []divisionConferencePick
	for _, confGroup := range confGroups {
		var divisions []divisionGroupPick
		total := 0
		for _, group := range confGroup.Divisions {
			pick := picksByDivision[group.Division]
			divisions = append(divisions, pick)
			total += pick.SelectedCount
		}
		conferences = append(conferences, divisionConferencePick{
			Conference: confGroup.Conference,
			Divisions:  divisions,
			Total:      total,
			Valid:      total == requiredTeamsPerConference,
		})
	}
	return conferences
}

// allConferencesValid is the AND of every conference's Valid - vacuously
// true when conferences is empty (see divisionPickView.AllValid's own
// doc comment for why that default is accepted rather than fixed).
func allConferencesValid(conferences []divisionConferencePick) bool {
	for _, conference := range conferences {
		if !conference.Valid {
			return false
		}
	}
	return true
}

// parseDivisionsSubmission reads every division's checkbox-group and
// winner-select values off the submitted form, keyed by division name, for
// every entry in teamDivisionOrder. r.Form must already be populated
// (r.ParseForm).
func parseDivisionsSubmission(r *http.Request) (playoffTeams map[string][]string, winners map[string]string) {
	playoffTeams = make(map[string][]string, len(teamDivisionOrder))
	winners = make(map[string]string, len(teamDivisionOrder))
	for _, division := range teamDivisionOrder {
		playoffTeams[division] = r.Form[divisionPlayoffTeamsFieldName(division)]
		winners[division] = r.FormValue(divisionWinnerFieldName(division))
	}
	return playoffTeams, winners
}

// dedupTeamIDs returns ids with duplicates removed, preserving first-seen
// order - a division's submitted checkbox values must never be
// double-counted against its cap or double-persisted for a repeated id
// (Design Notes correction, review loop 1).
func dedupTeamIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// divisionsHaveAnInvalidPlayoffTeam reports whether any division in
// playoffTeams contains a team id foreign to that division's own roster
// (e.g. submitted under the wrong division's field, or simply unknown).
func divisionsHaveAnInvalidPlayoffTeam(st *store.Store, playoffTeams map[string][]string) bool {
	for division, ids := range playoffTeams {
		members := divisionTeamRoster(st, division)
		for _, id := range ids {
			if !members.has(id) {
				return true
			}
		}
	}
	return false
}

// divisionCountsExceedCap reports whether any division in playoffTeams has
// more than maxTeamsPerDivision deduped selected teams. AD-10 requires the
// server to "independently re-validate and enforce every cap on submit
// regardless of what the client allowed" - this is the per-division half of
// that; divisionConferenceTotals/conferenceTotalsValid below is the
// per-conference half.
func divisionCountsExceedCap(playoffTeams map[string][]string) bool {
	for _, ids := range playoffTeams {
		if len(dedupTeamIDs(ids)) > maxTeamsPerDivision {
			return true
		}
	}
	return false
}

// divisionConferenceTotals sums, per conference, the deduped selected-team
// count across that conference's two divisions - groups derives each
// division's conference the same way newDivisionPickView does (Never: no
// hardcoded division->conference map).
func divisionConferenceTotals(groups []teamDivisionGroup, playoffTeams map[string][]string) map[string]int {
	totals := make(map[string]int)
	for _, confGroup := range groupDivisionsByConference(groups) {
		total := 0
		for _, group := range confGroup.Divisions {
			total += len(dedupTeamIDs(playoffTeams[group.Division]))
		}
		totals[confGroup.Conference] = total
	}
	return totals
}

// conferenceTotalsValid reports whether every conference in totals equals
// exactly requiredTeamsPerConference - an empty totals map (no conference
// groups at all) is never valid, matching AD-10's server-side
// re-validation stance (unlike the view-model's own AllValid, which
// defaults to valid when there are no conference groups to render at all -
// see the spec's Review Triage Log for why that divergence is accepted).
func conferenceTotalsValid(totals map[string]int) bool {
	if len(totals) == 0 {
		return false
	}
	for _, total := range totals {
		if total != requiredTeamsPerConference {
			return false
		}
	}
	return true
}

// divisionsHaveAnInvalidWinner reports whether any non-empty winner pick in
// winners doesn't belong to its own division's roster (per st.Teams()) - an
// empty pick is always valid, since winners are never part of the submit
// gate (FR-16).
func divisionsHaveAnInvalidWinner(st *store.Store, winners map[string]string) bool {
	for division, teamID := range winners {
		if teamID == "" {
			continue
		}
		if !divisionTeamRoster(st, division).has(teamID) {
			return true
		}
	}
	return false
}

// divisionsCountErrorText is the inline caption shown when a divisions
// submission fails the per-division max-5 cap, the per-conference total-8
// gate, or contains a team id foreign to the division it was submitted
// under - EXPERIENCE.md's Voice-and-Tone caption wording is authoritative
// over the click-dummy App.jsx's own placeholder wording (Design Notes).
const divisionsCountErrorText = "8 teams per conference — either a 4/4 split, or 5 in one division and 3 in the other."

// divisionsWinnerErrorText is the inline caption shown when a non-empty
// division-winner pick doesn't belong to its own division's roster.
const divisionsWinnerErrorText = "Pick a valid team for each division winner, or leave it blank."

// handleDivisionsSubmit handles divisionsSetID's branch of POST
// /predict/{id}: parses every division's playoff-teams checkboxes and
// winner select (parseDivisionsSubmission), then validates before anything
// is saved (Boundaries & Constraints: "Submit is all-or-nothing for the
// whole form"). A team id foreign to the division it was submitted under,
// any single division's deduped count exceeding maxTeamsPerDivision, or
// either conference's deduped total not exactly requiredTeamsPerConference
// all re-render the sheet (200) with divisionsCountErrorText and save
// nothing. A non-empty winner pick foreign to its own division's roster
// re-renders (200) with divisionsWinnerErrorText and also saves nothing -
// winners are otherwise never part of the submit gate (FR-16, no gate on an
// empty winner pick). A valid submission dedups every division's team ids
// before persisting them via one st.SaveDivisionPicks call and redirects to
// /predict (302).
func handleDivisionsSubmit(w http.ResponseWriter, r *http.Request, st *store.Store, set store.PredictionSet, playerID string, now time.Time) {
	playoffTeams, winners := parseDivisionsSubmission(r)

	switch {
	case divisionsHaveAnInvalidPlayoffTeam(st, playoffTeams),
		divisionCountsExceedCap(playoffTeams),
		!conferenceTotalsValid(divisionConferenceTotals(groupTeamsByDivision(st.Teams()), playoffTeams)):
		renderRejectedDivisionsPick(w, r, st, set, playerID, now, playoffTeams, winners, divisionsCountErrorText)
		return
	case divisionsHaveAnInvalidWinner(st, winners):
		renderRejectedDivisionsPick(w, r, st, set, playerID, now, playoffTeams, winners, divisionsWinnerErrorText)
		return
	}

	deduped := make(map[string][]string, len(playoffTeams))
	for division, ids := range playoffTeams {
		deduped[division] = dedupTeamIDs(ids)
	}

	if err := st.SaveDivisionPicks(playerID, deduped, winners, now); err != nil {
		slog.Error("save division picks", "player_id", playerID, "error", err)
		http.Error(w, genericErrorBody, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/predict", http.StatusFound)
}

// renderRejectedDivisionsPick re-renders the divisionsSetID sheet (200) with
// the rejected submission's own playoffTeams/winners retained and errText as
// the inline caption - nothing is saved. err from newSheetData can only come
// from set.DeadlineUTC failing to parse, already parsed successfully by
// handleSheetSubmit moments earlier.
func renderRejectedDivisionsPick(w http.ResponseWriter, r *http.Request, st *store.Store, set store.PredictionSet, playerID string, now time.Time, playoffTeams map[string][]string, winners map[string]string, errText string) {
	data, err := newSheetData(st, set, playerID, now, nil)
	if err != nil {
		slog.Error("parse deadline_utc for prediction set", "prediction_set_id", set.ID, "error", err)
		http.NotFound(w, r)
		return
	}

	view := newDivisionPickView(st, playerID, data.DivisionPick.Submitted, playoffTeams, winners)
	view.Error = errText
	data.DivisionPick = &view

	renderTemplate(w, "sheet.html", data)
}
