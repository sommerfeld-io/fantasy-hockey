package mailer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testClientID     = "test-client-id"
	testClientSecret = "test-client-secret-value"
	testRefreshToken = "1//test-refresh-token-value"
	testAccessToken  = "ya29.test-access-token-value"
	testSender       = "pool@example.com"
	testRecipient    = "player@example.com"
)

// fakeGoogle stands in for both Google endpoints and records every request.
type fakeGoogle struct {
	mu          sync.Mutex
	tokenCalls  []url.Values
	sendCalls   []fakeSendCall
	tokenStatus int
	tokenBody   string
	sendStatus  int
	sendBody    string
	expiresIn   int
	server      *httptest.Server
}

type fakeSendCall struct {
	auth        string
	contentType string
	raw         string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	g := &fakeGoogle{tokenStatus: http.StatusOK, sendStatus: http.StatusOK, expiresIn: 3600}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		g.mu.Lock()
		g.tokenCalls = append(g.tokenCalls, r.PostForm)
		status, body := g.tokenStatus, g.tokenBody
		expires := g.expiresIn
		g.mu.Unlock()
		w.WriteHeader(status)
		if body == "" {
			body = fmt.Sprintf(`{"access_token":%q,"expires_in":%d,"token_type":"Bearer"}`, testAccessToken, expires)
		}
		_, _ = io.WriteString(w, body)
	})
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Raw string `json:"raw"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		g.mu.Lock()
		g.sendCalls = append(g.sendCalls, fakeSendCall{
			auth: r.Header.Get("Authorization"), contentType: r.Header.Get("Content-Type"), raw: payload.Raw,
		})
		status, body := g.sendStatus, g.sendBody
		g.mu.Unlock()
		w.WriteHeader(status)
		if body == "" {
			body = `{"id":"abc"}`
		}
		_, _ = io.WriteString(w, body)
	})
	g.server = httptest.NewServer(mux)
	t.Cleanup(g.server.Close)
	return g
}

func (g *fakeGoogle) settings() Settings {
	return Settings{
		MailType: "API", GmailClientID: testClientID, GmailClientSecret: testClientSecret,
		GmailRefreshToken: testRefreshToken, GmailSender: testSender,
		GmailTokenURL: g.server.URL + "/token", GmailSendURL: g.server.URL + "/send",
	}
}

func (g *fakeGoogle) counts() (tokens, sends int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.tokenCalls), len(g.sendCalls)
}

func decodeRaw(t *testing.T, raw string) string {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(raw, "="))
	if err != nil {
		t.Fatalf("raw is not base64url: %v", err)
	}
	return string(b)
}

func newTestSender(t *testing.T, s Settings) Sender {
	t.Helper()
	send, err := New(s)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return send
}

func assertNoneOf(t *testing.T, text string, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(strings.ToLower(text), strings.ToLower(s)) {
			t.Errorf("error text %q must not contain %q", text, s)
		}
	}
}

func TestGmailSenderShouldExchangeTheRefreshTokenAndSendOneMessage(t *testing.T) {
	g := newFakeGoogle(t)
	send := newTestSender(t, g.settings())

	if err := send(testRecipient, "Your code", "Code: 123456"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.tokenCalls) != 1 || len(g.sendCalls) != 1 {
		t.Fatalf("expected one token and one send call, got %d and %d", len(g.tokenCalls), len(g.sendCalls))
	}
	form := g.tokenCalls[0]
	want := map[string]string{
		"grant_type": "refresh_token", "client_id": testClientID,
		"client_secret": testClientSecret, "refresh_token": testRefreshToken,
		"scope": "https://www.googleapis.com/auth/gmail.send",
	}
	for k, v := range want {
		if form.Get(k) != v {
			t.Errorf("token form %s: expected %q, got %q", k, v, form.Get(k))
		}
	}
	call := g.sendCalls[0]
	if call.auth != "Bearer "+testAccessToken {
		t.Errorf("expected Bearer header, got %q", call.auth)
	}
	if !strings.HasPrefix(call.contentType, "application/json") {
		t.Errorf("expected JSON content type, got %q", call.contentType)
	}
}

func TestGmailSenderShouldBuildACRLFMessageWithHeadersAndPlainTextBody(t *testing.T) {
	g := newFakeGoogle(t)
	send := newTestSender(t, g.settings())

	if err := send(testRecipient, "Your code", "line one\nline two"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := decodeRaw(t, g.sendCalls[0].raw)
	for _, want := range []string{
		"From: " + testSender + "\r\n", "To: " + testRecipient + "\r\n", "Subject: Your code\r\n",
		"Content-Type: text/plain", "\r\n\r\nline one\r\nline two",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected message to contain %q, got %q", want, msg)
		}
	}
	if strings.Contains(strings.ReplaceAll(msg, "\r\n", ""), "\n") {
		t.Errorf("message has a bare LF: %q", msg)
	}
}

func TestGmailSenderShouldRejectLineBreaksInRecipientOrSubjectWithoutAnyRequest(t *testing.T) {
	for name, args := range map[string][2]string{
		"LF in to":      {"a@example.com\nBcc: x@example.com", "s"},
		"CR in to":      {"a@example.com\rBcc: x@example.com", "s"},
		"CRLF in to":    {"a@example.com\r\nBcc: x@example.com", "s"},
		"LF in subject": {testRecipient, "s\nBcc: x@example.com"},
		"CR in subject": {testRecipient, "s\rBcc: x@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			g := newFakeGoogle(t)
			send := newTestSender(t, g.settings())

			err := send(args[0], args[1], "b")

			if err == nil {
				t.Fatal("expected an error")
			}
			assertNoneOf(t, err.Error(), args[0], "x@example.com")
			if tokens, sends := g.counts(); tokens+sends != 0 {
				t.Errorf("expected no request, got %d token and %d send", tokens, sends)
			}
		})
	}
}

func TestGmailSenderShouldReuseTheAccessTokenBeforeItExpires(t *testing.T) {
	g := newFakeGoogle(t)
	send := newTestSender(t, g.settings())

	for range 2 {
		if err := send(testRecipient, "s", "b"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if tokens, sends := g.counts(); tokens != 1 || sends != 2 {
		t.Fatalf("expected 1 token and 2 send calls, got %d and %d", tokens, sends)
	}
}

func TestGmailSenderShouldRequestANewTokenAfterExpiry(t *testing.T) {
	g := newFakeGoogle(t)
	g.expiresIn = 3600
	now := time.Now()
	send := newGmailSender(g.settings(), gmailTimeout, func() time.Time { return now })

	if err := send(testRecipient, "s", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	now = now.Add(3600 * time.Second)
	if err := send(testRecipient, "s", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tokens, _ := g.counts(); tokens != 2 {
		t.Fatalf("expected 2 token calls, got %d", tokens)
	}
}

func TestGmailSenderShouldNameTheFirstMissingSettingAndMakeNoRequest(t *testing.T) {
	cases := []struct {
		missing string
		clear   func(*Settings)
	}{
		{EnvGmailClientID, func(s *Settings) { s.GmailClientID = "" }},
		{EnvGmailClientSecret, func(s *Settings) { s.GmailClientSecret = "" }},
		{EnvGmailRefreshToken, func(s *Settings) { s.GmailRefreshToken = "" }},
		{EnvGmailSender, func(s *Settings) { s.GmailSender = "" }},
	}
	for _, c := range cases {
		t.Run(c.missing, func(t *testing.T) {
			g := newFakeGoogle(t)
			s := g.settings()
			c.clear(&s)
			send := newTestSender(t, s)

			err := send(testRecipient, "s", "b")

			if err == nil || !strings.Contains(err.Error(), c.missing) {
				t.Fatalf("expected an error naming %s, got %v", c.missing, err)
			}
			if tokens, sends := g.counts(); tokens+sends != 0 {
				t.Errorf("expected no request, got %d token and %d send", tokens, sends)
			}
		})
	}

	t.Run("all missing names the client ID first", func(t *testing.T) {
		send := newTestSender(t, Settings{MailType: "API"})

		err := send(testRecipient, "s", "b")

		if err == nil || !strings.Contains(err.Error(), EnvGmailClientID) {
			t.Fatalf("expected the client ID first, got %v", err)
		}
		for _, later := range []string{EnvGmailClientSecret, EnvGmailRefreshToken, EnvGmailSender} {
			if strings.Contains(err.Error(), later) {
				t.Errorf("error should only name the first missing variable, got %q", err)
			}
		}
	})
}

func TestGmailSenderShouldReportATokenEndpointFailureWithStatusAndReason(t *testing.T) {
	g := newFakeGoogle(t)
	g.tokenStatus = http.StatusBadRequest
	g.tokenBody = `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`
	send := newTestSender(t, g.settings())

	err := send(testRecipient, "s", "b")

	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"400", "invalid_grant"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in %q", want, err)
		}
	}
	assertNoneOf(t, err.Error(), testClientSecret, testRefreshToken, testRecipient)
	if _, sends := g.counts(); sends != 0 {
		t.Errorf("expected no send call, got %d", sends)
	}
}

func TestGmailSenderShouldReportATokenTransportErrorWithoutSecrets(t *testing.T) {
	g := newFakeGoogle(t)
	s := g.settings()
	g.server.Close()
	send := newTestSender(t, s)

	err := send(testRecipient, "s", "b")

	if err == nil {
		t.Fatal("expected an error")
	}
	assertNoneOf(t, err.Error(), testClientSecret, testRefreshToken, testRecipient)
}

func TestGmailSenderShouldReportASendFailureWithStatusAndReason(t *testing.T) {
	g := newFakeGoogle(t)
	g.sendStatus = http.StatusForbidden
	g.sendBody = `{"error":{"code":403,"message":"Request had insufficient authentication scopes.","status":"PERMISSION_DENIED"}}`
	send := newTestSender(t, g.settings())

	err := send(testRecipient, "s", "b")

	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"403", "insufficient authentication scopes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in %q", want, err)
		}
	}
	assertNoneOf(t, err.Error(), testRecipient, testAccessToken, testRefreshToken, testClientSecret)
}

func TestGmailSenderShouldRedactEverythingAnEchoingServerRepeats(t *testing.T) {
	echo := strings.Join([]string{
		testRecipient, strings.ToUpper(testRecipient), testClientSecret, testRefreshToken,
		url.QueryEscape(testRefreshToken),
	}, " | ")

	t.Run("send endpoint", func(t *testing.T) {
		g := newFakeGoogle(t)
		g.sendStatus = http.StatusBadRequest
		g.sendBody = fmt.Sprintf(`{"error":{"message":%q}}`, echo)
		send := newTestSender(t, g.settings())

		err := send(testRecipient, "s", "b")

		if err == nil {
			t.Fatal("expected an error")
		}
		assertNoneOf(t, err.Error(), testRecipient, testClientSecret, testRefreshToken,
			url.QueryEscape(testRefreshToken))
	})

	t.Run("send endpoint echoing the access token after a 401", func(t *testing.T) {
		g := newFakeGoogle(t)
		g.sendStatus = http.StatusUnauthorized
		g.sendBody = `{"error":{"message":"bad token ` + testAccessToken + `"}}`
		send := newTestSender(t, g.settings())

		err := send(testRecipient, "s", "b")

		if err == nil {
			t.Fatal("expected an error")
		}
		assertNoneOf(t, err.Error(), testAccessToken)
	})

	t.Run("send endpoint with a plain text body", func(t *testing.T) {
		g := newFakeGoogle(t)
		g.sendStatus = http.StatusBadGateway
		g.sendBody = echo
		send := newTestSender(t, g.settings())

		err := send(testRecipient, "s", "b")

		if err == nil {
			t.Fatal("expected an error")
		}
		assertNoneOf(t, err.Error(), testRecipient, testClientSecret, testRefreshToken, testAccessToken)
	})

	t.Run("token endpoint", func(t *testing.T) {
		g := newFakeGoogle(t)
		g.tokenStatus = http.StatusUnauthorized
		g.tokenBody = fmt.Sprintf(`{"error":"invalid_client","error_description":%q}`, echo)
		send := newTestSender(t, g.settings())

		err := send(testRecipient, "s", "b")

		if err == nil {
			t.Fatal("expected an error")
		}
		assertNoneOf(t, err.Error(), testRecipient, testClientSecret, testRefreshToken,
			url.QueryEscape(testRefreshToken))
	})
}

func TestGmailSenderShouldFailAfterTheTimeoutWhenGoogleHangs(t *testing.T) {
	release := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { <-release }))
	t.Cleanup(func() { close(release); hung.Close() })
	s := Settings{
		GmailClientID: testClientID, GmailClientSecret: testClientSecret, GmailRefreshToken: testRefreshToken,
		GmailSender: testSender, GmailTokenURL: hung.URL, GmailSendURL: hung.URL,
	}
	send := newGmailSender(s, 100*time.Millisecond, time.Now)

	start := time.Now()
	err := send(testRecipient, "s", "b")

	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("expected a quick failure, took %s", elapsed)
	}
	assertNoneOf(t, err.Error(), testRecipient, testClientSecret, testRefreshToken)
}

// roundTripFunc lets a test stand in for the network.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGmailSenderShouldSendToGoogleWhenNoURLsAreSet(t *testing.T) {
	var requested []string
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requested = append(requested, r.URL.String())
		body := `{}`
		if r.URL.Host == "oauth2.googleapis.com" {
			body = `{"access_token":"` + testAccessToken + `","expires_in":3600}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
	send := newGmailSender(Settings{
		GmailClientID: testClientID, GmailClientSecret: testClientSecret,
		GmailRefreshToken: testRefreshToken, GmailSender: testSender,
	}, gmailTimeout, time.Now)

	if err := send(testRecipient, "s", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"https://oauth2.googleapis.com/token", "https://gmail.googleapis.com/gmail/v1/users/me/messages/send"}
	if !slices.Equal(requested, want) {
		t.Fatalf("expected requests to %v, got %v", want, requested)
	}
}

func TestGmailSenderShouldRefetchTheTokenAfterGoogleRejectsIt(t *testing.T) {
	g := newFakeGoogle(t)
	send := newTestSender(t, g.settings())
	if err := send(testRecipient, "s", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g.mu.Lock()
	g.sendStatus = http.StatusUnauthorized
	g.mu.Unlock()
	if err := send(testRecipient, "s", "b"); err == nil {
		t.Fatal("expected an error")
	}
	g.mu.Lock()
	g.sendStatus = http.StatusOK
	g.mu.Unlock()

	if err := send(testRecipient, "s", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tokens, _ := g.counts(); tokens != 2 {
		t.Fatalf("expected the token to be fetched again, got %d token calls", tokens)
	}
}
