package web

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// Bottom-nav tab identifiers, matching shell.html's ActiveTab comparisons
// and this file's tabTitles lookup below.
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

// compareSetQueryParam is the /compare query parameter naming the selected
// Prediction Set, set by each selector chip's link.
const compareSetQueryParam = "set"

// shellData feeds templates/shell.html. PlayerName is empty when the
// session's player id has no matching player left in the store (a
// stale/deleted id) - the header then degrades to a neutral state instead
// of a 500 or panic. Season is already presentation-formatted (e.g. "NHL
// 2026–27"); ActiveTab selects which bottom-nav tab renders active; Title is
// every tab's page title. Predict, Leaderboard and Compare are nil except on
// their own tab, where they hold that tab's content.
type shellData struct {
	PlayerName  string
	Season      string
	ActiveTab   string
	Title       string
	Predict     *predictPhases
	Leaderboard *leaderboardView
	Compare     *compareView
}

// formatSeason turns the store's raw season value (e.g. "2026-27") into the
// header's display form ("NHL 2026–27") - presentation-only, matching how
// the login mockups already hardcode this same display string.
func formatSeason(season string) string {
	return "NHL " + strings.Replace(season, "-", "–", 1)
}

// handleShell renders the persistent app shell for tab: a pinned header
// (player name + season + logout control), a pinned bottom nav with tab
// active, and that tab's content - Predict's real, phase-grouped Prediction
// Set lists, the Leaderboard's ranked standings (recomputed on every
// request), or Compare's side-by-side picks for the set named by the "set"
// query parameter (also recomputed on every request).
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
		}
		if tab == tabPredict {
			phases := buildPredictPhases(st, playerID, clock.NowTime())
			data.Predict = &phases
		}
		if tab == tabLeaderboard {
			board := buildLeaderboard(st)
			data.Leaderboard = &board
		}
		if tab == tabCompare {
			compare := buildCompare(st, playerID, r.URL.Query().Get(compareSetQueryParam), clock.NowTime())
			data.Compare = &compare
		}

		renderTemplate(w, "shell.html", data)
	}
}
