// Package web is the presentation layer: it renders the Login page and
// handles login-code requests over stdlib net/http and html/template.
package web

import (
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*.css
var staticFS embed.FS

// confirmationMessage is shown after any login-code request, whether or not
// the submitted email matched a Participant. Never vary this string based on
// the outcome - doing so would leak which emails are registered (FR-1).
const confirmationMessage = "Check the entered email address."

// invalidCodeMessage is shown for a wrong, expired, or already-used code.
// Never differentiate the reason (mirrors FR-1's no-enumeration discipline).
const invalidCodeMessage = "Invalid code."

// sessionExpiredMessage is shown when a present session cookie's sliding
// timeout has elapsed - never shown when no cookie was present at all.
const sessionExpiredMessage = "Session expired."

// sessionCookieName is the name of the HMAC-signed session cookie.
const sessionCookieName = "session"

var loginTemplate = template.Must(template.ParseFS(templatesFS, "templates/login.html"))

var staticFiles = mustSubFS(staticFS, "static")

// mustSubFS narrows an embedded FS to a subdirectory. It panics on failure,
// which can only happen if the embed directive above is misconfigured - a
// programmer error caught immediately at package init, not at runtime.
func mustSubFS(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// loginPageData drives the login.html template's states: the initial email
// form, the code-entry step (with its confirmation message or an inline
// error), and the email form re-shown after a session times out.
type loginPageData struct {
	Step    string
	Message string
	Error   string
	Email   string
}

// server holds the dependencies the login handlers need.
type server struct {
	auth *auth.Service
}

// NewServer wires the Login page handlers against the given auth.Service and
// returns an http.Handler ready to be served.
func NewServer(a *auth.Service) http.Handler {
	s := &server{auth: a}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", s.handleLoginForm)
	mux.HandleFunc("POST /login", s.handleLoginSubmit)
	mux.HandleFunc("POST /login/code", s.handleCodeSubmit)
	mux.HandleFunc("GET /{$}", s.requireSession(s.handleHome))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFiles)))

	return mux
}

// handleLoginForm renders the empty email-entry form.
func (s *server) handleLoginForm(w http.ResponseWriter, _ *http.Request) {
	renderLogin(w, loginPageData{Step: "email"})
}

// handleLoginSubmit processes a login-code request. It always renders the
// same generic confirmation regardless of whether the submitted email
// matched a Participant - internal.auth.Service.RequestLoginCode already
// guarantees identical (nil) success behavior either way, and any
// unexpected infrastructure error is logged server-side rather than
// reflected back to the visitor, so the response never varies.
// maxLoginFormBytes bounds the size of a submitted login form body so a
// client can't tie up the handler by streaming an arbitrarily large request.
const maxLoginFormBytes = 4 << 10 // 4 KiB - generous for a one-field email form

func (s *server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))

	if err := s.auth.RequestLoginCode(r.Context(), email); err != nil {
		slog.Error("request login code", "error", err)
	}

	renderLogin(w, loginPageData{Step: "code", Message: confirmationMessage, Email: email})
}

// maxCodeFormBytes bounds the size of a submitted code-verification form.
const maxCodeFormBytes = 4 << 10 // 4 KiB - generous for a one-field code form

// handleCodeSubmit validates a submitted login code against the email that
// requested it. A wrong, expired, or already-used code all produce the same
// generic invalidCodeMessage with the code field cleared - the handler never
// reveals which of the three applies (I/O matrix). On success it issues the
// signed session cookie and redirects to the (placeholder) home route.
func (s *server) handleCodeSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCodeFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	code := strings.TrimSpace(r.FormValue("code"))

	sess, err := s.auth.ValidateLoginCode(r.Context(), email, code)
	if err != nil {
		if !errors.Is(err, auth.ErrInvalidCode) {
			slog.Error("validate login code", "error", err)
		}
		renderLogin(w, loginPageData{Step: "code", Error: invalidCodeMessage, Email: email})
		return
	}

	s.issueSessionCookie(w, sess.ParticipantID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleHome is a minimal authenticated placeholder proving the
// session/sliding-timeout mechanism works. It is replaced by the real
// Predictions home page in a later epic.
func (s *server) handleHome(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte("Signed in.")); err != nil {
		slog.Error("write home placeholder response", "error", err)
	}
}

// requireSession wraps next so it only runs when the request carries a
// still-valid session cookie. A valid session has its cookie re-issued with
// a fresh issued-at on every request (the sliding timeout); a missing,
// malformed, or expired session instead renders the Login page - showing
// sessionExpiredMessage only when a cookie was present and had genuinely
// timed out, never for a cookie that was simply absent or invalid. A
// present-but-undecodable cookie (expired or malformed) is explicitly
// cleared on the response so the browser stops resending a dead cookie on
// every subsequent request.
func (s *server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			renderLogin(w, loginPageData{Step: "email"})
			return
		}

		sess, err := s.auth.DecodeSession(cookie.Value)
		if err != nil {
			clearSessionCookie(w)
			if errors.Is(err, auth.ErrSessionExpired) {
				renderLogin(w, loginPageData{Step: "email", Message: sessionExpiredMessage})
				return
			}
			renderLogin(w, loginPageData{Step: "email"})
			return
		}

		s.issueSessionCookie(w, sess.ParticipantID)
		next(w, r)
	}
}

// issueSessionCookie signs a fresh session token for participantID, stamped
// with the current time, and sets it on the response.
func (s *server) issueSessionCookie(w http.ResponseWriter, participantID string) {
	token := s.auth.EncodeSession(auth.Session{ParticipantID: participantID, IssuedAt: clock.NowTime()})

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie removes a dead session cookie from the browser (MaxAge
// -1 deletes it immediately) so a decode failure doesn't leave the client
// resending a cookie that will never authenticate again.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// renderLogin executes the login template with data.
func renderLogin(w http.ResponseWriter, data loginPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := loginTemplate.Execute(w, data); err != nil {
		slog.Error("render login template", "error", err)
	}
}
