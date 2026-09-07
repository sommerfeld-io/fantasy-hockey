package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// fakeStore is an in-memory Store used to test Service without a real
// database connection.
type fakeStore struct {
	participants map[string]store.Participant
	codes        []store.LoginCode
	lookupErr    error
	insertErr    error
}

func (f *fakeStore) ParticipantByEmail(_ context.Context, email string) (store.Participant, error) {
	if f.lookupErr != nil {
		return store.Participant{}, f.lookupErr
	}
	p, ok := f.participants[email]
	if !ok {
		return store.Participant{}, store.ErrParticipantNotFound
	}
	return p, nil
}

func (f *fakeStore) InsertLoginCode(_ context.Context, code store.LoginCode) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.codes = append(f.codes, code)
	return nil
}

// fakeMailer is an in-memory Mailer used to test Service without sending
// real email.
type fakeMailer struct {
	sent    []sentEmail
	sendErr error
}

type sentEmail struct {
	to, subject, body string
}

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sent = append(f.sent, sentEmail{to: to, subject: subject, body: body})
	return nil
}

const matchedEmail = "basti@example.com"

func newMatchedFakeStore() *fakeStore {
	return &fakeStore{participants: map[string]store.Participant{
		matchedEmail: {ID: "participant-1", Name: "Basti", Email: matchedEmail},
	}}
}

func TestRequestLoginCodeShouldPersistAndSendWhenEmailMatches(t *testing.T) {
	fs := newMatchedFakeStore()
	fm := &fakeMailer{}
	svc := NewService(fs, fm)

	if err := svc.RequestLoginCode(context.Background(), matchedEmail); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(fs.codes) != 1 {
		t.Fatalf("expected 1 login code to be persisted, got %d", len(fs.codes))
	}
	if fs.codes[0].ParticipantID != "participant-1" {
		t.Errorf("expected login code to reference the matched participant, got %q", fs.codes[0].ParticipantID)
	}
	if len(fm.sent) != 1 || fm.sent[0].to != matchedEmail {
		t.Fatalf("expected 1 email sent to the matched participant, got %+v", fm.sent)
	}
}

func TestRequestLoginCodeShouldNotPersistOrSendWhenEmailDoesNotMatch(t *testing.T) {
	fs := newMatchedFakeStore()
	fm := &fakeMailer{}
	svc := NewService(fs, fm)

	err := svc.RequestLoginCode(context.Background(), "stranger@example.com")

	if err != nil {
		t.Fatalf("expected nil error for a non-matching email, got %v", err)
	}
	if len(fs.codes) != 0 {
		t.Errorf("expected no login code to be persisted, got %d", len(fs.codes))
	}
	if len(fm.sent) != 0 {
		t.Errorf("expected no email to be sent, got %d", len(fm.sent))
	}
}

var sixDigitCode = regexp.MustCompile(`\b\d{6}\b`)

func TestRequestLoginCodeShouldPersistOnlyTheHashedCodeNeverThePlaintext(t *testing.T) {
	fs := newMatchedFakeStore()
	fm := &fakeMailer{}
	svc := NewService(fs, fm)

	if err := svc.RequestLoginCode(context.Background(), matchedEmail); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.codes) != 1 || len(fm.sent) != 1 {
		t.Fatalf("expected exactly 1 persisted code and 1 sent email, got %d/%d", len(fs.codes), len(fm.sent))
	}

	plainCode := sixDigitCode.FindString(fm.sent[0].body)
	if plainCode == "" {
		t.Fatalf("expected the emailed body to contain a 6-digit code, got %q", fm.sent[0].body)
	}

	persistedHash := fs.codes[0].CodeHash
	if persistedHash == plainCode {
		t.Fatal("expected the persisted CodeHash to differ from the plaintext code")
	}
	if got, want := persistedHash, hashCode(plainCode); got != want {
		t.Errorf("expected CodeHash to be sha256(plaintext code); got %q, want %q", got, want)
	}
}

func TestRequestLoginCodeShouldCreateASecondCodeOnARepeatRequestWithoutTouchingTheFirst(t *testing.T) {
	fs := newMatchedFakeStore()
	fm := &fakeMailer{}
	svc := NewService(fs, fm)

	if err := svc.RequestLoginCode(context.Background(), matchedEmail); err != nil {
		t.Fatalf("first request: unexpected error: %v", err)
	}
	firstHash := fs.codes[0].CodeHash

	if err := svc.RequestLoginCode(context.Background(), matchedEmail); err != nil {
		t.Fatalf("second request: unexpected error: %v", err)
	}

	if len(fs.codes) != 2 {
		t.Fatalf("expected 2 persisted login codes after a repeat request, got %d", len(fs.codes))
	}
	if fs.codes[0].CodeHash != firstHash {
		t.Error("expected the first login code to remain untouched by the repeat request")
	}
	if fs.codes[0].UsedAt != nil {
		t.Error("expected the first login code to remain unused")
	}
	if len(fm.sent) != 2 {
		t.Errorf("expected 2 emails sent, got %d", len(fm.sent))
	}
}

func TestRequestLoginCodeShouldPropagateStoreLookupErrorsOtherThanNotFound(t *testing.T) {
	wantErr := errors.New("boom: database unavailable")
	fs := &fakeStore{lookupErr: wantErr}
	fm := &fakeMailer{}
	svc := NewService(fs, fm)

	err := svc.RequestLoginCode(context.Background(), matchedEmail)

	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected an error wrapping %v, got %v", wantErr, err)
	}
	if len(fm.sent) != 0 {
		t.Errorf("expected no email to be sent when the lookup fails, got %d", len(fm.sent))
	}
}

func TestRequestLoginCodeShouldNotSendEmailWhenPersistingFails(t *testing.T) {
	wantErr := errors.New("boom: insert failed")
	fs := newMatchedFakeStore()
	fs.insertErr = wantErr
	fm := &fakeMailer{}
	svc := NewService(fs, fm)

	err := svc.RequestLoginCode(context.Background(), matchedEmail)

	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected an error wrapping %v, got %v", wantErr, err)
	}
	if len(fm.sent) != 0 {
		t.Errorf("expected no email to be sent when persisting the code fails, got %d", len(fm.sent))
	}
}

func TestRequestLoginCodeShouldPropagateMailerErrors(t *testing.T) {
	wantErr := errors.New("boom: smtp unavailable")
	fs := newMatchedFakeStore()
	fm := &fakeMailer{sendErr: wantErr}
	svc := NewService(fs, fm)

	err := svc.RequestLoginCode(context.Background(), matchedEmail)

	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected an error wrapping %v, got %v", wantErr, err)
	}
	if len(fs.codes) != 1 {
		t.Errorf("expected the code to remain persisted even though the email failed to send, got %d", len(fs.codes))
	}
}

func TestGenerateCodeShouldReturnASixDigitNumericString(t *testing.T) {
	code, err := generateCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
		t.Errorf("expected a 6-digit numeric string, got %q", code)
	}
}

func TestGenerateCodeShouldNotAlwaysReturnTheSameValue(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[code] = true
	}

	if len(seen) < 2 {
		t.Errorf("expected varied codes across 20 calls, got only %v", seen)
	}
}

func TestHashCodeShouldReturnTheSameHashForTheSameInput(t *testing.T) {
	const code = "123456"
	first := hashCode(code)
	second := hashCode(code)

	if first != second {
		t.Error("expected hashCode to be deterministic for the same input")
	}
}

func TestHashCodeShouldReturnDifferentHashesForDifferentInput(t *testing.T) {
	if hashCode("123456") == hashCode("654321") {
		t.Error("expected different inputs to produce different hashes")
	}
}
