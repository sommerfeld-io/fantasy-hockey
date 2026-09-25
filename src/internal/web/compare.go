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
	compareSeriesValue          = "%s in %s"
)

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

// compareCellView is one player's value(s) for one category. Values is
// never empty: an unfilled value is emptyCellValue.
type compareCellView struct {
	Values []string
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
// present, and the selected set's table (nil when no set is selectable).
type compareView struct {
	Groups []compareGroupView
	Table  *compareTableView
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
	values  func(playerID string) []string
}

// buildCompare builds the Compare tab for playerID (whose column, if any,
// is marked own) with selectedID's set shown, or the default set when
// selectedID isn't selectable. It reads the store on every call and never
// writes to it.
func buildCompare(st *store.Store, playerID, selectedID string, now time.Time) compareView {
	sets := selectableCompareSets(st)
	selected, ok := findCompareSet(sets, selectedID)
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
			values = []string{emptyCellValue}
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

// singleTeamCategory shows each player's pick for kind by full team name.
func singleTeamCategory(st *store.Store, kind string) compareCategory {
	teams := teamRoster(st)
	return compareCategory{
		label: singleTeamSetLabels[kind],
		values: func(playerID string) []string {
			p, ok := st.FindPrediction(playerID, kind)
			if !ok || p.TeamID == "" {
				return nil
			}
			return []string{teamName(teams, p.TeamID)}
		},
	}
}

// divisionCategories is a playoff-teams row then a winner row per division,
// in store.Divisions() order, with team abbreviations as values.
func divisionCategories(st *store.Store) []compareCategory {
	var categories []compareCategory
	for _, division := range store.Divisions() {
		categories = append(categories,
			compareCategory{
				label: fmt.Sprintf(compareDivisionPlayoffLabel, division),
				values: func(playerID string) []string {
					p, _ := st.FindDivisionPlayoffTeams(playerID, division)
					return p.TeamIDs
				},
			},
			compareCategory{
				label: fmt.Sprintf(compareDivisionWinnerLabel, division),
				values: func(playerID string) []string {
					p, ok := st.FindDivisionWinner(playerID, division)
					if !ok || p.TeamID == "" {
						return nil
					}
					return []string{p.TeamID}
				},
			},
		)
	}
	return categories
}

// awardCategories is one stacked row per award, in awardOrder, with each
// finalist's display name.
func awardCategories(st *store.Store) []compareCategory {
	categories := make([]compareCategory, 0, len(awardOrder))
	for _, award := range awardOrder {
		categories = append(categories, compareCategory{
			label:   awardTitle[award],
			stacked: true,
			values: func(playerID string) []string {
				p, _ := st.FindAwardFinalists(playerID, award)
				names := make([]string, 0, len(p.FinalistSlugs))
				for _, slug := range p.FinalistSlugs {
					names = append(names, displayNameForSlug(st, slug))
				}
				return names
			},
		})
	}
	return categories
}

// seriesCategories is one row per recorded matchup of setID, showing each
// player's "<winner> in <games>".
func seriesCategories(st *store.Store, setID string) []compareCategory {
	matchups := st.PlayoffMatchups(setID)
	categories := make([]compareCategory, 0, len(matchups))
	for _, m := range matchups {
		seriesKey := store.JoinSeriesKey(setID, m.Key)
		categories = append(categories, compareCategory{
			label: seriesLabel(st, setID, m),
			values: func(playerID string) []string {
				p, ok := st.FindSeriesPick(playerID, seriesKey)
				if !ok || p.TeamID == "" {
					return nil
				}
				return []string{fmt.Sprintf(compareSeriesValue, p.TeamID, p.Games)}
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
