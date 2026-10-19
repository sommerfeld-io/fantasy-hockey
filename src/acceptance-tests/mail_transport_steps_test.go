package acceptance_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/mailer"
	"github.com/sommerfeld-io/fantasy-hockey/internal/observe"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// fakeSMTPServer is a minimal in-process SMTP server that counts connections
// and records each accepted message's recipient and data.
type fakeSMTPServer struct {
	listener net.Listener
	mu       sync.Mutex
	conns    int
	messages []fakeSMTPMessage
}

type fakeSMTPMessage struct {
	rcpt string
	data string
}

func newFakeSMTPServer() (*fakeSMTPServer, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	f := &fakeSMTPServer{listener: l}
	go f.serve()
	return f, nil
}

func (f *fakeSMTPServer) serve() {
	for {
		c, err := f.listener.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.conns++
		f.mu.Unlock()
		go f.handle(c)
	}
}

func (f *fakeSMTPServer) handle(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	reply := func(s string) { _, _ = io.WriteString(c, s+"\r\n") }
	reply("220 fake ESMTP")
	var msg fakeSMTPMessage
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			reply("250 fake")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			msg.rcpt = strings.Trim(strings.TrimSpace(line)[len("RCPT TO:"):], "<> ")
			reply("250 ok")
		case strings.HasPrefix(cmd, "DATA"):
			reply("354 go ahead")
			var data strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(l, "\r\n") == "." {
					break
				}
				data.WriteString(l)
			}
			msg.data = data.String()
			f.mu.Lock()
			f.messages = append(f.messages, msg)
			f.mu.Unlock()
			reply("250 queued")
		case strings.HasPrefix(cmd, "QUIT"):
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

func (f *fakeSMTPServer) snapshot() (int, []fakeSMTPMessage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conns, append([]fakeSMTPMessage(nil), f.messages...)
}

// fakeGmailServer stands in for Google's token and send endpoints and records
// every request.
type fakeGmailServer struct {
	server *httptest.Server
	mu     sync.Mutex
	tokens int
	sends  []string

	// rejectFirst makes the first send answer 401, as a revoked token does.
	rejectFirst bool
	rejected    int
}

func newFakeGmailServer() *fakeGmailServer {
	f := &fakeGmailServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "refresh_token" ||
			r.PostForm.Get("client_id") == "" || r.PostForm.Get("client_secret") == "" || r.PostForm.Get("refresh_token") == "" {
			http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.tokens++
		f.mu.Unlock()
		_, _ = io.WriteString(w, `{"access_token":"fake-access-token","expires_in":3600,"token_type":"Bearer"}`)
	})
	mux.HandleFunc("/send", f.handleSend)
	f.server = httptest.NewServer(mux)
	return f
}

func (f *fakeGmailServer) handleSend(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	reject := f.rejectFirst && f.rejected == 0
	if reject {
		f.rejected++
	}
	f.mu.Unlock()
	if reject {
		http.Error(w, `{"error":{"message":"invalid credentials"}}`, http.StatusUnauthorized)
		return
	}
	if r.Header.Get("Authorization") != "Bearer fake-access-token" {
		http.Error(w, `{"error":{"message":"missing or wrong bearer token"}}`, http.StatusUnauthorized)
		return
	}
	var payload struct {
		Raw string `json:"raw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":{"message":"bad json"}}`, http.StatusBadRequest)
		return
	}
	msg, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(payload.Raw, "="))
	if err != nil {
		http.Error(w, `{"error":{"message":"bad base64"}}`, http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.sends = append(f.sends, string(msg))
	f.mu.Unlock()
	_, _ = io.WriteString(w, `{"id":"fake"}`)
}

func (f *fakeGmailServer) snapshot() (int, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokens, append([]string(nil), f.sends...)
}

// gmailRecipient returns the To header of a decoded message.
func gmailRecipient(msg string) string {
	for _, line := range strings.Split(msg, "\r\n") {
		if v, ok := strings.CutPrefix(line, "To: "); ok {
			return v
		}
	}
	return ""
}

// mailTransportState holds the fixtures and results of one mail-transport
// scenario; a fresh one is created per scenario.
type mailTransportState struct {
	smtp     *fakeSMTPServer
	env      map[string]string
	email    string
	name     string
	sender   mailer.Sender
	startErr error
	ignored  []string
	missing  []string
	server   *httptest.Server
	sendDone chan struct{}
	smtpHost string
	dataFile string
	gmail    *fakeGmailServer
	sendErr  error
}

func newMailTransportState() *mailTransportState {
	return &mailTransportState{env: map[string]string{}}
}

func (s *mailTransportState) close() {
	if s.server != nil {
		s.server.Close()
	}
	if s.gmail != nil {
		s.gmail.server.Close()
	}
	if s.smtp != nil {
		_ = s.smtp.listener.Close()
	}
	if s.dataFile != "" {
		_ = os.RemoveAll(filepath.Dir(s.dataFile))
	}
}

func (s *mailTransportState) aFakeSMTPServerIsRunning() error {
	f, err := newFakeSMTPServer()
	if err != nil {
		return err
	}
	s.smtp = f
	return nil
}

func (s *mailTransportState) theSMTPSettingsPointAtTheFakeSMTPServer() error {
	host, port, err := net.SplitHostPort(s.smtp.listener.Addr().String())
	if err != nil {
		return fmt.Errorf("split address: %w", err)
	}
	s.smtpHost = host
	s.env[mailer.EnvSMTPHost] = host
	s.env[mailer.EnvSMTPPort] = port
	return nil
}

func (s *mailTransportState) aFakeGmailServerIsRunning() error {
	s.gmail = newFakeGmailServer()
	return nil
}

func (s *mailTransportState) theFakeGmailServerRejectsTheFirstSendWith401() error {
	s.gmail.mu.Lock()
	defer s.gmail.mu.Unlock()
	s.gmail.rejectFirst = true
	return nil
}

func (s *mailTransportState) theFakeGmailServerRejectedExactlyNSendRequests(n int) error {
	s.gmail.mu.Lock()
	defer s.gmail.mu.Unlock()
	if s.gmail.rejected != n {
		return fmt.Errorf("expected %d rejected sends, got %d", n, s.gmail.rejected)
	}
	return nil
}

func (s *mailTransportState) theGmailSettingsPointAtTheFakeGmailServer() error {
	s.env[mailer.EnvGmailClientID] = "acceptance-client-id"
	s.env[mailer.EnvGmailClientSecret] = "acceptance-client-secret"
	s.env[mailer.EnvGmailRefreshToken] = "acceptance-refresh-token"
	s.env[mailer.EnvGmailSender] = "pool@example.com"
	return nil
}

func (s *mailTransportState) aPoolPlayerWithEmail(name, email string) error {
	s.name, s.email = name, email
	return nil
}

func (s *mailTransportState) theMailSettingIs(name, value string) error {
	s.env[name] = value
	return nil
}

// theAppStarts builds the sender exactly as main does: settings from the
// environment values, then mailer.New. A failed build is recorded, not
// returned, so a scenario can assert on the startup error.
func (s *mailTransportState) theAppStarts() error {
	settings := mailer.SettingsFromEnv(func(k string) string { return s.env[k] })
	s.ignored = mailer.IgnoredVars(settings)
	s.missing = mailer.MissingVars(settings)
	if s.gmail != nil {
		settings.GmailTokenURL = s.gmail.server.URL + "/token"
		settings.GmailSendURL = s.gmail.server.URL + "/send"
	}

	send, err := mailer.New(settings)
	if err != nil {
		s.startErr = err
		return nil
	}

	dir, err := os.MkdirTemp("", "fantasy-hockey-mail-transport-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	s.dataFile = filepath.Join(dir, store.DataFileName)
	seed := fmt.Sprintf("season: \"2026-27\"\nplayers:\n    - id: %s\n      name: %s\n      email: %s\n",
		strings.ToLower(s.name), s.name, s.email)
	if err := os.WriteFile(s.dataFile, []byte(seed), 0o600); err != nil {
		return fmt.Errorf("seed data file: %w", err)
	}
	st, err := store.New(s.dataFile)
	if err != nil {
		return fmt.Errorf("store.New: %w", err)
	}

	// Signal once the sender returned, so "no SMTP connection" is asserted
	// after the async send really ran rather than after a sleep.
	s.sendDone = make(chan struct{}, 1)
	s.sender = func(to, subject, body string) error {
		defer func() {
			select {
			case s.sendDone <- struct{}{}:
			default:
			}
		}()
		err := send(to, subject, body)
		s.sendErr = err
		return err
	}
	s.server = httptest.NewServer(web.NewServer(st, s.sender, testSessionSecret, observe.New()))
	return nil
}

func (s *mailTransportState) thePoolPlayerAsksForALoginCode() error {
	if s.server == nil {
		return fmt.Errorf("the app did not start: %w", s.startErr)
	}
	resp, err := http.PostForm(s.server.URL+"/login", url.Values{"email": {s.email}})
	if err != nil {
		return fmt.Errorf("post /login: %w", err)
	}
	_ = resp.Body.Close()

	select {
	case <-s.sendDone:
		return nil
	case <-time.After(sendWaitTimeout):
		return fmt.Errorf("timed out waiting for the mail sender to run")
	}
}

func (s *mailTransportState) theFakeSMTPServerReceivedExactlyNMessagesAddressedTo(n int, email string) error {
	deadline := time.Now().Add(sendWaitTimeout)
	for {
		_, msgs := s.smtp.snapshot()
		if len(msgs) >= n || time.Now().After(deadline) {
			if len(msgs) != n {
				return fmt.Errorf("expected %d message(s), got %d", n, len(msgs))
			}
			for _, m := range msgs {
				if !strings.EqualFold(m.rcpt, email) {
					return fmt.Errorf("expected recipient %q, got %q", email, m.rcpt)
				}
			}
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *mailTransportState) theMessageContainsASixDigitLoginCode() error {
	_, msgs := s.smtp.snapshot()
	if len(msgs) == 0 {
		return fmt.Errorf("no message received")
	}
	if extractSixDigitCode(msgs[0].data) == "" {
		return fmt.Errorf("no 6-digit code in message %q", msgs[0].data)
	}
	return nil
}

func (s *mailTransportState) theFakeGmailServerReceivedExactlyNSendRequestsAddressedTo(n int, email string) error {
	_, sends := s.gmail.snapshot()
	if len(sends) != n {
		return fmt.Errorf("expected %d send request(s), got %d", n, len(sends))
	}
	for _, m := range sends {
		if !strings.EqualFold(gmailRecipient(m), email) {
			return fmt.Errorf("expected recipient %q, got %q", email, gmailRecipient(m))
		}
	}
	return nil
}

// theDecodedMessageContainsALoginCodeThatLogsThePlayerIn submits the code
// from the decoded Gmail message to POST /login/code and expects a session.
func (s *mailTransportState) theDecodedMessageContainsALoginCodeThatLogsThePlayerIn() error {
	_, sends := s.gmail.snapshot()
	if len(sends) == 0 {
		return fmt.Errorf("no message received")
	}
	code := extractSixDigitCode(sends[0])
	if code == "" {
		return fmt.Errorf("no 6-digit code in message %q", sends[0])
	}
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.PostForm(s.server.URL+"/login/code", url.Values{"code": {code}})
	if err != nil {
		return fmt.Errorf("post /login/code: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound || len(resp.Cookies()) == 0 {
		return fmt.Errorf("expected a 302 with a session cookie, got %d and %d cookies", resp.StatusCode, len(resp.Cookies()))
	}
	return nil
}

func (s *mailTransportState) theMailSendFailedWithAnErrorNaming(name string) error {
	if s.sendErr == nil {
		return fmt.Errorf("expected a send error naming %q, got none", name)
	}
	if !strings.Contains(s.sendErr.Error(), name) {
		return fmt.Errorf("expected a send error naming %q, got %w", name, s.sendErr)
	}
	return nil
}

func (s *mailTransportState) theFakeGmailServerReceivedNoRequest() error {
	if tokens, sends := s.gmail.snapshot(); tokens+len(sends) != 0 {
		return fmt.Errorf("expected no Gmail request, got %d token and %d send", tokens, len(sends))
	}
	return nil
}

func (s *mailTransportState) theFakeSMTPServerReceivedNoConnection() error {
	if conns, _ := s.smtp.snapshot(); conns != 0 {
		return fmt.Errorf("expected no SMTP connection, got %d", conns)
	}
	return nil
}

func (s *mailTransportState) theAppDidNotWarnAboutIgnoredMailSettings() error {
	if len(s.ignored) != 0 {
		return fmt.Errorf("expected no ignored settings, got %v", s.ignored)
	}
	return nil
}

func (s *mailTransportState) theAppWarnedAboutIgnoring(names ...string) error {
	for _, n := range names {
		if !containsString(s.ignored, n) {
			return fmt.Errorf("expected %q among the ignored settings, got %v", n, s.ignored)
		}
	}
	return nil
}

func (s *mailTransportState) theAppWarnedAboutIgnoringOne(a string) error {
	return s.theAppWarnedAboutIgnoring(a)
}

func (s *mailTransportState) theAppWarnedAboutIgnoringTwo(a, b string) error {
	return s.theAppWarnedAboutIgnoring(a, b)
}

func (s *mailTransportState) theWarningDoesNotContainTheSMTPHostValue() error {
	return s.theWarningDoesNotContain(s.smtpHost)
}

func (s *mailTransportState) theWarningDoesNotContain(value string) error {
	if strings.Contains(strings.Join(s.ignored, " "), value) {
		return fmt.Errorf("warning variables %v must not contain %q", s.ignored, value)
	}
	return nil
}

func (s *mailTransportState) theAppStartedWithoutError() error {
	if s.startErr != nil {
		return fmt.Errorf("expected the app to start, got %w", s.startErr)
	}
	return nil
}

func (s *mailTransportState) theAppWarnedAboutTheMissingGmailSettings(a, b, c string) error {
	want := []string{a, b, c}
	if strings.Join(s.missing, ",") != strings.Join(want, ",") {
		return fmt.Errorf("expected the missing settings %v, got %v", want, s.missing)
	}
	return nil
}

func (s *mailTransportState) theAppWarnedAboutTheMissingMailSettings(a, b string) error {
	if got, want := strings.Join(s.missing, ","), a+","+b; got != want {
		return fmt.Errorf("expected the missing settings %v, got %v", want, got)
	}
	return nil
}

func (s *mailTransportState) theAppDidNotWarnAboutMissingGmailSettings() error {
	if len(s.missing) != 0 {
		return fmt.Errorf("expected no missing settings warning, got %v", s.missing)
	}
	return nil
}

func (s *mailTransportState) theMissingSettingsWarningDoesNotContain(value string) error {
	if strings.Contains(strings.Join(s.missing, " "), value) {
		return fmt.Errorf("missing-settings warning %v must not contain %q", s.missing, value)
	}
	return nil
}

func (s *mailTransportState) startupFailsWithAnErrorNaming(a, b, c string) error {
	if s.startErr == nil {
		return fmt.Errorf("expected startup to fail")
	}
	for _, want := range []string{a, b, c} {
		if !strings.Contains(s.startErr.Error(), want) {
			return fmt.Errorf("expected the error to name %q, got %q", want, s.startErr.Error())
		}
	}
	return nil
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// InitializeMailTransportScenario registers the mail-transport step
// definitions with GoDog.
func InitializeMailTransportScenario(ctx *godog.ScenarioContext) {
	s := newMailTransportState()
	ctx.Before(func(gctx context.Context, _ *godog.Scenario) (context.Context, error) {
		*s = *newMailTransportState()
		return gctx, nil
	})
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^a fake SMTP server is running$`, s.aFakeSMTPServerIsRunning)
	ctx.Step(`^the SMTP settings point at the fake SMTP server$`, s.theSMTPSettingsPointAtTheFakeSMTPServer)
	ctx.Step(`^a fake Gmail server is running$`, s.aFakeGmailServerIsRunning)
	ctx.Step(`^the Gmail settings point at the fake Gmail server$`, s.theGmailSettingsPointAtTheFakeGmailServer)
	ctx.Step(`^the fake Gmail server received exactly (\d+) send requests? addressed to "([^"]*)"$`, s.theFakeGmailServerReceivedExactlyNSendRequestsAddressedTo)
	ctx.Step(`^the decoded message contains a 6-digit login code that logs the pool player in$`, s.theDecodedMessageContainsALoginCodeThatLogsThePlayerIn)
	ctx.Step(`^the mail send failed with an error naming "([^"]*)"$`, s.theMailSendFailedWithAnErrorNaming)
	ctx.Step(`^the fake Gmail server received no request$`, s.theFakeGmailServerReceivedNoRequest)
	ctx.Step(`^a pool player "([^"]*)" with email "([^"]*)"$`, s.aPoolPlayerWithEmail)
	ctx.Step(`^the mail setting "([^"]*)" is "([^"]*)"$`, s.theMailSettingIs)
	ctx.Step(`^the app starts$`, s.theAppStarts)
	ctx.Step(`^the pool player asks for a login code$`, s.thePoolPlayerAsksForALoginCode)
	ctx.Step(`^the fake SMTP server received exactly (\d+) messages? addressed to "([^"]*)"$`, s.theFakeSMTPServerReceivedExactlyNMessagesAddressedTo)
	ctx.Step(`^the message contains a 6-digit login code$`, s.theMessageContainsASixDigitLoginCode)
	ctx.Step(`^the fake SMTP server received no connection$`, s.theFakeSMTPServerReceivedNoConnection)
	ctx.Step(`^the app did not warn about ignored mail settings$`, s.theAppDidNotWarnAboutIgnoredMailSettings)
	ctx.Step(`^the app warned about ignoring "([^"]*)" and "([^"]*)"$`, s.theAppWarnedAboutIgnoringTwo)
	ctx.Step(`^the app warned about ignoring "([^"]*)"$`, s.theAppWarnedAboutIgnoringOne)
	ctx.Step(`^the warning does not contain the SMTP host value$`, s.theWarningDoesNotContainTheSMTPHostValue)
	ctx.Step(`^the warning does not contain the value "([^"]*)"$`, s.theWarningDoesNotContain)
	ctx.Step(`^the app started without error$`, s.theAppStartedWithoutError)
	ctx.Step(`^the app warned about the missing Gmail settings "([^"]*)", "([^"]*)" and "([^"]*)"$`, s.theAppWarnedAboutTheMissingGmailSettings)
	ctx.Step(`^the fake Gmail server rejects the first send with 401$`, s.theFakeGmailServerRejectsTheFirstSendWith401)
	ctx.Step(`^the fake Gmail server rejected exactly (\d+) send requests?$`, s.theFakeGmailServerRejectedExactlyNSendRequests)
	ctx.Step(`^the app warned about the missing mail settings "([^"]*)" and "([^"]*)"$`, s.theAppWarnedAboutTheMissingMailSettings)
	ctx.Step(`^the app did not warn about missing mail settings$`, s.theAppDidNotWarnAboutMissingGmailSettings)
	ctx.Step(`^the fake Gmail server issued exactly (\d+) access tokens$`, s.theFakeGmailServerIssuedExactlyNAccessTokens)
	ctx.Step(`^the missing-settings warning does not contain the value "([^"]*)"$`, s.theMissingSettingsWarningDoesNotContain)
	ctx.Step(`^startup fails with an error naming "([^"]*)", "([^"]*)" and "([^"]*)"$`, s.startupFailsWithAnErrorNaming)
}

// theFakeGmailServerIssuedExactlyNAccessTokens proves a retry fetched a fresh
// token instead of reusing the one Google had just rejected.
func (s *mailTransportState) theFakeGmailServerIssuedExactlyNAccessTokens(n int) error {
	if tokens, _ := s.gmail.snapshot(); tokens != n {
		return fmt.Errorf("expected %d access token request(s), got %d", n, tokens)
	}
	return nil
}
