package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// awardOrder is the fixed display order of the 5 individual awards
// (PRD FR-17/epics.md Story 2.6's own AC) - never hand-maintained data.
var awardOrder = []string{
	store.AwardHart,
	store.AwardNorris,
	store.AwardVezina,
	store.AwardArtRoss,
	store.AwardRocketRichard,
}

// awardTitle is each award's trophy-group label, matching the click-dummy
// App.jsx's own exact FieldGroup title wording verbatim (Design Notes:
// click-dummy UI text is authoritative) - "Trophy" appears for Hart/Norris/
// Vezina but deliberately not for Art Ross/Rocket Richard, preserved as-is
// rather than normalized.
var awardTitle = map[string]string{
	store.AwardHart:          "Hart Trophy finalists",
	store.AwardNorris:        "Norris Trophy finalists",
	store.AwardVezina:        "Vezina Trophy finalists",
	store.AwardArtRoss:       "Art Ross finalists",
	store.AwardRocketRichard: "Rocket Richard finalists",
}

// awardPlaceholder is each award's finalist text-input placeholder,
// matching the click-dummy's own per-position wording.
var awardPlaceholder = map[string]string{
	store.AwardHart:          "Player name",
	store.AwardNorris:        "Defenseman name",
	store.AwardVezina:        "Goalie name",
	store.AwardArtRoss:       "Player name",
	store.AwardRocketRichard: "Player name",
}

// awardEligiblePosition maps each award to the NHL Player position its 3
// finalists must belong to (Hart/Art Ross/Rocket Richard need skaters,
// Norris needs defensemen, Vezina needs goalies) - fixed by the PRD/
// epics.md AC directly, so it lives in code as a plain lookup, never
// hand-maintained data (Boundaries & Constraints).
var awardEligiblePosition = map[string]string{
	store.AwardHart:          store.PositionSkater,
	store.AwardNorris:        store.PositionDefenseman,
	store.AwardVezina:        store.PositionGoalie,
	store.AwardArtRoss:       store.PositionSkater,
	store.AwardRocketRichard: store.PositionSkater,
}

// awardFinalistCount aliases store.AwardFinalistCount under this package's
// own established naming, so call sites here don't need every reference
// rewritten to the longer store.AwardFinalistCount spelling.
const awardFinalistCount = store.AwardFinalistCount

// awardFinalistTextFieldName/awardFinalistSlugFieldName name slot i's
// (0-based) visible text / paired hidden slug form fields for award -
// divisionPlayoffTeamsFieldName/divisionWinnerFieldName's own per-field
// naming precedent.
func awardFinalistTextFieldName(award string, slot int) string {
	return fmt.Sprintf("award_%s_text_%d", award, slot)
}

func awardFinalistSlugFieldName(award string, slot int) string {
	return fmt.Sprintf("award_%s_slug_%d", award, slot)
}

// invalidFinalistErrorText is the inline caption shown on any finalist slot
// whose non-blank text failed to resolve to a slug valid for that award's
// own eligible position (AD-10's server-revalidates stance) - an
// unresolved name and a resolved-but-wrong-position slug both render this
// identical caption (I/O matrix), and nothing is saved for the whole
// submission.
const invalidFinalistErrorText = "Pick a name from the suggestions."

// finalistSlotPick is one award-finalist slot's rendered state: Text/Slug
// are the visible text input's and paired hidden slug input's values;
// TextField/SlugField are their submitted form field names. Error is
// invalidFinalistErrorText once Invalid is true (a rejected resubmission's
// own offending slot), empty otherwise.
type finalistSlotPick struct {
	Text      string
	Slug      string
	TextField string
	SlugField string
	AriaLabel string
	Invalid   bool
	Error     string
}

// awardGroupPick is one award's rendered trophy group: Finalists holds its
// 3 slots in awardFinalistTextFieldName/awardFinalistSlugFieldName's own
// slot order. Filled is true once every slot resolves to a slug valid for
// Position - it renders a green check independent of the whole set's
// Submitted state (Boundaries & Constraints).
type awardGroupPick struct {
	Award       string
	Title       string
	Placeholder string
	Position    string
	Finalists   [awardFinalistCount]finalistSlotPick
	Filled      bool
}

// awardsPickView is the "awards" sheet's whole pick-entry state: Groups
// holds all 5 trophy groups in awardOrder; Submitted mirrors
// divisionPickView's own field (setSubmitted's "true if any award has a
// saved row" convention). SkaterOptionsJSON/DefensemanOptionsJSON/
// GoalieOptionsJSON are the 3 position-scoped embed lists Story 2.6's
// autocomplete widget reads, one per position, reused across every slot
// needing that position (Boundaries & Constraints: "one script tag per
// position").
type awardsPickView struct {
	Groups                []awardGroupPick
	Submitted             bool
	SkaterOptionsJSON     template.JS
	DefensemanOptionsJSON template.JS
	GoalieOptionsJSON     template.JS
}

// isValidAwardFinalist reports whether slug names a real NHL Player whose
// own Position matches award's fixed awardEligiblePosition - the
// server-side re-validation AD-10 requires regardless of what the client
// allowed or what newAwardFinalistOptions' own embed happened to offer.
func isValidAwardFinalist(st *store.Store, award, slug string) bool {
	finalist, ok := playerRoster(st).get(slug)
	return ok && finalist.Position == awardEligiblePosition[award]
}

// displayNameForSlug resolves slug to its NHL Player's own DisplayName, for
// re-rendering a saved pick's visible text input. An unmatched slug (e.g. a
// hand-edit removing a player after it was picked) degrades to rendering
// the raw slug as its own display text rather than a blank input.
func displayNameForSlug(st *store.Store, slug string) string {
	if finalist, ok := playerRoster(st).get(slug); ok {
		return finalist.DisplayName
	}
	return slug
}

// awardSlotSubmission is one submitted finalist slot's raw text/slug pair,
// as parsed off the form (parseAwardsSubmission) or retained from a
// rejected resubmission (renderRejectedAwardsPick).
type awardSlotSubmission struct {
	Text string
	Slug string
}

// awardSlotInvalid reports whether slots[i] is invalid: its non-blank Text
// failed to resolve to a slug valid for award (isValidAwardFinalist) - the
// same "typed-but-unresolved" condition regardless of whether the slug was
// empty or resolved to the wrong position - or its resolved slug duplicates
// another slot's slug within the same award's own 3 slots ("pick 3
// finalists" means 3 distinct players, independently re-validated
// server-side per AD-10 regardless of what the client allowed). A blank
// Text with no slug is never invalid - a deliberately empty slot is never
// an error (FR-11).
func awardSlotInvalid(st *store.Store, award string, slots [awardFinalistCount]awardSlotSubmission, i int) bool {
	slot := slots[i]
	if slot.Text != "" && !isValidAwardFinalist(st, award, slot.Slug) {
		return true
	}
	return slot.Slug != "" && awardSlotIsDuplicateSlug(slots, i)
}

// awardSlotIsDuplicateSlug reports whether slots[i]'s own non-empty Slug
// also appears in another one of slots' own entries.
func awardSlotIsDuplicateSlug(slots [awardFinalistCount]awardSlotSubmission, i int) bool {
	slug := slots[i].Slug
	if slug == "" {
		return false
	}
	for j, other := range slots {
		if j != i && other.Slug == slug {
			return true
		}
	}
	return false
}

// awardFinalistSlots resolves award's currently-rendered 3 slots: override's
// own retained (invalid) submission when override is non-nil, otherwise
// playerID's saved FindAwardFinalists row (blank slots when none is saved) -
// selectedTeamsForDivision/winnerForDivision's own override-vs-saved
// precedent.
func awardFinalistSlots(st *store.Store, playerID, award string, override map[string][awardFinalistCount]awardSlotSubmission) [awardFinalistCount]awardSlotSubmission {
	if override != nil {
		return override[award]
	}

	var slots [awardFinalistCount]awardSlotSubmission
	prediction, ok := st.FindAwardFinalists(playerID, award)
	if !ok {
		return slots
	}
	for i := 0; i < awardFinalistCount && i < len(prediction.FinalistSlugs); i++ {
		slug := prediction.FinalistSlugs[i]
		slots[i] = awardSlotSubmission{Text: displayNameForSlug(st, slug), Slug: slug}
	}
	return slots
}

// newAwardGroupPick builds one award's rendered trophy group - factored out
// of newAwardsPickView purely to keep its own cyclomatic complexity low
// (gocyclo), mirroring newDivisionGroupPick's own precedent. Invalid/Error
// are only ever computed from awardSlotInvalid when override is non-nil
// (a rejected resubmission's own re-render) - never for a plain saved-state
// render, so a previously-saved, valid slug that later stops resolving
// (e.g. a hand-edited nhl_players: roster) degrades to simply losing its
// green check (Filled below) rather than incorrectly showing the
// rejection-only goal-border/caption just from opening the page.
func newAwardGroupPick(st *store.Store, playerID, award string, override map[string][awardFinalistCount]awardSlotSubmission) awardGroupPick {
	var finalists [awardFinalistCount]finalistSlotPick
	filled := true
	rejected := override != nil

	slots := awardFinalistSlots(st, playerID, award, override)
	for i, slot := range slots {
		invalid := rejected && awardSlotInvalid(st, award, slots, i)
		errText := ""
		if invalid {
			errText = invalidFinalistErrorText
		}
		finalists[i] = finalistSlotPick{
			Text:      slot.Text,
			Slug:      slot.Slug,
			TextField: awardFinalistTextFieldName(award, i),
			SlugField: awardFinalistSlugFieldName(award, i),
			AriaLabel: fmt.Sprintf("%s, slot %d", awardTitle[award], i+1),
			Invalid:   invalid,
			Error:     errText,
		}
		if !isValidAwardFinalist(st, award, slot.Slug) || awardSlotIsDuplicateSlug(slots, i) {
			filled = false
		}
	}

	return awardGroupPick{
		Award:       award,
		Title:       awardTitle[award],
		Placeholder: awardPlaceholder[award],
		Position:    awardEligiblePosition[award],
		Finalists:   finalists,
		Filled:      filled,
	}
}

// marshalAwardOptions marshals newAwardFinalistOptions(st.NHLPlayersByPosition(position))
// to JSON for direct embedding inside a <script type="application/json">
// tag. encoding/json's default HTML-escaping (SetEscapeHTML's on-by-default
// behavior) keeps this safe to embed even though display names are
// hand-maintained, free-form text. A marshal failure - which
// newAwardFinalistOptions' own {string,string} embedOption shape can never
// actually produce - embeds an empty array rather than panicking.
func marshalAwardOptions(st *store.Store, position string) template.JS {
	out, err := json.Marshal(newAwardFinalistOptions(st.NHLPlayersByPosition(position)))
	if err != nil {
		slog.Error("marshal award options", "position", position, "error", err)
		return template.JS("[]")
	}
	return template.JS(out)
}

// newAwardsPickView builds sheetData's AwardsPick field for playerID: every
// award group from awardOrder, each group's 3 finalist slots reflecting
// either playerID's saved picks or - when override is non-nil - a rejected
// resubmission's own (invalid) values retained for re-rendering
// (renderRejectedAwardsPick), plus the 3 position-scoped embed lists the
// autocomplete widget reads.
func newAwardsPickView(st *store.Store, playerID string, submitted bool, override map[string][awardFinalistCount]awardSlotSubmission) awardsPickView {
	groups := make([]awardGroupPick, len(awardOrder))
	for i, award := range awardOrder {
		groups[i] = newAwardGroupPick(st, playerID, award, override)
	}

	return awardsPickView{
		Groups:                groups,
		Submitted:             submitted,
		SkaterOptionsJSON:     marshalAwardOptions(st, store.PositionSkater),
		DefensemanOptionsJSON: marshalAwardOptions(st, store.PositionDefenseman),
		GoalieOptionsJSON:     marshalAwardOptions(st, store.PositionGoalie),
	}
}

// parseAwardsSubmission reads every award's 3 finalist slots off the
// submitted form, keyed by award, for every entry in awardOrder. r.Form
// must already be populated (r.ParseForm) - parseDivisionsSubmission's own
// per-set-id precedent.
func parseAwardsSubmission(r *http.Request) map[string][awardFinalistCount]awardSlotSubmission {
	submission := make(map[string][awardFinalistCount]awardSlotSubmission, len(awardOrder))
	for _, award := range awardOrder {
		var slots [awardFinalistCount]awardSlotSubmission
		for i := 0; i < awardFinalistCount; i++ {
			slots[i] = awardSlotSubmission{
				Text: strings.TrimSpace(r.FormValue(awardFinalistTextFieldName(award, i))),
				Slug: strings.TrimSpace(r.FormValue(awardFinalistSlugFieldName(award, i))),
			}
		}
		submission[award] = slots
	}
	return submission
}

// awardsSubmissionHasAnInvalidSlot reports whether any slot across the
// whole submission is invalid (awardSlotInvalid, which also catches the
// same slug repeated across one award's own 3 slots) - the whole-form gate
// (Boundaries & Constraints: any single typed-but-unresolved, wrong-
// position, or duplicate slug blocks the entire POST, saving nothing).
func awardsSubmissionHasAnInvalidSlot(st *store.Store, submission map[string][awardFinalistCount]awardSlotSubmission) bool {
	for award, slots := range submission {
		for i := range slots {
			if awardSlotInvalid(st, award, slots, i) {
				return true
			}
		}
	}
	return false
}

// awardFinalistSlugsToSave returns the subset of submission whose award has
// all awardFinalistCount slots resolved to a slug valid for that award
// (isValidAwardFinalist) - an award left partially or fully blank is simply
// absent from the result (FR-11: scores zero, never blocks another award).
// Callers must already have passed awardsSubmissionHasAnInvalidSlot's gate,
// so every non-blank slot here is already known valid; this only counts
// how many of the 3 actually resolved.
func awardFinalistSlugsToSave(st *store.Store, submission map[string][awardFinalistCount]awardSlotSubmission) map[string][]string {
	toSave := make(map[string][]string, len(submission))
	for award, slots := range submission {
		slugs := make([]string, 0, awardFinalistCount)
		for _, slot := range slots {
			if !isValidAwardFinalist(st, award, slot.Slug) {
				continue
			}
			slugs = append(slugs, slot.Slug)
		}
		if len(slugs) == awardFinalistCount {
			toSave[award] = slugs
		}
	}
	return toSave
}

// handleAwardsSubmit handles awardsSetID's branch of POST /predict/{id}:
// parses all 15 finalist slots (parseAwardsSubmission), then validates
// before anything is saved (Boundaries & Constraints: "Submit is
// all-or-nothing for the whole form"). Any slot whose non-blank text failed
// to resolve to a slug valid for its own award's eligible position
// re-renders the sheet (200) with invalidFinalistErrorText on every
// offending slot and saves nothing, including otherwise-complete awards. A
// valid submission saves every award whose 3 slots are all filled via one
// st.SaveAwardPicks call - a partially or fully blank award simply isn't
// saved this submission (FR-11) - and redirects to /predict (302).
func handleAwardsSubmit(w http.ResponseWriter, r *http.Request, st *store.Store, set store.PredictionSet, playerID string, now time.Time) {
	submission := parseAwardsSubmission(r)

	if awardsSubmissionHasAnInvalidSlot(st, submission) {
		renderRejectedAwardsPick(w, r, st, set, playerID, now, submission)
		return
	}

	if err := st.SaveAwardPicks(playerID, awardFinalistSlugsToSave(st, submission), now); err != nil {
		slog.Error("save award picks", "player_id", playerID, "error", err)
		http.Error(w, genericErrorBody, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/predict", http.StatusFound)
}

// renderRejectedAwardsPick re-renders the awardsSetID sheet (200) with
// submission's own typed text/slug values retained per slot and every
// offending slot marked Invalid (goal-border + inline caption) - nothing is
// saved. err from newSheetData can only come from set.DeadlineUTC failing
// to parse, already parsed successfully by handleSheetSubmit moments
// earlier.
func renderRejectedAwardsPick(w http.ResponseWriter, r *http.Request, st *store.Store, set store.PredictionSet, playerID string, now time.Time, submission map[string][awardFinalistCount]awardSlotSubmission) {
	data, err := newSheetData(st, set, playerID, now, nil)
	if err != nil {
		slog.Error("parse deadline_utc for prediction set", "prediction_set_id", set.ID, "error", err)
		http.NotFound(w, r)
		return
	}

	view := newAwardsPickView(st, playerID, data.AwardsPick.Submitted, submission)
	data.AwardsPick = &view

	renderTemplate(w, "sheet.html", data)
}
