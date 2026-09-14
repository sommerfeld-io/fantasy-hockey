// Package web is the presentation layer: it serves the home page and the
// login-code request flow over the standard library's net/http, rendering
// server-side html/template views.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/clock"
	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// templates holds every parsed view. Parsing once at package init means a
// broken template fails fast at startup rather than on first render.
var templates = template.Must(template.ParseFS(templatesFS, "templates/*.html"))

// genericErrorBody is shown for a failure a Player can't do anything about
// (e.g. a disk write failing) - deliberately generic, never leaking
// internals into the response.
const genericErrorBody = "Something went wrong. Please try again."

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

// NewServer wires the application's routes and returns an http.Handler
// ready to be served. st and send back the login-code request flow
// (GET/POST /login, POST /login/code); secret signs the session cookie a
// successful POST /login/code sets. The existing home route is untouched.
func NewServer(st *store.Store, send mailer.Sender, secret string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleHome)
	mux.HandleFunc("GET /login", handleLoginForm)
	mux.HandleFunc("POST /login", handleLoginSubmit(st, send))
	mux.HandleFunc("POST /login/code", handleLoginCodeSubmit(st, secret))
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFiles())))
	return mux
}

// staticFiles returns static/'s contents rooted at "/", so a request for
// /static/styles.css maps to the embedded static/styles.css.
func staticFiles() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// static/ is embedded at build time via go:embed above, so this
		// can only fail from a programmer error, not runtime input.
		panic(fmt.Sprintf("web: static assets: %v", err))
	}
	return sub
}

// handleHome renders the application name and the current date and time.
func handleHome(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	body := fmt.Sprintf("<h1>Fantasy Hockey</h1>\n<p>%s</p>\n", clock.NowTime().Format(time.RFC1123))
	if _, err := w.Write([]byte(body)); err != nil {
		slog.Error("write home page response", "error", err)
	}
}

// handleLoginForm renders the email-entry step of the login flow.
func handleLoginForm(w http.ResponseWriter, _ *http.Request) {
	renderTemplate(w, "login-email.html", nil)
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
// all re-render the same code-entry screen with an identical generic error
// and the submitted value retained (FR-2) - ValidateLoginCode never tells
// the three cases apart, so this handler can't leak which one happened
// either. Only a failure to persist the consumed row surfaces as a 500.
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
			renderTemplate(w, "login-code.html", loginCodeData{Code: code, Error: genericCodeErrorText})
			return
		}

		http.SetCookie(w, auth.IssueSessionCookie(playerID, secret))
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// renderTemplate writes name to w with data available to it, logging
// (rather than surfacing) a rendering failure, since the response has
// typically already started streaming by the time html/template can fail.
func renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("render template", "template", name, "error", err)
	}
}
