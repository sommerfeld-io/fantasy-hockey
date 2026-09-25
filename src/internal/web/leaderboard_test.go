package web

import (
	"go/parser"
	"go/token"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// leaderboardSeedBase declares three players (in non-alphabetical file
// order), the two teams every pick uses, and the recorded results: FLA won
// the Cup and TOR the Presidents' Trophy, so a correct cup or presidents
// pick scores 20 Regular and a correct playoffcup pick 20 Playoff.
const leaderboardSeedBase = `season: "2026-27"
players:
    - {id: tobbi, name: Tobbi, email: tobbi@example.com}
    - {id: sadl, name: Sadl, email: sadl@example.com}
    - {id: basti, name: Basti, email: basti@example.com}
teams:
    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}
    - {id: TOR, name: Toronto Maple Leafs, conference: Eastern, division: Atlantic}
results:
    presidents_trophy: TOR
    stanley_cup_winner: FLA
predictions:
`

// leaderboardPick renders one saved single-team pick.
func leaderboardPick(playerID, kind, teamID string) string {
	return "    - {player_id: " + playerID + ", submitted_at: \"2026-09-20T10:00:00Z\", kind: " + kind + ", team_id: " + teamID + "}\n"
}

// newTestStoreWithLeaderboard opens a store on leaderboardSeedBase plus the
// given picks.
func newTestStoreWithLeaderboard(t *testing.T, picks ...string) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), store.DataFileName)
	if err := os.WriteFile(path, []byte(leaderboardSeedBase+strings.Join(picks, "")), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return st
}

// getLeaderboard renders /leaderboard for playerID and returns the body.
func getLeaderboard(t *testing.T, st *store.Store, playerID string) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/leaderboard", nil)
	req.AddCookie(auth.IssueSessionCookie(playerID, testSecret))
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	return rec.Body.String()
}

// leaderboardRowFragment isolates playerID's rendered row.
func leaderboardRowFragment(t *testing.T, body, playerID string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<tr id="leaderboard-row-` + regexp.QuoteMeta(playerID) + `" class="lb-row">.*?</tr>`).FindString(body)
	if m == "" {
		t.Fatalf("expected a Leaderboard row for %q, got %q", playerID, body)
	}
	return m
}

// clearLeaderPicks gives Basti 40/20, Sadl 40/0 and Tobbi 20/0.
var clearLeaderPicks = []string{
	leaderboardPick("basti", store.KindCupChampion, "FLA"),
	leaderboardPick("basti", store.KindPresidentsTrophy, "TOR"),
	leaderboardPick("basti", store.KindPlayoffsCup, "FLA"),
	leaderboardPick("sadl", store.KindCupChampion, "FLA"),
	leaderboardPick("sadl", store.KindPresidentsTrophy, "TOR"),
	leaderboardPick("tobbi", store.KindCupChampion, "FLA"),
}

func TestLeaderboardShouldRenderEveryRowInRankOrderWithItsPoints(t *testing.T) {
	body := getLeaderboard(t, newTestStoreWithLeaderboard(t, clearLeaderPicks...), "basti")

	assertMarkersInOrder(t, body, `id="leaderboard-row-basti"`, `id="leaderboard-row-sadl"`, `id="leaderboard-row-tobbi"`)
	for _, tt := range []struct {
		id, rank, name, regular, playoff, total string
	}{
		{"basti", "1", "Basti", "40", "20", "60"},
		{"sadl", "2", "Sadl", "40", "0", "40"},
		{"tobbi", "3", "Tobbi", "20", "0", "20"},
	} {
		fragment := leaderboardRowFragment(t, body, tt.id)
		assertMarkersInOrder(t, fragment,
			`">`+tt.rank+`</span>`, `<span class="lb-name">`+tt.name+`</span>`, `<td class="lb-num">`+tt.regular+`</td>`)
		wantCells := regexp.MustCompile(`<td class="lb-num">` + tt.regular + `</td>\s*<td class="lb-num">` + tt.playoff + `</td>\s*<td class="lb-total[^"]*">` + tt.total + `</td>`)
		if !wantCells.MatchString(fragment) {
			t.Errorf("expected %s's row to read %s / %s / %s, got %q", tt.name, tt.regular, tt.playoff, tt.total, fragment)
		}
	}
}

func TestLeaderboardShouldRenderTheTableHeadingsInColumnOrder(t *testing.T) {
	body := getLeaderboard(t, newTestStoreWithLeaderboard(t), "basti")

	assertMarkersInOrder(t, body, `<th`, `>Player</th>`, `>Regular</th>`, `>Playoff</th>`, `>Total</th>`)
}

func TestLeaderboardShouldMarkOnlyTheLeaderInGold(t *testing.T) {
	body := getLeaderboard(t, newTestStoreWithLeaderboard(t, clearLeaderPicks...), "basti")

	leader := leaderboardRowFragment(t, body, "basti")
	if !strings.Contains(leader, `class="rank-badge rank-badge--gold"`) || !strings.Contains(leader, `class="lb-total lb-total--gold"`) {
		t.Errorf("expected the leader's badge and Total in gold, got %q", leader)
	}
	for _, id := range []string{"sadl", "tobbi"} {
		fragment := leaderboardRowFragment(t, body, id)
		if strings.Contains(fragment, "gold") {
			t.Errorf("expected %s's row to carry no gold, got %q", id, fragment)
		}
	}
	if n := strings.Count(body, "--gold"); n != 2 {
		t.Errorf("expected gold only on the leader's badge and Total (2 uses), got %d", n)
	}
}

func TestLeaderboardShouldMarkEveryPlayerTiedAtTheTopInGold(t *testing.T) {
	st := newTestStoreWithLeaderboard(t,
		leaderboardPick("basti", store.KindCupChampion, "FLA"),
		leaderboardPick("sadl", store.KindPlayoffsCup, "FLA"),
	)

	body := getLeaderboard(t, st, "basti")

	for _, id := range []string{"basti", "sadl"} {
		fragment := leaderboardRowFragment(t, body, id)
		if !strings.Contains(fragment, `rank-badge--gold">1</span>`) || !strings.Contains(fragment, `lb-total--gold">20</td>`) {
			t.Errorf("expected %s to be a gold rank-1 leader, got %q", id, fragment)
		}
	}
	tobbi := leaderboardRowFragment(t, body, "tobbi")
	if !strings.Contains(tobbi, `<span class="rank-badge">3</span>`) || strings.Contains(tobbi, "gold") {
		t.Errorf("expected Tobbi at a neutral rank 3, got %q", tobbi)
	}
}

func TestLeaderboardShouldNotMarkAnyoneInGoldBeforeAnythingScores(t *testing.T) {
	body := getLeaderboard(t, newTestStoreWithLeaderboard(t, leaderboardPick("basti", store.KindCupChampion, "TOR")), "basti")

	if strings.Contains(body, "--gold") {
		t.Errorf("expected no gold before anything scores, got %q", body)
	}
	if n := strings.Count(body, `<span class="rank-badge">1</span>`); n != 3 {
		t.Errorf("expected all 3 players at a neutral rank 1, got %d", n)
	}
}

func TestLeaderboardShouldNotMarkTheSignedInPlayersOwnRow(t *testing.T) {
	st := newTestStoreWithLeaderboard(t,
		leaderboardPick("sadl", store.KindCupChampion, "FLA"),
		leaderboardPick("tobbi", store.KindCupChampion, "FLA"),
	)

	body := getLeaderboard(t, st, "sadl")

	if strings.Contains(body, "(you)") {
		t.Errorf("expected no \"(you)\" marker, got %q", body)
	}
	own := anonymizedRow(t, body, "sadl", "Sadl")
	other := anonymizedRow(t, body, "tobbi", "Tobbi")
	if own != other {
		t.Errorf("expected the signed-in player's row to be marked up like an equal-scoring other row, got %q vs %q", own, other)
	}
}

// anonymizedRow returns playerID's row with its id and name replaced, so
// two equal-scoring rows can be compared for identical markup.
func anonymizedRow(t *testing.T, body, playerID, name string) string {
	t.Helper()
	fragment := strings.ReplaceAll(leaderboardRowFragment(t, body, playerID), playerID, "ID")
	return strings.ReplaceAll(fragment, name, "NAME")
}

func TestLeaderboardShouldShowItsHeadingCaptionAndFooterInsteadOfThePlaceholder(t *testing.T) {
	body := getLeaderboard(t, newTestStoreWithLeaderboard(t), "basti")

	assertMarkersInOrder(t, body,
		"<span>Leaderboard</span>",
		"Ranked by total points.",
		"<table",
		"Regular-season picks feed the Regular column, playoff-round picks feed Playoff, and Total is what decides the standings.")
	if strings.Contains(body, "coming soon") || strings.Contains(body, `class="coming-soon"`) {
		t.Errorf("expected the coming-soon placeholder to be gone, got %q", body)
	}
}

func TestLeaderboardShouldNotRenderOnAnyOtherTab(t *testing.T) {
	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStoreWithLeaderboard(t, clearLeaderPicks...), noopSender, testSecret).ServeHTTP(rec, req)

	if body := rec.Body.String(); strings.Contains(body, "leaderboard-row-") || strings.Contains(body, "Ranked by total points.") {
		t.Errorf("expected Predict to render no Leaderboard content, got %q", body)
	}
}

func TestWebShouldNotImportScoring(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list package files: %v", err)
	}
	const scoringPath = "github.com/sommerfeld-io/fantasy-hockey/internal/scoring"

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, imp := range parsed.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquote import %s in %s: %v", imp.Path.Value, file, err)
			}
			if path == scoringPath {
				t.Errorf("%s imports internal/scoring; web may read points only through standings.Row", file)
			}
		}
	}
}
