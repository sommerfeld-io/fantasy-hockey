package web

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

func TestGetLoginShouldRenderTheEmailEntryScreen(t *testing.T) {
	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Email address") {
		t.Errorf("expected the email-entry screen to be rendered, got %q", rec.Body.String())
	}
}

func TestGetLoginShouldRedirectToTheShellForAnAlreadyAuthenticatedSession(t *testing.T) {
	req := httptest.NewRequest("GET", "/login", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/" {
		t.Errorf("expected a redirect to %q, got %q", "/", rec.Header().Get("Location"))
	}
}

func TestGetLoginShouldRenderTheEmailEntryScreenForAnIdleExpiredSession(t *testing.T) {
	req := httptest.NewRequest("GET", "/login", nil)
	req.AddCookie(auth.IssueSessionCookieAt("basti", time.Now().UTC().Add(-31*time.Minute), testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Email address") {
		t.Errorf("expected the email-entry screen to be rendered, got %q", rec.Body.String())
	}
}

func TestPostLoginShouldRenderTheCodeEntryScreenOnAMatch(t *testing.T) {
	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=basti%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "6-digit code") {
		t.Errorf("expected the code-entry screen to be rendered, got %q", rec.Body.String())
	}
}

func TestPostLoginShouldRenderAnIdenticalBodyRegardlessOfAMatch(t *testing.T) {
	matchReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=basti%40example.com"))
	matchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	matchRec := httptest.NewRecorder()
	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(matchRec, matchReq)

	noMatchReq := httptest.NewRequest("POST", "/login", strings.NewReader("email=unknown%40example.com"))
	noMatchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	noMatchRec := httptest.NewRecorder()
	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(noMatchRec, noMatchReq)

	if matchRec.Code != noMatchRec.Code {
		t.Fatalf("expected identical status codes, got %d and %d", matchRec.Code, noMatchRec.Code)
	}
	if matchRec.Body.String() != noMatchRec.Body.String() {
		t.Errorf("expected identical response bodies for match and no-match, got %q and %q",
			matchRec.Body.String(), noMatchRec.Body.String())
	}
}

func TestPostLoginShouldReturn500WhenTheStoreWriteFails(t *testing.T) {
	dir := t.TempDir()
	seed := `season: "2026-27"
players:
    - id: basti
      name: Basti
      email: basti@example.com
`
	path := filepath.Join(dir, store.DataFileName)
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	st, err := store.New(path)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	// Remove the directory out from under the store so the atomic
	// write-and-rename that CreateLoginCode performs fails, simulating a
	// disk write failure. Unlike a read-only permission bit, this fails
	// even when the test process runs as root (e.g. inside a container
	// build), which otherwise bypasses ordinary permission checks.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}

	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=basti%40example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("expected status 500 when the store write fails, got %d", rec.Code)
	}
}

func TestPostLoginShouldReturn500WhenTheRequestBodyIsMalformed(t *testing.T) {
	req := httptest.NewRequest("POST", "/login", strings.NewReader("email=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("expected status 500 for a malformed request body, got %d", rec.Code)
	}
}

// assertClearsSessionCookieAndRedirectsToLogin asserts rec is a 302 to
// /login carrying a Set-Cookie that instructs the browser to delete the
// session cookie immediately (empty value, negative Max-Age).
func assertClearsSessionCookieAndRedirectsToLogin(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/login" {
		t.Errorf("expected a redirect to %q, got %q", "/login", rec.Header().Get("Location"))
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie to be set, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != auth.SessionCookieName {
		t.Errorf("expected cookie name %q, got %q", auth.SessionCookieName, c.Name)
	}
	if c.Value != "" {
		t.Errorf("expected an empty cookie value, got %q", c.Value)
	}
	if c.MaxAge >= 0 {
		t.Errorf("expected a negative Max-Age so the browser deletes the cookie immediately, got %d", c.MaxAge)
	}

	// Assert the literal wire text too, not just the parsed MaxAge field:
	// Go's net/http renders any MaxAge<0 as "Max-Age=0" (RFC 6265's
	// immediate-deletion form), and this is the exact instruction that
	// actually reaches a browser or cookiejar - a regression here wouldn't
	// necessarily be caught by asserting the parsed struct field alone.
	if raw := rec.Header().Get("Set-Cookie"); !strings.Contains(raw, "Max-Age=0") {
		t.Errorf("expected the raw Set-Cookie header to contain %q, got %q", "Max-Age=0", raw)
	}
}

func TestPostLogoutShouldClearTheSessionCookieAndRedirectToLoginWithAValidSession(t *testing.T) {
	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertClearsSessionCookieAndRedirectsToLogin(t, rec)
}

func TestPostLogoutShouldClearTheSessionCookieAndRedirectToLoginWithNoSessionCookie(t *testing.T) {
	req := httptest.NewRequest("POST", "/logout", nil)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertClearsSessionCookieAndRedirectsToLogin(t, rec)
}

func TestPostLogoutShouldClearTheSessionCookieAndRedirectToLoginWithAnExpiredSessionCookie(t *testing.T) {
	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(auth.IssueSessionCookieAt("basti", time.Now().UTC().Add(-31*time.Minute), testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertClearsSessionCookieAndRedirectsToLogin(t, rec)
}

func TestPostLogoutShouldClearTheSessionCookieAndRedirectToLoginWithATamperedSessionCookie(t *testing.T) {
	c := auth.IssueSessionCookie("basti", testSecret)
	last := c.Value[len(c.Value)-1]
	replacement := byte('0')
	if last == replacement {
		replacement = '1'
	}
	c.Value = c.Value[:len(c.Value)-1] + string(replacement)

	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	assertClearsSessionCookieAndRedirectsToLogin(t, rec)
}

func TestGetLogoutShouldNotClearTheSessionOrRedirect(t *testing.T) {
	req := httptest.NewRequest("GET", "/logout", nil)
	req.AddCookie(auth.IssueSessionCookie("basti", testSecret))
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code == http.StatusFound {
		t.Fatalf("expected GET /logout not to redirect like POST /logout does, got status %d", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Errorf("expected GET /logout not to touch the session cookie, got %v", rec.Result().Cookies())
	}
}

// newTestStoreWithLoginCode seeds a store with one player and one persisted
// LoginCode row for code, issued issuedAt ago.
func newTestStoreWithLoginCode(t *testing.T, code string, issuedAt time.Time) *store.Store {
	t.Helper()
	st := newTestStore(t)
	sum := sha256.Sum256([]byte(code))
	hash := hex.EncodeToString(sum[:])
	if err := st.CreateLoginCode("basti", hash, issuedAt.Format(time.RFC3339)); err != nil {
		t.Fatalf("CreateLoginCode: %v", err)
	}
	return st
}

func postCode(t *testing.T, handler http.Handler, code string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/login/code", strings.NewReader("code="+code))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestPostLoginCodeShouldRedirectAndSetASessionCookieOnAValidCode(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	rec := postCode(t, NewServer(st, noopSender, testSecret), "123456")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
	}
	if rec.Header().Get("Location") != "/" {
		t.Errorf("expected a redirect to %q, got %q", "/", rec.Header().Get("Location"))
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie to be set, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "session" {
		t.Errorf("expected cookie name %q, got %q", "session", c.Name)
	}
	if !c.HttpOnly {
		t.Error("expected the session cookie to be HttpOnly")
	}
	if c.Secure {
		t.Error("expected the session cookie not to be Secure (AD-14 - plain HTTP today)")
	}
	if strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Error("expected no error text in the response on a successful login")
	}
	if strings.Contains(rec.Body.String(), `class="code-input error"`) {
		t.Error("expected no error class in the response on a successful login")
	}
}

func TestPostLoginCodeShouldMatchAWhitespacePaddedSubmittedCode(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	rec := postCode(t, NewServer(st, noopSender, testSecret), " 123456\n")

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status %d for a whitespace-padded code, got %d", http.StatusFound, rec.Code)
	}
	if len(rec.Result().Cookies()) != 1 {
		t.Error("expected a session cookie to be set for a whitespace-padded code")
	}
}

func TestPostLoginCodeShouldMarkTheCodeUsed(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())
	handler := NewServer(st, noopSender, testSecret)

	postCode(t, handler, "123456")
	rec := postCode(t, handler, "123456")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 on reuse, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Errorf("expected a reused code to show the generic error, got %q", rec.Body.String())
	}
}

func TestPostLoginCodeShouldShowTheGenericErrorAndRetainTheCodeOnAWrongCode(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	rec := postCode(t, NewServer(st, noopSender, testSecret), "000000")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Errorf("expected the generic error, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `value="000000"`) {
		t.Errorf("expected the submitted code to be retained in the input, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `class="code-input error"`) {
		t.Errorf("expected the code input to carry the error class, got %q", rec.Body.String())
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("expected no cookie to be set for a wrong code")
	}
}

func TestPostLoginCodeShouldShowTheGenericErrorWhenTheCodeFieldIsMissing(t *testing.T) {
	st := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())

	req := httptest.NewRequest("POST", "/login/code", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(st, noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for a missing code field, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), renderedGenericCodeErrorText) {
		t.Errorf("expected the generic error for a missing code field, got %q", rec.Body.String())
	}
}

func TestPostLoginCodeShouldShowTheIdenticalErrorForWrongExpiredAndUsedCodes(t *testing.T) {
	wrongStore := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())
	expiredStore := newTestStoreWithLoginCode(t, "123456", time.Now().UTC().Add(-11*time.Minute))
	usedStore := newTestStoreWithLoginCode(t, "123456", time.Now().UTC())
	usedHandler := NewServer(usedStore, noopSender, testSecret)
	postCode(t, usedHandler, "123456")

	wrongRec := postCode(t, NewServer(wrongStore, noopSender, testSecret), "000000")
	expiredRec := postCode(t, NewServer(expiredStore, noopSender, testSecret), "123456")
	usedRec := postCode(t, usedHandler, "123456")

	if wrongRec.Code != expiredRec.Code || expiredRec.Code != usedRec.Code {
		t.Fatalf("expected identical status codes, got wrong=%d expired=%d used=%d", wrongRec.Code, expiredRec.Code, usedRec.Code)
	}
	for name, body := range map[string]string{"wrong": wrongRec.Body.String(), "expired": expiredRec.Body.String(), "used": usedRec.Body.String()} {
		if !strings.Contains(body, renderedGenericCodeErrorText) {
			t.Errorf("expected the %s case to show the generic error, got %q", name, body)
		}
		if !strings.Contains(body, `class="code-input error"`) {
			t.Errorf("expected the %s case to carry the error class, got %q", name, body)
		}
	}
	if !strings.Contains(expiredRec.Body.String(), `value="123456"`) {
		t.Errorf("expected the expired case to retain the submitted code, got %q", expiredRec.Body.String())
	}
	if !strings.Contains(usedRec.Body.String(), `value="123456"`) {
		t.Errorf("expected the used case to retain the submitted code, got %q", usedRec.Body.String())
	}
}

func TestPostLoginCodeShouldReturn500WhenTheRequestBodyIsMalformed(t *testing.T) {
	req := httptest.NewRequest("POST", "/login/code", strings.NewReader("code=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	NewServer(newTestStore(t), noopSender, testSecret).ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Errorf("expected status 500 for a malformed request body, got %d", rec.Code)
	}
}
