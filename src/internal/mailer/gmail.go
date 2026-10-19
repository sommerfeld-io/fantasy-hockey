package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Google's real endpoints and the OAuth scope. They are constants, never read
// from the environment; tests override them through Settings.
const (
	gmailTokenURL = "https://oauth2.googleapis.com/token"
	gmailSendURL  = "https://gmail.googleapis.com/gmail/v1/users/me/messages/send"

	// gmailScope is the only scope needed. It is sent with the refresh so
	// the access token is never broader than sending mail.
	gmailScope = "https://www.googleapis.com/auth/gmail.send"
)

const (
	// gmailTimeout stops a hung connection from holding a login request open.
	gmailTimeout = 10 * time.Second

	// gmailExpiryMargin refreshes the token slightly early so it cannot
	// expire between the cache check and Google handling the send.
	gmailExpiryMargin = 30 * time.Second

	// maxGmailResponseBytes bounds how much of a Google reply is read.
	maxGmailResponseBytes = 64 << 10

	// maxGmailErrorRunes bounds an error text, applied after redaction so a
	// secret cut in half by the limit cannot leave a prefix behind.
	maxGmailErrorRunes = 300
)

// redactedSecret replaces any secret or token found in an error text.
const redactedSecret = "[redacted]"

// errLineBreak is returned for header-injection attempts. It carries no
// input, since the input may be the recipient.
var errLineBreak = errors.New("mailer: sender, recipient or subject contains a line break")

// gmailSender sends mail through the Gmail API with OAuth 2.0 credentials.
type gmailSender struct {
	s      Settings
	client *http.Client
	now    func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time

	// issued is the last access token Google handed out. It outlives a
	// dropped cache entry so a later error can still be redacted.
	issued string
}

// newGmailSender builds the Gmail Sender. Missing settings do not fail here,
// so the app starts without them; every send checks them instead.
func newGmailSender(s Settings, timeout time.Duration, now func() time.Time) Sender {
	if s.GmailTokenURL == "" {
		s.GmailTokenURL = gmailTokenURL
	}
	if s.GmailSendURL == "" {
		s.GmailSendURL = gmailSendURL
	}
	g := &gmailSender{s: s, client: &http.Client{Timeout: timeout}, now: now}
	return g.send
}

func (g *gmailSender) send(to, subject, body string) error {
	if err := g.checkSettings(); err != nil {
		return err
	}
	if strings.ContainsAny(to+subject+g.s.GmailSender, "\r\n") {
		return errLineBreak
	}

	ctx, cancel := context.WithTimeout(context.Background(), g.client.Timeout)
	defer cancel()
	if err := g.post(ctx, to, subject, body); err != nil {
		return &redactedError{cause: err, text: truncateRunes(g.redact(err.Error(), to), maxGmailErrorRunes)}
	}
	return nil
}

func (g *gmailSender) checkSettings() error {
	for _, v := range gmailVars(g.s) {
		if v.value == "" {
			return fmt.Errorf("mailer: %s is not set", v.name)
		}
	}
	return nil
}

func (g *gmailSender) post(ctx context.Context, to, subject, body string) error {
	token, err := g.accessToken(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"raw": g.rawMessage(to, subject, body)})
	if err != nil {
		return fmt.Errorf("mailer: encode gmail message: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.s.GmailSendURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("mailer: build gmail send request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	status, respBody, err := g.do(req)
	if err != nil {
		return fmt.Errorf("mailer: gmail send: %w", err)
	}
	if status < 200 || status > 299 {
		if status == http.StatusUnauthorized {
			g.dropToken()
		}
		return fmt.Errorf("mailer: gmail send failed: status %d: %s", status, googleReason(respBody))
	}
	return nil
}

// accessToken returns the cached token, or exchanges the refresh token for a
// new one when none is cached or it is about to expire.
func (g *gmailSender) accessToken(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.token != "" && g.now().Before(g.expires) {
		return g.token, nil
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {g.s.GmailClientID},
		"client_secret": {g.s.GmailClientSecret},
		"refresh_token": {g.s.GmailRefreshToken},
		"scope":         {gmailScope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.s.GmailTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("mailer: build gmail token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	status, respBody, err := g.do(req)
	if err != nil {
		return "", fmt.Errorf("mailer: gmail token request: %w", err)
	}
	if status < 200 || status > 299 {
		return "", fmt.Errorf("mailer: gmail token request failed: status %d: %s", status, googleReason(respBody))
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &tok); err != nil || tok.AccessToken == "" {
		return "", errors.New("mailer: gmail token response has no access token")
	}
	g.token, g.issued = tok.AccessToken, tok.AccessToken
	g.expires = g.now().Add(time.Duration(tok.ExpiresIn)*time.Second - gmailExpiryMargin)
	return g.token, nil
}

func (g *gmailSender) dropToken() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.token = ""
}

func (g *gmailSender) do(req *http.Request) (int, []byte, error) {
	resp, err := g.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxGmailResponseBytes))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, b, nil
}

// rawMessage builds the RFC 5322 message with CRLF line endings, base64url
// encoded as users.messages.send expects.
func (g *gmailSender) rawMessage(to, subject, body string) string {
	crlfBody := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n")
	msg := "From: " + g.s.GmailSender + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + crlfBody + "\r\n"
	return base64.URLEncoding.EncodeToString([]byte(msg))
}

// redact removes the recipient and every secret or token, in plain and
// URL-escaped form, from text. Google may echo any of them back.
func (g *gmailSender) redact(text, to string) string {
	g.mu.Lock()
	access := g.issued
	g.mu.Unlock()

	for _, v := range []string{to, g.s.GmailClientSecret, g.s.GmailRefreshToken, access} {
		for _, form := range []string{v, url.QueryEscape(v), url.PathEscape(v)} {
			if form != "" {
				text = redactAll(text, form, redactedSecret)
			}
		}
	}
	return text
}

// redactAll replaces every occurrence of needle in text, ignoring case.
func redactAll(text, needle, with string) string {
	return regexp.MustCompile(`(?i)`+regexp.QuoteMeta(needle)).ReplaceAllLiteralString(text, with)
}

// googleReason extracts the error reason from either Google error shape:
// {"error":"invalid_grant","error_description":"..."} or
// {"error":{"message":"...","status":"..."}}. It falls back to the raw body.
func googleReason(body []byte) string {
	var shape struct {
		Error       json.RawMessage `json:"error"`
		Description string          `json:"error_description"`
	}
	reason := strings.TrimSpace(string(body))
	if json.Unmarshal(body, &shape) == nil && len(shape.Error) > 0 {
		var code string
		var detail struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		}
		switch {
		case json.Unmarshal(shape.Error, &code) == nil:
			reason = strings.TrimSpace(code + " " + shape.Description)
		case json.Unmarshal(shape.Error, &detail) == nil:
			reason = strings.TrimSpace(detail.Status + " " + detail.Message)
		}
	}
	if strings.TrimSpace(reason) == "" {
		reason = strings.TrimSpace(string(body))
	}
	return reason
}

// truncateRunes cuts text to at most max runes.
func truncateRunes(text string, max int) string {
	if r := []rune(text); len(r) > max {
		return string(r[:max])
	}
	return text
}
