package web

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

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

// loginCodeData feeds templates/login-code.html: Code is the submitted
// value (retained on error so the Player doesn't have to retype it), and
// Error is the generic message shown for any wrong/expired/used code, empty
// when there's nothing to report.
type loginCodeData struct {
	Code  string
	Error string
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
