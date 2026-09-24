package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
)

func TestNewServerShouldReturnOKForTheHomePage(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

// shellRouteTests is every shell route this server registers, alongside the
// bottom-nav tab that should render active and the unique "Coming soon"
// fragment that route's content should show. "/" aliases to Predict.
var shellRouteTests = []struct {
	path       string
	activeTab  string
	navLabel   string
	messageFor string
}{
	{"/", "predict", "Predict", "Before the season"},
	{"/predict", "predict", "Predict", "Before the season"},
	{"/leaderboard", "leaderboard", "Leaderboard", "The leaderboard is coming soon."},
	{"/compare", "compare", "Compare", "Player comparison is coming soon."},
}

func TestNewServerShouldRenderTheShellForEveryDestination(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			if rec.Code != 200 {
				t.Fatalf("expected status 200, got %d", rec.Code)
			}
			body := rec.Body.String()
			assertShellHeader(t, body)
			if !strings.Contains(body, tt.messageFor) {
				t.Errorf("expected the %q content, got %q", tt.messageFor, body)
			}
			assertActiveTab(t, body, tt.navLabel)
			assertInactiveTabs(t, body, tt.activeTab)
			assertLogoutControl(t, body)
		})
	}
}

// assertShellHeader checks the header shows the logged-in player's name and
// the formatted season label.
func assertShellHeader(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, "Basti") {
		t.Errorf("expected the header to show the player's name, got %q", body)
	}
	if !strings.Contains(body, "NHL 2026–27") {
		t.Errorf("expected the header to show the formatted season, got %q", body)
	}
}

// navHrefs maps each nav label to its route, so tests can tie a label to
// its own anchor's active/inactive state without depending on the icon
// markup rendered between the anchor's opening tag and its label.
var navHrefs = map[string]string{
	"Predict":     "/predict",
	"Leaderboard": "/leaderboard",
	"Compare":     "/compare",
}

// assertActiveTab checks navLabel's anchor renders as the active tab, in
// `ice` with aria-current="page" for assistive technology.
func assertActiveTab(t *testing.T, body, navLabel string) {
	t.Helper()
	activeOpenTag := `<a href="` + navHrefs[navLabel] + `" class="nav-item active" aria-current="page">`
	if !strings.Contains(body, activeOpenTag) {
		t.Errorf("expected %q's anchor to render active with aria-current=\"page\", got %q", navLabel, body)
	}
	if !strings.Contains(body, `<span class="nav-label">`+navLabel+`</span>`) {
		t.Errorf("expected %q to render as a nav label, got %q", navLabel, body)
	}
}

// assertInactiveTabs checks every tab other than activeTab renders as
// inactive (muted, no aria-current).
func assertInactiveTabs(t *testing.T, body, activeTab string) {
	t.Helper()
	for _, other := range shellRouteTests {
		if other.activeTab == activeTab {
			continue
		}
		inactiveOpenTag := `<a href="` + navHrefs[other.navLabel] + `" class="nav-item">`
		if !strings.Contains(body, inactiveOpenTag) {
			t.Errorf("expected %q's anchor to render inactive (muted, no aria-current), got %q", other.navLabel, body)
		}
	}
}

// assertLogoutControl checks the shell renders a visible logout button wired
// to POST /logout, so a regression that broke or removed it would be caught.
func assertLogoutControl(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `<form class="logout-form" method="post" action="/logout">`) {
		t.Errorf("expected the rendered shell to contain the logout form wired to POST /logout, got %q", body)
	}
	if !strings.Contains(body, `<button type="submit" class="logout-btn">Log out</button>`) {
		t.Errorf("expected the rendered shell to contain the visible logout button, got %q", body)
	}
}

func TestNewServerShouldRedirectToLoginForEveryShellDestinationWithNoSessionCookie(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			assertRedirectsToLoginWithNoCookie(t, rec)
		})
	}
}

// TestNewServerShouldRedirectToLoginForEveryShellDestinationWithAnIdleExpiredSession
// extends the idle-expired-session coverage (already proven for GET /{$} by
// TestGetHomeShouldRedirectToLoginForAnIdleExpiredSession) to the 3 new shell
// routes, matching the I/O & Edge-Case Matrix's "any shell route" scope.
func TestNewServerShouldRedirectToLoginForEveryShellDestinationWithAnIdleExpiredSession(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.AddCookie(auth.IssueSessionCookieAt("basti", time.Now().UTC().Add(-31*time.Minute), testSecret))
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			assertRedirectsToLoginWithNoCookie(t, rec)
		})
	}
}

// TestNewServerShouldRedirectToLoginForEveryShellDestinationWithATamperedSessionCookie
// extends the tampered-cookie coverage to the 3 new shell routes.
func TestNewServerShouldRedirectToLoginForEveryShellDestinationWithATamperedSessionCookie(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			c := auth.IssueSessionCookie("basti", testSecret)
			last := c.Value[len(c.Value)-1]
			replacement := byte('0')
			if last == replacement {
				replacement = '1'
			}
			c.Value = c.Value[:len(c.Value)-1] + string(replacement)

			req := httptest.NewRequest("GET", tt.path, nil)
			req.AddCookie(c)
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			assertRedirectsToLoginWithNoCookie(t, rec)
		})
	}
}

// TestNewServerShouldRedirectToLoginForEveryShellDestinationWithAnEmptyPlayerIDSession
// extends the empty-player-id coverage to the 3 new shell routes.
func TestNewServerShouldRedirectToLoginForEveryShellDestinationWithAnEmptyPlayerIDSession(t *testing.T) {
	for _, tt := range shellRouteTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.AddCookie(auth.IssueSessionCookieAt("", time.Now().UTC(), testSecret))
			rec := httptest.NewRecorder()

			NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

			assertRedirectsToLoginWithNoCookie(t, rec)
		})
	}
}

func TestNewServerShouldDegradeTheHeaderWithoutPanickingForAStalePlayerID(t *testing.T) {
	req := httptest.NewRequest("GET", "/predict", nil)
	req.AddCookie(auth.IssueSessionCookie("a-deleted-player-id", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200 (no crash) for a stale player id, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Basti") {
		t.Errorf("expected no real player name to leak into the degraded header, got %q", body)
	}
	if !strings.Contains(body, `<span class="header-name">&mdash;</span>`) {
		t.Errorf("expected the header to degrade to a neutral placeholder, got %q", body)
	}
}

func TestNewServerShouldReturn404ForAnUnknownPath(t *testing.T) {
	req := httptest.NewRequest("GET", "/unknown", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("expected status 404 for an unknown path, got %d", rec.Code)
	}
}
