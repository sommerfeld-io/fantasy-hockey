// Package web is the presentation layer: it serves the app shell
// (Predict/Leaderboard/Compare) and the login-code request flow over the
// standard library's net/http, rendering server-side html/template views.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// templates holds every parsed view. Parsing once at package init means a
// broken template fails fast at startup rather than on first render.
var templates = template.Must(template.ParseFS(templatesFS, "templates/*.html"))

// genericErrorBody is shown for a failure a Player can't do anything about
// (e.g. a disk write failing) - deliberately generic, never leaking
// internals into the response.
const genericErrorBody = "Something went wrong. Please try again."

// genericCodeErrorText is shown on the code-entry screen for a wrong,
// expired, or already-used code alike - the three cases are never
// distinguished, so a submitted code can't be used to probe which one
// happened.
const genericCodeErrorText = "That code didn't work — check it and try again."

// maxLoginFormBytes bounds the POST /login body: an email address needs a
// few hundred bytes at most, so this leaves generous headroom while still
// capping how much an unbounded request body can make the server read.
const maxLoginFormBytes = 4096

// maxCodeFormBytes bounds the POST /login/code body the same way
// maxLoginFormBytes bounds POST /login: a 6-digit code needs only a handful
// of bytes, so this leaves generous headroom while still capping how much an
// unbounded request body can make the server read.
const maxCodeFormBytes = 4096

// Bottom-nav tab identifiers, matching shell.html's ActiveTab comparisons
// and this file's tabTitles/tabMessages lookups below.
const (
	tabPredict     = "predict"
	tabLeaderboard = "leaderboard"
	tabCompare     = "compare"
)

// tabTitles names each tab for the shell's <title> and nav label alike.
var tabTitles = map[string]string{
	tabPredict:     "Predict",
	tabLeaderboard: "Leaderboard",
	tabCompare:     "Compare",
}

// tabMessages is each remaining placeholder tab's short, unique "Coming
// soon" message - Epics 4/5 replace these with real functionality. Predict
// no longer has an entry: Story 2.1 replaced its placeholder with the real
// phase-grouped Prediction Set lists (shellData.Predict), so handleShell
// never looks this map up for tabPredict.
var tabMessages = map[string]string{
	tabLeaderboard: "The leaderboard is coming soon.",
	tabCompare:     "Player comparison is coming soon.",
}

// predictSheetPattern is the single source of truth for the per-Prediction-
// Set sheet page's route, registered on both authMux and the outer mux so the
// two can't drift out of sync, the same way shellRoutes is for the bottom-nav
// routes.
const predictSheetPattern = "GET /predict/{id}"

// predictSheetSubmitPattern is the single source of truth for the cup/
// presidents pick submission route, registered on both authMux and the outer
// mux the same way predictSheetPattern is, so the two can't drift.
const predictSheetSubmitPattern = "POST /predict/{id}"

// Prediction Set phases, matching fantasy-hockey.yml's prediction_sets[].phase
// values and the Predict screen's two section labels (DESIGN.md's Section
// header icon component: a target icon for "Before the season," a trophy
// icon for "Playoffs").
const (
	phaseBeforeSeason = "before_season"
	phasePlayoffs     = "playoffs"
)

// Prediction Set status labels and their status-pill CSS classes
// (styles.css).
const (
	statusOpen      = "Open"
	statusSubmitted = "Submitted"
	statusClosed    = "Closed"
	statusUpcoming  = "Upcoming"

	statusPillOpen      = "status-pill--open"
	statusPillSubmitted = "status-pill--submitted"
	statusPillClosed    = "status-pill--closed"
	statusPillUpcoming  = "status-pill--upcoming"
)

// predictSetView is one Predict-screen row: every field is already
// presentation-ready (formatted deadline, computed status/pill/accent), so
// shell.html only renders - it never computes status or formats a
// timestamp itself (Boundaries & Constraints: "one reusable helper, not
// duplicated per template").
type predictSetView struct {
	ID             string
	Title          string
	Subtitle       string
	DeadlineText   string
	Countdown      string
	CountdownFaint bool // true for Upcoming rows (DESIGN.md: faint countdown when Upcoming, ice otherwise)
	Status         string
	StatusPillCSS  string
	AccentCSS      string
	Actionable     bool // Open/Submitted/Closed rows link to /predict/{id}; Upcoming rows don't
}

// predictPhases is Predict's two phase-grouped sections.
type predictPhases struct {
	BeforeSeason []predictSetView
	Playoffs     []predictSetView
}

// newPredictSetView derives set's presentation-ready row from its
// hand-maintained YAML fields, evaluating status/countdown against now.
// submitted is whether the current player already has a saved Prediction row
// for set (st.FindPrediction(playerID, set.ID)); it only ever affects
// non-upcoming sets, since Upcoming always wins (Boundaries & Constraints).
// An error means set.DeadlineUTC isn't valid RFC3339 - a hand-edit mistake in
// fantasy-hockey.yml, not something a caller can recover from per-row, so
// the caller drops the row rather than rendering a broken one.
func newPredictSetView(set store.PredictionSet, submitted bool, now time.Time) (predictSetView, error) {
	deadline, err := time.Parse(time.RFC3339, set.DeadlineUTC)
	if err != nil {
		return predictSetView{}, fmt.Errorf("parse deadline_utc %q: %w", set.DeadlineUTC, err)
	}

	status, pillCSS := predictStatus(set.Upcoming, submitted, deadline, now)
	accentCSS := "set-row--open"
	switch {
	case set.Upcoming:
		accentCSS = "set-row--upcoming"
	case submitted:
		accentCSS = "set-row--submitted"
	}

	return predictSetView{
		ID:             set.ID,
		Title:          set.Title,
		Subtitle:       set.Subtitle,
		DeadlineText:   clock.FormatDeadline(deadline),
		Countdown:      clock.Countdown(deadline, now),
		CountdownFaint: set.Upcoming,
		Status:         status,
		StatusPillCSS:  pillCSS,
		AccentCSS:      accentCSS,
		Actionable:     !set.Upcoming,
	}, nil
}

// predictStatus computes a Prediction Set's status label and status-pill CSS
// class from its Upcoming flag, whether the player already has a saved pick
// (submitted), and its deadline. Precedence is Upcoming > Closed > Submitted
// > Open - a deadline exactly at now counts as closed (now is never "still
// open" once it reaches the deadline), and a closed set never shows
// Submitted even if a pick was saved before the deadline passed.
func predictStatus(upcoming, submitted bool, deadline, now time.Time) (label, pillCSS string) {
	switch {
	case upcoming:
		return statusUpcoming, statusPillUpcoming
	case !deadline.After(now):
		return statusClosed, statusPillClosed
	case submitted:
		return statusSubmitted, statusPillSubmitted
	default:
		return statusOpen, statusPillOpen
	}
}

// buildPredictPhases groups st's Prediction Sets into their two Predict
// sections, in the order fantasy-hockey.yml lists them. playerID is the
// logged-in player's id (may be empty for a session lookup miss), used to
// look up each set's saved Prediction (kind == set.ID) so its row can show
// Submitted status - a set with no matching Prediction.Kind (every id other
// than "cup"/"presidents", today) naturally never finds one. A row whose
// deadline_utc fails to parse is logged and skipped rather than failing the
// whole page - one hand-edit mistake shouldn't take down every other
// Prediction Set. A row with a phase other than "before_season" or
// "playoffs" is likewise logged and skipped.
func buildPredictPhases(st *store.Store, playerID string, now time.Time) predictPhases {
	var phases predictPhases
	for _, set := range st.PredictionSets() {
		submitted := setSubmitted(st, set, playerID)
		view, err := newPredictSetView(set, submitted, now)
		if err != nil {
			slog.Error("build predict set view", "prediction_set_id", set.ID, "error", err)
			continue
		}

		switch set.Phase {
		case phaseBeforeSeason:
			phases.BeforeSeason = append(phases.BeforeSeason, view)
		case phasePlayoffs:
			phases.Playoffs = append(phases.Playoffs, view)
		default:
			slog.Error("unknown prediction set phase", "prediction_set_id", set.ID, "phase", set.Phase)
		}
	}
	return phases
}

// teamOption is one entry in the shared autocomplete embed shape (AD-19):
// every embedding site (this story's teams, and 2.4/2.6's own data) renders
// this identical {"id", "label"} JSON object, and the widget always submits
// ID, never Label, into its bound form field.
type teamOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// newTeamOptions converts teams into the shared embed shape, preserving
// input order. Not called from any handler yet - Story 2.3's cup/presidents
// sheet renders native <option>s from Store.Teams() directly instead
// (Boundaries & Constraints: no JSON-embed shape for this story); 2.4/2.6
// embed it once they add their own dropdowns/chips.
func newTeamOptions(teams []store.Team) []teamOption {
	options := make([]teamOption, len(teams))
	for i, team := range teams {
		options[i] = teamOption{ID: team.ID, Label: team.Name}
	}
	return options
}

// shellData feeds templates/shell.html. PlayerName is empty when the
// session's player id has no matching player left in the store (a
// stale/deleted id) - the header then degrades to a neutral state instead
// of a 500 or panic. Season is already presentation-formatted (e.g. "NHL
// 2026–27"); ActiveTab selects which bottom-nav tab renders active; Title is
// every tab's page title. Message is Leaderboard/Compare's static
// placeholder content and stays empty for Predict; Predict is nil for every
// other tab and holds Predict's real, phase-grouped content instead.
type shellData struct {
	PlayerName string
	Season     string
	ActiveTab  string
	Title      string
	Message    string
	Predict    *predictPhases
}

// formatSeason turns the store's raw season value (e.g. "2026-27") into the
// header's display form ("NHL 2026–27") - presentation-only, matching how
// the login mockups already hardcode this same display string.
func formatSeason(season string) string {
	return "NHL " + strings.Replace(season, "-", "–", 1)
}

// loginCodeData feeds templates/login-code.html: Code is the submitted
// value (retained on error so the Player doesn't have to retype it), and
// Error is the generic message shown for any wrong/expired/used code, empty
// when there's nothing to report.
type loginCodeData struct {
	Code  string
	Error string
}

// shellRoutes is the single source of truth for every authenticated
// app-shell route this server registers, pairing each route pattern with
// the bottom-nav tab it renders active. NewServer registers each pattern on
// both authMux and the outer mux from this one list, so the two can no
// longer drift out of sync as future stories add routes (AD-2/AD-11). GET
// /{$} aliases to the same Predict shell as GET /predict.
var shellRoutes = []struct {
	pattern string
	tab     string
}{
	{"GET /{$}", tabPredict},
	{"GET /predict", tabPredict},
	{"GET /leaderboard", tabLeaderboard},
	{"GET /compare", tabCompare},
}

// NewServer wires the application's routes and returns an http.Handler
// ready to be served. st and send back the login-code request flow
// (GET/POST /login, POST /login/code); secret signs the session cookie a
// successful POST /login/code sets and verifies the ones requireSession
// reads back. shellRoutes' authenticated app-shell routes are registered on
// their own authMux, which requireSession wraps before it's mounted on the
// outer mux alongside the public routes - a future protected route joins
// shellRoutes the same way, without touching how public routes are wired.
// GET /predict/{id} (the per-Prediction-Set sheet page) and POST /predict/{id}
// (its cup/presidents pick submission) are registered the same way but
// outside shellRoutes, since neither is a bottom-nav destination; they get
// their own handlers (handleSheet, handleSheetSubmit) rather than
// handleShell.
// POST /logout is registered directly on the outer mux rather than authMux,
// since clearing the session cookie must work even when the presented
// cookie is missing, expired, or tampered.
func NewServer(st *store.Store, send mailer.Sender, secret string) http.Handler {
	authMux := http.NewServeMux()
	for _, r := range shellRoutes {
		authMux.Handle(r.pattern, handleShell(st, r.tab))
	}
	authMux.Handle(predictSheetPattern, handleSheet(st))
	authMux.Handle(predictSheetSubmitPattern, handleSheetSubmit(st))
	protected := requireSession(secret, authMux)

	mux := http.NewServeMux()
	for _, r := range shellRoutes {
		mux.Handle(r.pattern, protected)
	}
	mux.Handle(predictSheetPattern, protected)
	mux.Handle(predictSheetSubmitPattern, protected)
	mux.HandleFunc("GET /login", handleLoginForm(secret))
	mux.HandleFunc("POST /login", handleLoginSubmit(st, send))
	mux.HandleFunc("POST /login/code", handleLoginCodeSubmit(st, secret))
	mux.HandleFunc("POST /logout", handleLogout)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFiles())))
	return mux
}

// requireSession wraps next so it only runs for a request carrying a valid,
// unexpired session cookie (auth.ValidateSession). A valid session is
// re-issued with a fresh issued_at before next runs, sliding the idle
// timeout forward (PRD FR-3); anything else - a missing cookie, a bad
// signature, an empty decoded player id, or one idle-expired past
// auth.SessionIdleTimeout - redirects to /login with no distinguishing
// message, since auth.ValidateSession never says which case occurred. Every
// authenticated response also gets Cache-Control: no-store, since a shared
// cache in front of the app could otherwise serve one player's page to
// another once this route's content stops being identical for everyone. The
// validated player id is threaded onto the request context (via
// auth.ContextWithPlayerID) so next and anything it calls can read identity
// without re-parsing the cookie.
func requireSession(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie(auth.SessionCookieName)
		playerID, ok := auth.ValidateSession(c, secret)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		http.SetCookie(w, auth.IssueSessionCookie(playerID, secret))
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r.WithContext(auth.ContextWithPlayerID(r.Context(), playerID)))
	})
}

// staticFiles returns static/'s contents rooted at "/", so a request for
// /static/styles.css maps to the embedded static/styles.css.
func staticFiles() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// static/ is embedded at build time via go:embed above, so this
		// can only fail from a programmer error, not runtime input.
		panic(fmt.Sprintf("web: static assets: %v", err))
	}
	return sub
}

// handleShell renders the persistent app shell for tab: a pinned header
// (player name + season + logout control), a pinned bottom nav with tab
// active, and that tab's content - Predict's real, phase-grouped Prediction
// Set lists, or Leaderboard/Compare's short static "Coming soon" message.
// The player id comes from the request context requireSession populates; a
// stale/deleted id with no matching player left in st degrades to an empty
// PlayerName (a neutral, non-crashing header) rather than a 500 or panic,
// logging the lookup miss server-side.
func handleShell(st *store.Store, tab string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, ok := auth.PlayerIDFromContext(r.Context())

		var name string
		if ok {
			if player, found := st.FindPlayerByID(playerID); found {
				name = player.Name
			} else {
				slog.Error("resolve player for shell header", "player_id", playerID)
			}
		}

		data := shellData{
			PlayerName: name,
			Season:     formatSeason(st.Season()),
			ActiveTab:  tab,
			Title:      tabTitles[tab],
			Message:    tabMessages[tab],
		}
		if tab == tabPredict {
			phases := buildPredictPhases(st, playerID, clock.NowTime())
			data.Predict = &phases
		}

		renderTemplate(w, "shell.html", data)
	}
}

// divisionsSetID is the one Prediction Set id that gets Story 2.4's
// checkbox-chip playoff-teams-and-winners form rather than a single-team
// dropdown - referenced everywhere this file dispatches on it, so the id
// literal can't drift between call sites.
const divisionsSetID = "divisions"

// pickableSheetKinds is the set of Prediction Set ids that get a real
// pick-entry form instead of the static stub - Story 2.3's single-team
// dropdown for the "cup" and "presidents" ids, Story 2.4's checkbox-chip
// form for divisionsSetID. Every other id keeps rendering the stub
// unchanged.
var pickableSheetKinds = map[string]bool{
	store.KindCupChampion:      true,
	store.KindPresidentsTrophy: true,
	divisionsSetID:             true,
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
	roster := make(map[string]bool, len(group.Teams))
	for _, team := range group.Teams {
		roster[team.ID] = true
	}

	set := make(map[string]bool)
	for _, id := range selected {
		if roster[id] {
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

// setSubmitted reports whether playerID already has a saved pick for set,
// used both by buildPredictPhases (the Predict list's Submitted status) and
// newSheetData (the sheet's Update-vs-Submit button text). For every id
// other than divisionsSetID this is Story 2.1's single (PlayerID, Kind ==
// set.ID) lookup; divisionsSetID can't use that lookup, since its picks are
// stored across per-division rows keyed by (PlayerID, Kind, Division) rather
// than one row keyed by Kind == set.ID (AD-28) - submitted there means "has
// saved at least one playoff-teams row for any division," matching the
// reference App.jsx's own single "submitted" flag per set rather than one
// per division.
func setSubmitted(st *store.Store, set store.PredictionSet, playerID string) bool {
	if set.ID == divisionsSetID {
		for _, division := range teamDivisionOrder {
			if _, ok := st.FindDivisionPlayoffTeams(playerID, division); ok {
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
// for every Prediction Set id. Closed, Pick, and DivisionPick are only
// populated for pickableSheetKinds ids (handleSheet/handleSheetSubmit) -
// Pick populates for "cup"/"presidents", DivisionPick for divisionsSetID;
// both stay nil for every other id, which keeps rendering the static "not
// available yet." stub (Boundaries & Constraints).
type sheetData struct {
	ID           string
	Title        string
	DeadlineText string
	Countdown    string
	Closed       bool
	Pick         *pickView
	DivisionPick *divisionPickView
}

// newSheetData builds sheetData for set as seen by playerID at now: the
// common Title/DeadlineText/Countdown/Closed fields every id gets, plus a
// populated Pick for the "cup"/"presidents" ids or a populated DivisionPick
// for divisionsSetID (built from playerID's saved picks). teamIDOverride,
// when non-nil, replaces the saved pick's team id in the rendered cup/
// presidents form - used by renderRejectedPick to re-render the sheet with
// the rejected submission's (invalid) value instead of the last-saved one;
// it has no effect for divisionsSetID, whose own rejected-resubmission
// re-render goes through renderRejectedDivisionsPick and newDivisionPickView
// directly instead. err is non-nil only when set.DeadlineUTC fails to parse,
// mirroring newPredictSetView.
func newSheetData(st *store.Store, set store.PredictionSet, playerID string, now time.Time, teamIDOverride *string) (sheetData, error) {
	submitted := setSubmitted(st, set, playerID)

	view, err := newPredictSetView(set, submitted, now)
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
// path, so it can't be used to probe which ids exist. A row whose
// deadline_utc fails to parse is treated the same way, since there's nothing
// sensible to render for it either.
func handleSheet(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		playerID, _ := auth.PlayerIDFromContext(r.Context())

		set, found := findPredictionSetByID(st, id)
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

// findPredictionSetByID returns the Prediction Set matching id, if any -
// shared by handleSheet and handleSheetSubmit so the lookup can't drift
// between the two.
func findPredictionSetByID(st *store.Store, id string) (store.PredictionSet, bool) {
	for _, set := range st.PredictionSets() {
		if set.ID == id {
			return set, true
		}
	}
	return store.PredictionSet{}, false
}

// isKnownTeamID reports whether teamID matches one of st.Teams() - the
// server-side re-validation AD-10 requires regardless of what the client
// submitted.
func isKnownTeamID(st *store.Store, teamID string) bool {
	for _, team := range st.Teams() {
		if team.ID == teamID {
			return true
		}
	}
	return false
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
// submission (single team_id) or, for divisionsSetID, the division
// playoff-teams-and-winners submission (handleDivisionsSubmit). {id} not
// being a pickableSheetKinds id, or matching no Prediction Set, gets a
// generic http.StatusNotFound - identical to handleSheet's own unknown-id
// handling, so it can't be used to probe which ids exist. A closed
// (past-deadline) set rejects the submission outright with
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

		set, found := findPredictionSetByID(st, id)
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

		teamID := r.FormValue("team_id")
		if !isKnownTeamID(st, teamID) {
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

// divisionRoster returns the set of team ids belonging to division, per
// st.Teams() - used to validate a submitted team id actually belongs to the
// division it was submitted under (AD-10).
func divisionRoster(st *store.Store, division string) map[string]bool {
	roster := make(map[string]bool)
	for _, team := range st.Teams() {
		if team.Division == division {
			roster[team.ID] = true
		}
	}
	return roster
}

// allBelongToRoster reports whether every id in ids is present in roster.
func allBelongToRoster(ids []string, roster map[string]bool) bool {
	for _, id := range ids {
		if !roster[id] {
			return false
		}
	}
	return true
}

// divisionsHaveAnInvalidPlayoffTeam reports whether any division in
// playoffTeams contains a team id foreign to that division's own roster
// (e.g. submitted under the wrong division's field, or simply unknown).
func divisionsHaveAnInvalidPlayoffTeam(st *store.Store, playoffTeams map[string][]string) bool {
	for division, ids := range playoffTeams {
		if !allBelongToRoster(ids, divisionRoster(st, division)) {
			return true
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
		if !divisionRoster(st, division)[teamID] {
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

// handleLoginForm renders the email-entry step of the login flow, unless
// the request already carries a valid session - a still-logged-in player
// following a bookmark or the back button straight back to /login is sent
// into the shell instead of being shown the anonymous form again.
func handleLoginForm(secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie(auth.SessionCookieName)
		if _, ok := auth.ValidateSession(c, secret); ok {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		renderTemplate(w, "login-email.html", nil)
	}
}

// handleLoginSubmit matches the submitted email against st and always
// renders the code-entry step - the response is identical whether or not
// the email matched (FR-1). Only a failure to persist the new login code
// surfaces as a 500; a failure to email it is handled (logged) entirely
// inside internal/auth and never reaches here.
func handleLoginSubmit(st *store.Store, send mailer.Sender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxLoginFormBytes)
		if err := r.ParseForm(); err != nil {
			slog.Error("parse login form", "error", err)
			http.Error(w, genericErrorBody, http.StatusInternalServerError)
			return
		}

		email := r.FormValue("email")
		if err := auth.RequestLoginCode(st, send, email); err != nil {
			slog.Error("request login code", "error", err)
			http.Error(w, genericErrorBody, http.StatusInternalServerError)
			return
		}

		renderTemplate(w, "login-code.html", loginCodeData{})
	}
}

// handleLoginCodeSubmit validates the submitted code against st. A valid,
// unused, unexpired code sets a signed session cookie (secret-keyed), marks
// that LoginCode row used, and redirects to the home placeholder (Story
// 1.5 builds the real destination). A wrong, expired, or already-used code
// all re-render the same code-entry screen (401, not 200 - the credential
// presented was rejected) with an identical generic error and the submitted
// value retained (FR-2) - ValidateLoginCode never tells the three cases
// apart, so this handler can't leak which one happened either. Only a
// failure to persist the consumed row surfaces as a 500.
func handleLoginCodeSubmit(st *store.Store, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxCodeFormBytes)
		if err := r.ParseForm(); err != nil {
			slog.Error("parse login-code form", "error", err)
			http.Error(w, genericErrorBody, http.StatusInternalServerError)
			return
		}

		code := strings.TrimSpace(r.FormValue("code"))
		playerID, ok, err := auth.ValidateLoginCode(st, code)
		if err != nil {
			slog.Error("validate login code", "error", err)
			http.Error(w, genericErrorBody, http.StatusInternalServerError)
			return
		}
		if !ok || playerID == "" {
			if ok {
				slog.Error("validate login code", "error", "matched a login code row with an empty player id")
			}
			renderTemplateStatus(w, http.StatusUnauthorized, "login-code.html", loginCodeData{Code: code, Error: genericCodeErrorText})
			return
		}

		http.SetCookie(w, auth.IssueSessionCookie(playerID, secret))
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// handleLogout clears the session cookie and redirects to /login. It's
// intentionally not wrapped by requireSession: a player with an already-
// expired or tampered cookie still needs logout to work, so clearing is
// unconditional and idempotent regardless of what cookie (if any) was
// presented.
func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, auth.ClearSessionCookie())
	http.Redirect(w, r, "/login", http.StatusFound)
}

// renderTemplate writes name to w with data available to it and an implicit
// 200 status. See renderTemplateStatus for a response that must not default
// to 200.
func renderTemplate(w http.ResponseWriter, name string, data any) {
	renderTemplateStatus(w, http.StatusOK, name, data)
}

// renderTemplateStatus is renderTemplate with an explicit status code,
// logging (rather than surfacing) a rendering failure, since the response
// has typically already started streaming by the time html/template can
// fail. The status must be written before any body bytes, so this always
// calls WriteHeader itself rather than letting the first Write default to
// 200.
func renderTemplateStatus(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("render template", "template", name, "error", err)
	}
}
