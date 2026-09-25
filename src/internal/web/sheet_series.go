package web

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// stanleyCupFinalLabel is the single group heading
// store.StanleyCupFinalSetID's one matchup renders under, in place of an
// "Eastern Conference"/"Western Conference" subheader (Intent: the Final's
// two teams belong to different conferences, so conference-grouping doesn't
// apply to it).
const stanleyCupFinalLabel = "Stanley Cup Final"

// seriesConferenceLabelSuffix turns a Team.Conference value ("Eastern"/
// "Western") into its full group heading ("Eastern Conference"/"Western
// Conference") - grouping falls out of the matchup data itself (each
// series' own Team.Conference), never a second hardcoded per-round
// conference lookup (Design Notes, mirroring groupDivisionsByConference's
// own precedent).
const seriesConferenceLabelSuffix = " Conference"

// seriesGameOptions is the fixed 4-7 game-count button set every series card
// renders (Intent: "four number buttons 4/5/6/7") - never hand-maintained
// data.
var seriesGameOptions = []string{"4", "5", "6", "7"}

// seriesErrorText is the inline caption shown on a series card whose
// submitted team_id/games was non-blank but invalid - a team_id matching
// neither of the series' own two teams, or a games value outside
// seriesGameOptions (AD-10's server-revalidates stance). A half-filled but
// otherwise valid card (only one of the two fields set) never shows this;
// it's simply not saved (Intent).
const seriesErrorText = "Pick a valid team and game count (4-7), or leave the series blank."

// seriesTeamFieldName/seriesGamesFieldName name one series' winner-pick and
// game-count-pick submitted form fields, keyed by the matchup's own
// (round-scoped) Key - divisionPlayoffTeamsFieldName/divisionWinnerFieldName's
// own per-field naming precedent.
func seriesTeamFieldName(key string) string  { return "series_team_" + key }
func seriesGamesFieldName(key string) string { return "series_games_" + key }

// seriesCardPick is one series card's rendered state: TeamAID/TeamBID and
// TeamAName/TeamBName are its two full-width winner buttons; TeamField/
// GamesField are this series' two submitted form field names.
// SelectedTeamID/SelectedGames pre-fill the currently checked winner/
// game-count button, from either playerID's saved pick or a rejected
// resubmission's own retained values. Error is seriesErrorText once this one
// series was rejected as invalid on resubmission - every other series in the
// same submission renders independently of it (Intent).
type seriesCardPick struct {
	Key            string
	TeamAID        string
	TeamAName      string
	TeamBID        string
	TeamBName      string
	TeamField      string
	GamesField     string
	SelectedTeamID string
	SelectedGames  string
	Error          string
}

// seriesGroupPick is one rendered group of series cards under one subheader
// - "Eastern Conference"/"Western Conference" for every round but the
// Final, or the single stanleyCupFinalLabel group for it.
type seriesGroupPick struct {
	Label string
	Cards []seriesCardPick
}

// seriesPickView is the r1/r2/cf/scf sheet's whole pick-entry state: Groups
// holds every rendered group, in matchup order; Submitted mirrors
// divisionPickView's own field (setSubmitted's "has saved at least one row"
// convention); GameOptions is seriesGameOptions, exposed here so
// templates/sheet.html never hardcodes the 4-7 button set itself.
type seriesPickView struct {
	Groups      []seriesGroupPick
	Submitted   bool
	GameOptions []string
}

// seriesSubmission is one submitted series' raw team_id/games pair, as
// parsed off the form (parseSeriesSubmission) or retained from a rejected
// resubmission (renderRejectedSeriesPick) - awardSlotSubmission's own
// per-slot shape, scoped per series instead of per award-finalist slot.
type seriesSubmission struct {
	TeamID string
	Games  string
}

// seriesRejection carries a rejected series submission's retained values (by
// matchup Key) and which of those keys failed validation - passed to
// newSeriesPickView only when re-rendering a rejected resubmission
// (renderRejectedSeriesPick); nil for every other render, which falls back
// to playerID's saved picks and shows no error.
type seriesRejection struct {
	submission map[string]seriesSubmission
	invalid    map[string]bool
}

// teamName resolves id to its canonical Team.Name via teams, falling back to
// the raw id itself if unmatched (e.g. a hand-edited playoff_matchups entry
// naming a team no longer in the teams: roster) - displayNameForSlug's own
// degrade-to-raw-value precedent.
func teamName(teams roster[store.Team], id string) string {
	if team, ok := teams.get(id); ok {
		return team.Name
	}
	return id
}

// conferenceForMatchup returns m's own conference, read off its TeamA's
// Team.Conference (both of a series' teams always share one conference,
// except store.StanleyCupFinalSetID's own single matchup, which never calls
// this). An unmatched TeamA (e.g. a hand-edit mistake) degrades to grouping
// under an empty-string conference rather than panicking or dropping the
// card.
func conferenceForMatchup(st *store.Store, m store.PlayoffMatchup) string {
	if team, ok := teamRoster(st).get(m.TeamA); ok {
		return team.Conference
	}
	return ""
}

// selectedSeriesPick resolves m's currently-selected winner/games: a
// rejection's own retained (possibly invalid) submission when rejection is
// non-nil, otherwise playerID's saved FindSeriesPick row (blank when none is
// saved) - selectedTeamsForDivision/winnerForDivision's own
// override-vs-saved precedent.
func selectedSeriesPick(st *store.Store, playerID, setID string, m store.PlayoffMatchup, rejection *seriesRejection) (teamID, games string) {
	if rejection != nil {
		sub := rejection.submission[m.Key]
		return sub.TeamID, sub.Games
	}
	if prediction, ok := st.FindSeriesPick(playerID, store.JoinSeriesKey(setID, m.Key)); ok {
		return prediction.TeamID, prediction.Games
	}
	return "", ""
}

// newSeriesCardPick builds one matchup's rendered series card - factored out
// of newSeriesPickView purely to keep its own cyclomatic complexity low
// (gocyclo), mirroring newDivisionGroupPick's own precedent.
func newSeriesCardPick(st *store.Store, playerID, setID string, m store.PlayoffMatchup, rejection *seriesRejection) seriesCardPick {
	teams := teamRoster(st)
	selectedTeamID, selectedGames := selectedSeriesPick(st, playerID, setID, m, rejection)

	errText := ""
	if rejection != nil && rejection.invalid[m.Key] {
		errText = seriesErrorText
	}

	return seriesCardPick{
		Key:            m.Key,
		TeamAID:        m.TeamA,
		TeamAName:      teamName(teams, m.TeamA),
		TeamBID:        m.TeamB,
		TeamBName:      teamName(teams, m.TeamB),
		TeamField:      seriesTeamFieldName(m.Key),
		GamesField:     seriesGamesFieldName(m.Key),
		SelectedTeamID: selectedTeamID,
		SelectedGames:  selectedGames,
		Error:          errText,
	}
}

// groupSeriesByConference groups matchups into their two conferences, in
// each conference's own first-appearance order among matchups (no fixed,
// hand-maintained conference order exists for series, unlike
// store.Divisions()) - groupDivisionsByConference's own grouping shape,
// derived from the matchup data itself rather than a second hardcoded
// division/round->conference map.
func groupSeriesByConference(st *store.Store, playerID, setID string, matchups []store.PlayoffMatchup, rejection *seriesRejection) []seriesGroupPick {
	byConference := make(map[string][]seriesCardPick)
	var order []string
	for _, m := range matchups {
		conference := conferenceForMatchup(st, m)
		if _, seen := byConference[conference]; !seen {
			order = append(order, conference)
		}
		byConference[conference] = append(byConference[conference], newSeriesCardPick(st, playerID, setID, m, rejection))
	}

	groups := make([]seriesGroupPick, 0, len(order))
	for _, conference := range order {
		groups = append(groups, seriesGroupPick{Label: conference + seriesConferenceLabelSuffix, Cards: byConference[conference]})
	}
	return groups
}

// newSeriesPickView builds sheetData's SeriesPick field for playerID: one
// card per matchup recorded for set.ID (st.PlayoffMatchups) - however many
// that happens to be, never a hardcoded expected count (Design Notes) -
// grouped under "Eastern Conference"/"Western Conference" by each matchup's
// own conference, or the single stanleyCupFinalLabel group when set.ID is
// store.StanleyCupFinalSetID. rejection, when non-nil, retains a rejected
// resubmission's own values and per-series errors for re-rendering
// (renderRejectedSeriesPick); nil for every other render.
func newSeriesPickView(st *store.Store, set store.PredictionSet, playerID string, submitted bool, rejection *seriesRejection) seriesPickView {
	matchups := st.PlayoffMatchups(set.ID)

	var groups []seriesGroupPick
	switch set.ID {
	case store.StanleyCupFinalSetID:
		if len(matchups) > 0 {
			cards := make([]seriesCardPick, len(matchups))
			for i, m := range matchups {
				cards[i] = newSeriesCardPick(st, playerID, set.ID, m, rejection)
			}
			groups = []seriesGroupPick{{Label: stanleyCupFinalLabel, Cards: cards}}
		}
	default:
		groups = groupSeriesByConference(st, playerID, set.ID, matchups, rejection)
	}

	return seriesPickView{Groups: groups, Submitted: submitted, GameOptions: seriesGameOptions}
}

// parseSeriesSubmission reads every recorded matchup's team_id/games fields
// off the submitted form, keyed by the matchup's own Key - only for
// matchups actually recorded for the set being submitted (Design Notes: no
// round gets a hardcoded expected series count). r.Form must already be
// populated (r.ParseForm), parseDivisionsSubmission's own per-set-id
// precedent.
func parseSeriesSubmission(r *http.Request, matchups []store.PlayoffMatchup) map[string]seriesSubmission {
	submission := make(map[string]seriesSubmission, len(matchups))
	for _, m := range matchups {
		submission[m.Key] = seriesSubmission{
			TeamID: strings.TrimSpace(r.FormValue(seriesTeamFieldName(m.Key))),
			Games:  strings.TrimSpace(r.FormValue(seriesGamesFieldName(m.Key))),
		}
	}
	return submission
}

// seriesSubmissionIsBlank reports whether sub has neither a team nor a
// game-count pick - the "an empty series pick never blocks other picks"
// case (Intent): simply not saved, and never an error.
func seriesSubmissionIsBlank(sub seriesSubmission) bool {
	return sub.TeamID == "" && sub.Games == ""
}

// seriesSubmissionIsHalfFilled reports whether sub has exactly one of
// team_id/games set, the other left blank - Intent's own explicit "a half-
// filled series ... is simply not saved and shows no error" case, extending
// seriesSubmissionIsBlank's "never an error" treatment to a partial pick as
// well as a fully empty one. Whether the one filled field's own value would
// itself be valid is irrelevant here - it's presence, not validity, that
// makes a submission half-filled rather than invalid.
func seriesSubmissionIsHalfFilled(sub seriesSubmission) bool {
	return (sub.TeamID == "") != (sub.Games == "")
}

// seriesSubmissionIsComplete reports whether sub has both a team pick
// matching one of m's own two teams and a game-count pick within
// seriesGameOptions - the only shape handleSeriesSubmit ever persists via
// SaveSeriesPick. A caller only ever reaches this once
// seriesSubmissionIsBlank/seriesSubmissionIsHalfFilled have both already
// returned false, i.e. both fields are non-blank - a false result here means
// at least one of the two is invalid, not merely absent.
func seriesSubmissionIsComplete(m store.PlayoffMatchup, sub seriesSubmission) bool {
	return (sub.TeamID == m.TeamA || sub.TeamID == m.TeamB) && slices.Contains(seriesGameOptions, sub.Games)
}

// handleSeriesSubmit handles a series-pickable set id's branch of POST
// /predict/{id}: parses every recorded matchup's team_id/games
// (parseSeriesSubmission), then walks each one independently - unlike
// handleAwardsSubmit's whole-form gate, a series submission never blocks on
// another series (Design Notes). A blank series (neither field set) or a
// half-filled one (exactly one field set) is simply skipped - Intent's own
// "shows no error" case for both. A complete, valid series (a team matching
// one of its own two teams, and a games value within seriesGameOptions) is
// saved immediately via st.SaveSeriesPick - the winner and game-count save
// together as one row. Any other submission - both fields set, but a
// foreign team_id or an out-of-range games value - is collected as invalid
// and saves nothing for that one series. Once every matchup has been
// walked, an empty invalid set redirects to /predict (302); otherwise the
// sheet re-renders (200) with every invalid series' own inline error, while
// every valid series saved moments earlier stays saved.
func handleSeriesSubmit(w http.ResponseWriter, r *http.Request, st *store.Store, set store.PredictionSet, playerID string, now time.Time) {
	matchups := st.PlayoffMatchups(set.ID)
	submission := parseSeriesSubmission(r, matchups)

	invalid := make(map[string]bool)
	for _, m := range matchups {
		sub := submission[m.Key]
		switch {
		case seriesSubmissionIsBlank(sub), seriesSubmissionIsHalfFilled(sub):
			continue
		case seriesSubmissionIsComplete(m, sub):
			if err := st.SaveSeriesPick(playerID, store.JoinSeriesKey(set.ID, m.Key), sub.TeamID, sub.Games, now); err != nil {
				slog.Error("save series pick", "player_id", playerID, "series_key", store.JoinSeriesKey(set.ID, m.Key), "error", err)
				http.Error(w, genericErrorBody, http.StatusInternalServerError)
				return
			}
		default:
			invalid[m.Key] = true
		}
	}

	if len(invalid) > 0 {
		renderRejectedSeriesPick(w, r, st, set, playerID, now, &seriesRejection{submission: submission, invalid: invalid})
		return
	}

	http.Redirect(w, r, "/predict", http.StatusFound)
}

// renderRejectedSeriesPick re-renders set's sheet (200) with rejection's own
// retained team_id/games values per series and an inline error caption on
// every series rejection marks invalid - the valid, complete series were
// already saved by handleSeriesSubmit's own per-series walk before this is
// ever called. err from newSheetData can only come from set.DeadlineUTC
// failing to parse, already parsed successfully by handleSheetSubmit moments
// earlier.
func renderRejectedSeriesPick(w http.ResponseWriter, r *http.Request, st *store.Store, set store.PredictionSet, playerID string, now time.Time, rejection *seriesRejection) {
	data, err := newSheetData(st, set, playerID, now, nil)
	if err != nil {
		slog.Error("parse deadline_utc for prediction set", "prediction_set_id", set.ID, "error", err)
		http.NotFound(w, r)
		return
	}

	view := newSeriesPickView(st, set, playerID, data.SeriesPick.Submitted, rejection)
	data.SeriesPick = &view

	renderTemplate(w, "sheet.html", data)
}
