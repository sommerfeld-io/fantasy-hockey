package web

import (
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Compare selector group labels, matching the Predict screen's section
// labels for the same two phases.
const (
	compareGroupBeforeSeason = "Before the season"
	compareGroupPlayoffs     = "Playoffs"
)

// emptyCellValue is what a Compare cell shows when a player has no value for
// its category, so a cell is never blank.
const emptyCellValue = "—"

// Compare row labels and label fragments (click-dummy wording).
const (
	compareCupLabel             = "Stanley Cup winner"
	comparePresidentsLabel      = "Presidents' Trophy"
	compareDivisionPlayoffLabel = "%s — playoff teams"
	compareDivisionWinnerLabel  = "%s — winner"
	compareSeriesLabel          = "%s · %s vs %s"
	compareSeriesNoConfLabel    = "%s vs %s"
	compareSeriesGamesSuffix    = " in %s"
)

// compareGatedRoundNote replaces the table when a hand-typed ?set= names a
// real, still-gated playoff round (Boundaries: unreachable via any rendered
// chip - a gated round's chip never renders, so this only matters for a
// direct URL).
const compareGatedRoundNote = "Matchups not set."

// Compare CSS classes (styles.css): chips reuse the shared Chip component
// with a --selected modifier; the signed-in player's column carries an own
// modifier on its header and every one of its cells.
const (
	compareChipCSS         = "chip compare-chip"
	compareChipSelectedCSS = "chip compare-chip chip--selected"
	comparePlayerCSS       = "cmp-player"
	comparePlayerOwnCSS    = "cmp-player cmp-player--own"
	compareCellCSS         = "cmp-cell"
	compareCellOwnCSS      = "cmp-cell cmp-cell--own"
)

// Compare value CSS classes (styles.css): a team abbreviation (playoff
// team, division winner, series winner) gets the small cmp-tag treatment; a
// full team name, an award finalist's display name, and a series row's
// " in N" suffix are plain; an unfilled value renders faint instead of
// blank.
const (
	compareValueCSS      = "cmp-value"
	compareValueTagCSS   = "cmp-value cmp-tag"
	compareValueEmptyCSS = "cmp-value cmp-value--empty"
)

// singleTeamSetLabels maps each single-team Prediction Set id to its one
// row's label. Both Cup picks share a label: they ask the same question at
// different times.
var singleTeamSetLabels = map[string]string{
	store.KindCupChampion:      compareCupLabel,
	store.KindPlayoffsCup:      compareCupLabel,
	store.KindPresidentsTrophy: comparePresidentsLabel,
}

// compareChipView is one selector chip, a link (Href) to /compare?set=<ID>.
type compareChipView struct {
	ID       string
	Title    string
	Href     string
	Selected bool
	CSS      string
}

// compareGroupView is one labelled selector row of chips. LabelID is the
// label's element id, which the chips' group references for assistive tech.
type compareGroupView struct {
	Label   string
	LabelID string
	Chips   []compareChipView
}

// compareColumnView is one player's column header.
type compareColumnView struct {
	Name string
	Own  bool
	CSS  string
}

// compareValueView is one value within a cell: its display text and
// precomputed CSS (tag, plain, or empty - compareValueTagCSS/compareValueCSS/
// compareValueEmptyCSS), mirroring the file's existing precomputed-CSS
// pattern (compareCellCSS) instead of template-side conditionals.
type compareValueView struct {
	Text string
	CSS  string
}

// tagValue is a team-abbreviation value (a playoff team, division winner,
// or series winner), rendered with the shared small tag treatment.
func tagValue(text string) compareValueView {
	return compareValueView{Text: text, CSS: compareValueTagCSS}
}

// tagValues is every id in ids as a tagValue, in order.
func tagValues(ids []string) []compareValueView {
	values := make([]compareValueView, 0, len(ids))
	for _, id := range ids {
		values = append(values, tagValue(id))
	}
	return values
}

// plainValue is a full team name, an award finalist's display name, or a
// series row's " in N" suffix - never tagged.
func plainValue(text string) compareValueView {
	return compareValueView{Text: text, CSS: compareValueCSS}
}

// emptyCompareValue is what a Compare cell shows when a player has no value
// for its category, so a cell is never blank - rendered faint rather than
// full-strength.
func emptyCompareValue() compareValueView {
	return compareValueView{Text: emptyCellValue, CSS: compareValueEmptyCSS}
}

// compareCellView is one player's value(s) for one category. Values is
// never empty: an unfilled value is a single emptyCompareValue().
type compareCellView struct {
	Values []compareValueView
	Own    bool
	CSS    string
}

// compareRowView is one prediction category: its label and one cell per
// column. Stacked rows show their values one per line (award finalists).
type compareRowView struct {
	Label   string
	Stacked bool
	Cells   []compareCellView
}

// compareTableView is the selected set's side-by-side table.
type compareTableView struct {
	Title        string
	DeadlineText string
	Countdown    string
	Columns      []compareColumnView
	ColumnCount  int
	Rows         []compareRowView
}

// compareView is the Compare tab's content: both selector groups, always
// present, and either the selected set's table, or - for a hand-typed
// ?set= naming a real, still-gated round - Note in its place (Table is nil
// whenever Note is set, and vice versa).
type compareView struct {
	Groups []compareGroupView
	Table  *compareTableView
	Note   string
}

// compareSet is one selectable Prediction Set with its parsed deadline.
type compareSet struct {
	set      store.PredictionSet
	deadline time.Time
}

// compareCategory is one row's label and how to read one player's values.
type compareCategory struct {
	label   string
	stacked bool
	values  func(playerID string) []compareValueView
}

// buildCompare builds the Compare tab for playerID (whose column, if any,
// is marked own) with selectedID's set shown, or the default set when
// selectedID isn't selectable. A selectedID naming a real, still-gated round
// (isGatedRound) shows Note instead of falling back - unreachable via any
// rendered chip, since a gated round's chip never appears, so this only
// matters for a hand-typed URL. It reads the store on every call and never
// writes to it.
func buildCompare(st *store.Store, playerID, selectedID string, now time.Time) compareView {
	sets := selectableCompareSets(st)
	selected, ok := findCompareSet(sets, selectedID)
	if !ok && isGatedRound(st, selectedID) {
		return compareView{Groups: newCompareGroups(sets, ""), Note: compareGatedRoundNote}
	}
	if !ok {
		selected, ok = defaultCompareSet(sets)
	}

	v := compareView{Groups: newCompareGroups(sets, selected.set.ID)}
	if ok {
		table := newCompareTable(st, playerID, selected, now)
		v.Table = &table
	}
	return v
}

// isGatedRound reports whether id is a roundGatedSetIDs id whose matchups
// aren't recorded yet - the one roundGatedSetIDs case effectiveUpcoming
// computes from playoff_matchups (predict.go) that selectableCompareSets
// excludes from its chips. Deliberately narrower than "any effectively
// Upcoming set": a before-season set or r1 (hand-gated by its own upcoming
// flag, not roundGatedSetIDs) marked Upcoming still falls back to the
// default set instead (5.1 behavior, unchanged) - only r2/conference
// finals/the Final show the note. No new store methods, just the existing
// PlayoffMatchups check.
func isGatedRound(st *store.Store, id string) bool {
	return roundGatedSetIDs[id] && len(st.PlayoffMatchups(id)) == 0
}

// selectableCompareSets is every Prediction Set that gets a chip, in file
// order: a known phase, a parseable deadline, and not effectively Upcoming.
// A bad deadline or unknown phase is logged and the set left out.
func selectableCompareSets(st *store.Store) []compareSet {
	var sets []compareSet
	for _, set := range st.PredictionSets() {
		if set.Phase != phaseBeforeSeason && set.Phase != phasePlayoffs {
			slog.Error("unknown prediction set phase", "prediction_set_id", set.ID, "phase", set.Phase)
			continue
		}
		deadline, err := time.Parse(time.RFC3339, set.DeadlineUTC)
		if err != nil {
			slog.Error("build compare chip", "prediction_set_id", set.ID, "error", fmt.Errorf("parse deadline_utc %q: %w", set.DeadlineUTC, err))
			continue
		}
		if effectiveUpcoming(st, set) {
			continue
		}
		sets = append(sets, compareSet{set: set, deadline: deadline})
	}
	return sets
}

func findCompareSet(sets []compareSet, id string) (compareSet, bool) {
	for _, s := range sets {
		if s.set.ID == id {
			return s, true
		}
	}
	return compareSet{}, false
}

// defaultCompareSet is the earliest-deadline "Before the season" set (the
// first listed on a tie), or the first selectable set when no
// before-the-season set is selectable.
func defaultCompareSet(sets []compareSet) (compareSet, bool) {
	var best compareSet
	found := false
	for _, s := range sets {
		if s.set.Phase == phaseBeforeSeason && (!found || s.deadline.Before(best.deadline)) {
			best, found = s, true
		}
	}
	if !found && len(sets) > 0 {
		return sets[0], true
	}
	return best, found
}

func newCompareGroups(sets []compareSet, selectedID string) []compareGroupView {
	groups := []compareGroupView{
		{Label: compareGroupBeforeSeason, LabelID: compareGroupLabelID(phaseBeforeSeason)},
		{Label: compareGroupPlayoffs, LabelID: compareGroupLabelID(phasePlayoffs)},
	}
	for _, s := range sets {
		i := 0
		if s.set.Phase == phasePlayoffs {
			i = 1
		}
		groups[i].Chips = append(groups[i].Chips, newCompareChip(s.set, s.set.ID == selectedID))
	}
	return groups
}

// compareGroupLabelID is the element id of phase's selector group label.
func compareGroupLabelID(phase string) string {
	return "compare-group-" + phase
}

func newCompareChip(set store.PredictionSet, selected bool) compareChipView {
	css := compareChipCSS
	if selected {
		css = compareChipSelectedCSS
	}
	return compareChipView{ID: set.ID, Title: set.Title, Href: compareSetHref(set.ID), Selected: selected, CSS: css}
}

// compareSetHref is the Compare tab's URL with setID selected.
func compareSetHref(setID string) string {
	return "/" + tabCompare + "?" + url.Values{compareSetQueryParam: {setID}}.Encode()
}

func newCompareTable(st *store.Store, playerID string, selected compareSet, now time.Time) compareTableView {
	players := st.Players()
	table := compareTableView{
		Title:        selected.set.Title,
		DeadlineText: clock.FormatDeadline(selected.deadline),
		Countdown:    clock.Countdown(selected.deadline, now),
		Columns:      newCompareColumns(players, playerID),
		ColumnCount:  len(players),
	}
	for _, c := range compareCategories(st, selected.set.ID) {
		table.Rows = append(table.Rows, newCompareRow(c, players, playerID))
	}
	return table
}

func newCompareColumns(players []store.Player, playerID string) []compareColumnView {
	columns := make([]compareColumnView, 0, len(players))
	for _, p := range players {
		own := p.ID == playerID
		css := comparePlayerCSS
		if own {
			css = comparePlayerOwnCSS
		}
		columns = append(columns, compareColumnView{Name: p.Name, Own: own, CSS: css})
	}
	return columns
}

func newCompareRow(c compareCategory, players []store.Player, playerID string) compareRowView {
	row := compareRowView{Label: c.label, Stacked: c.stacked}
	for _, p := range players {
		values := c.values(p.ID)
		if len(values) == 0 {
			values = []compareValueView{emptyCompareValue()}
		}
		own := p.ID == playerID
		css := compareCellCSS
		if own {
			css = compareCellOwnCSS
		}
		row.Cells = append(row.Cells, compareCellView{Values: values, Own: own, CSS: css})
	}
	return row
}

// compareCategories is setID's rows, dispatched on the same set-id keys as
// the pick sheets. An id of no known kind has no rows.
func compareCategories(st *store.Store, setID string) []compareCategory {
	switch {
	case singleTeamSetLabels[setID] != "":
		return []compareCategory{singleTeamCategory(st, setID)}
	case setID == divisionsSetID:
		return divisionCategories(st)
	case setID == awardsSetID:
		return awardCategories(st)
	case seriesSetIDs[setID]:
		return seriesCategories(st, setID)
	default:
		return nil
	}
}

// singleTeamCategory shows each player's pick for kind by full team name,
// plain (never tagged - Boundaries: full team names stay plain text).
func singleTeamCategory(st *store.Store, kind string) compareCategory {
	teams := teamRoster(st)
	return compareCategory{
		label: singleTeamSetLabels[kind],
		values: func(playerID string) []compareValueView {
			p, ok := st.FindPrediction(playerID, kind)
			if !ok || p.TeamID == "" {
				return nil
			}
			return []compareValueView{plainValue(teamName(teams, p.TeamID))}
		},
	}
}

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

// awardCategories is one stacked row per award, in awardOrder, with each
// finalist's display name, plain.
func awardCategories(st *store.Store) []compareCategory {
	categories := make([]compareCategory, 0, len(awardOrder))
	for _, award := range awardOrder {
		categories = append(categories, compareCategory{
			label:   awardTitle[award],
			stacked: true,
			values: func(playerID string) []compareValueView {
				p, _ := st.FindAwardFinalists(playerID, award)
				values := make([]compareValueView, 0, len(p.FinalistSlugs))
				for _, slug := range p.FinalistSlugs {
					values = append(values, plainValue(displayNameForSlug(st, slug)))
				}
				return values
			},
		})
	}
	return categories
}

// seriesCategories is one row per recorded matchup of setID, showing each
// player's winner tagged plus a plain " in <games>" suffix (Boundaries:
// series rows render the winner as a tag plus plain text, not one opaque
// string).
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
				return []compareValueView{tagValue(p.TeamID), plainValue(fmt.Sprintf(compareSeriesGamesSuffix, p.Games))}
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
