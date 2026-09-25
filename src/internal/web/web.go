// Package web is the presentation layer: it serves the app shell
// (Predict/Leaderboard/Compare) and the login-code request flow over the
// standard library's net/http, rendering server-side html/template views.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
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

// predictSheetPattern is the single source of truth for the per-Prediction-
// Set sheet page's route, registered on both authMux and the outer mux so the
// two can't drift out of sync, the same way shellRoutes is for the bottom-nav
// routes.
const predictSheetPattern = "GET /predict/{id}"

// predictSheetSubmitPattern is the single source of truth for the cup/
// presidents pick submission route, registered on both authMux and the outer
// mux the same way predictSheetPattern is, so the two can't drift.
const predictSheetSubmitPattern = "POST /predict/{id}"

// shellRoutes is the single source of truth for every authenticated
// app-shell route this server registers, pairing each route pattern with
// the bottom-nav tab it renders active. NewServer registers each pattern on
// both authMux and the outer mux from this one list, so the two can no
// longer drift out of sync as future stories add routes (AD-2/AD-11). GET
// /{$} aliases to the same Predict shell as GET /predict.
var shellRoutes = []struct {
	pattern string
	tab     string
}{
	{"GET /{$}", tabPredict},
	{"GET /predict", tabPredict},
	{"GET /leaderboard", tabLeaderboard},
	{"GET /compare", tabCompare},
}

// NewServer wires the application's routes and returns an http.Handler
// ready to be served. st and send back the login-code request flow
// (GET/POST /login, POST /login/code); secret signs the session cookie a
// successful POST /login/code sets and verifies the ones requireSession
// reads back. shellRoutes' authenticated app-shell routes are registered on
// their own authMux, which requireSession wraps before it's mounted on the
// outer mux alongside the public routes - a future protected route joins
// shellRoutes the same way, without touching how public routes are wired.
// GET /predict/{id} (the per-Prediction-Set sheet page) and POST /predict/{id}
// (its cup/presidents pick submission) are registered the same way but
// outside shellRoutes, since neither is a bottom-nav destination; they get
// their own handlers (handleSheet, handleSheetSubmit) rather than
// handleShell.
// POST /logout is registered directly on the outer mux rather than authMux,
// since clearing the session cookie must work even when the presented
// cookie is missing, expired, or tampered.
func NewServer(st *store.Store, send mailer.Sender, secret string) http.Handler {
	authMux := http.NewServeMux()
	for _, r := range shellRoutes {
		authMux.Handle(r.pattern, handleShell(st, r.tab))
	}
	authMux.Handle(predictSheetPattern, handleSheet(st))
	authMux.Handle(predictSheetSubmitPattern, handleSheetSubmit(st))
	protected := requireSession(secret, authMux)

	mux := http.NewServeMux()
	for _, r := range shellRoutes {
		mux.Handle(r.pattern, protected)
	}
	mux.Handle(predictSheetPattern, protected)
	mux.Handle(predictSheetSubmitPattern, protected)
	mux.HandleFunc("GET /login", handleLoginForm(secret))
	mux.HandleFunc("POST /login", handleLoginSubmit(st, send))
	mux.HandleFunc("POST /login/code", handleLoginCodeSubmit(st, secret))
	mux.HandleFunc("POST /logout", handleLogout)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFiles())))
	return mux
}

// requireSession wraps next so it only runs for a request carrying a valid,
// unexpired session cookie (auth.ValidateSession). A valid session is
// re-issued with a fresh issued_at before next runs, sliding the idle
// timeout forward (PRD FR-3); anything else - a missing cookie, a bad
// signature, an empty decoded player id, or one idle-expired past
// auth.SessionIdleTimeout - redirects to /login with no distinguishing
// message, since auth.ValidateSession never says which case occurred. Every
// authenticated response also gets Cache-Control: no-store, since a shared
// cache in front of the app could otherwise serve one player's page to
// another once this route's content stops being identical for everyone. The
// validated player id is threaded onto the request context (via
// auth.ContextWithPlayerID) so next and anything it calls can read identity
// without re-parsing the cookie.
func requireSession(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie(auth.SessionCookieName)
		playerID, ok := auth.ValidateSession(c, secret)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		http.SetCookie(w, auth.IssueSessionCookie(playerID, secret))
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r.WithContext(auth.ContextWithPlayerID(r.Context(), playerID)))
	})
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

// renderTemplate writes name to w with data available to it and an implicit
// 200 status. See renderTemplateStatus for a response that must not default
// to 200.
func renderTemplate(w http.ResponseWriter, name string, data any) {
	renderTemplateStatus(w, http.StatusOK, name, data)
}

// renderTemplateStatus is renderTemplate with an explicit status code,
// logging (rather than surfacing) a rendering failure, since the response
// has typically already started streaming by the time html/template can
// fail. The status must be written before any body bytes, so this always
// calls WriteHeader itself rather than letting the first Write default to
// 200.
func renderTemplateStatus(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("render template", "template", name, "error", err)
	}
}
