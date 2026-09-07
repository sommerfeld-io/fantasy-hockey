// Package web is the presentation layer: it renders the Login page and
// handles login-code requests over stdlib net/http and html/template.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*.css
var staticFS embed.FS

// confirmationMessage is shown after any login-code request, whether or not
// the submitted email matched a Participant. Never vary this string based on
// the outcome - doing so would leak which emails are registered (FR-1).
const confirmationMessage = "Check the entered email address."

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

// loginPageData drives the login.html template's two states: the initial
// email form, and the post-submission confirmation.
type loginPageData struct {
	Step    string
	Message string
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

	renderLogin(w, loginPageData{Step: "confirmation", Message: confirmationMessage})
}

// renderLogin executes the login template with data.
func renderLogin(w http.ResponseWriter, data loginPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := loginTemplate.Execute(w, data); err != nil {
		slog.Error("render login template", "error", err)
	}
}
