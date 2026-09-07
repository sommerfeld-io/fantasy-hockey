package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// fakeStore is an in-memory Store used to test Service without a real
// database connection.
type fakeStore struct {
	participants map[string]store.Participant
	codes        []store.LoginCode
	lookupErr    error
	insertErr    error
	unusedErr    error
	markUsedErr  error
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

func (f *fakeStore) UnusedLoginCodesForParticipant(_ context.Context, participantID string, issuedAfter time.Time) ([]store.LoginCode, error) {
	if f.unusedErr != nil {
		return nil, f.unusedErr
	}
	var out []store.LoginCode
	for _, c := range f.codes {
		if c.ParticipantID == participantID && c.UsedAt == nil && c.IssuedAt.After(issuedAfter) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeStore) MarkLoginCodeUsed(_ context.Context, id string) error {
	if f.markUsedErr != nil {
		return f.markUsedErr
	}
	for i := range f.codes {
		if f.codes[i].ID == id {
			usedAt := time.Now().UTC()
			f.codes[i].UsedAt = &usedAt
		}
	}
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

// testSessionSecret is the SESSION_SECRET stand-in used by every test that
// doesn't specifically exercise session signing behavior.
const testSessionSecret = "test-session-secret"

func newMatchedFakeStore() *fakeStore {
	return &fakeStore{participants: map[string]store.Participant{
		matchedEmail: {ID: "participant-1", Name: "Basti", Email: matchedEmail},
	}}
}

func TestRequestLoginCodeShouldPersistAndSendWhenEmailMatches(t *testing.T) {
	fs := newMatchedFakeStore()
	fm := &fakeMailer{}
	svc := NewService(fs, fm, testSessionSecret)

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
	svc := NewService(fs, fm, testSessionSecret)

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
	svc := NewService(fs, fm, testSessionSecret)

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
	svc := NewService(fs, fm, testSessionSecret)

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
	svc := NewService(fs, fm, testSessionSecret)

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
	svc := NewService(fs, fm, testSessionSecret)

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
	svc := NewService(fs, fm, testSessionSecret)

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

// newStoreWithLoginCode returns a fakeStore seeded with matchedEmail's
// Participant and one login code for it, issued issuedAgo in the past and
// used (if usedAt is non-nil).
func newStoreWithLoginCode(plainCode string, issuedAgo time.Duration, usedAt *time.Time) *fakeStore {
	fs := newMatchedFakeStore()
	fs.codes = append(fs.codes, store.LoginCode{
		ID:            "code-1",
		ParticipantID: "participant-1",
		CodeHash:      hashCode(plainCode),
		IssuedAt:      time.Now().UTC().Add(-issuedAgo),
		UsedAt:        usedAt,
	})
	return fs
}

func TestValidateLoginCodeShouldReturnASessionForAValidUnusedCode(t *testing.T) {
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	sess, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sess.ParticipantID != "participant-1" {
		t.Errorf("expected session to reference the matched participant, got %q", sess.ParticipantID)
	}
	if sess.IssuedAt.IsZero() {
		t.Error("expected the session's IssuedAt to be set")
	}
}

func TestValidateLoginCodeShouldMarkTheMatchingCodeUsedOnSuccess(t *testing.T) {
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	if _, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fs.codes[0].UsedAt == nil {
		t.Error("expected the matching code to be marked used")
	}
}

func TestValidateLoginCodeShouldNotAuthenticateTheSameCodeTwice(t *testing.T) {
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	if _, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456"); err != nil {
		t.Fatalf("first validation: unexpected error: %v", err)
	}

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode for a reused code, got %v", err)
	}
}

func TestValidateLoginCodeShouldReturnErrInvalidCodeForAWrongCode(t *testing.T) {
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "000000")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode for a wrong code, got %v", err)
	}
}

func TestValidateLoginCodeShouldReturnErrInvalidCodeForAnExpiredCode(t *testing.T) {
	fs := newStoreWithLoginCode("123456", 11*time.Minute, nil)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode for an expired code, got %v", err)
	}
}

func TestValidateLoginCodeShouldReturnErrInvalidCodeForAnAlreadyUsedCode(t *testing.T) {
	usedAt := time.Now().UTC()
	fs := newStoreWithLoginCode("123456", time.Minute, &usedAt)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode for an already-used code, got %v", err)
	}
}

// TestValidateLoginCodeShouldReturnErrInvalidCodeForACodeIssuedExactlyAtTheValidityWindowBoundary
// locks in the exact-boundary semantics documented in the spec's Design
// Notes: a code issued exactly codeValidityWindow ago is expired.
// ValidateLoginCode always computes its cutoff at call time, strictly after
// this fixture's IssuedAt was stamped, so IssuedAt can never come out
// "after" that later cutoff - this holds deterministically, with no timing
// margin needed, unlike the session-timeout boundary below.
func TestValidateLoginCodeShouldReturnErrInvalidCodeForACodeIssuedExactlyAtTheValidityWindowBoundary(t *testing.T) {
	fs := newStoreWithLoginCode("123456", codeValidityWindow, nil)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode for a code issued exactly at the %v validity window boundary, got %v", codeValidityWindow, err)
	}
}

func TestValidateLoginCodeShouldReturnErrInvalidCodeWhenTheStoreReportsTheCodeWasAlreadyClaimedDuringUpdate(t *testing.T) {
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	fs.markUsedErr = store.ErrLoginCodeAlreadyUsed
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode when the store reports the code was already claimed during the update (a lost redemption race), got %v", err)
	}
}

func TestValidateLoginCodeShouldReturnErrInvalidCodeForAnUnregisteredEmail(t *testing.T) {
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), "stranger@example.com", "123456")

	if !errors.Is(err, ErrInvalidCode) {
		t.Errorf("expected ErrInvalidCode for an unregistered email, got %v", err)
	}
}

func TestValidateLoginCodeShouldNotMarkAnyCodeUsedWhenTheSubmittedCodeIsWrong(t *testing.T) {
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	if _, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "000000"); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("expected ErrInvalidCode, got %v", err)
	}

	if fs.codes[0].UsedAt != nil {
		t.Error("expected the unrelated stored code to remain unused after a failed attempt")
	}
}

func TestValidateLoginCodeShouldPropagateStoreLookupErrorsOtherThanNotFound(t *testing.T) {
	wantErr := errors.New("boom: database unavailable")
	fs := &fakeStore{lookupErr: wantErr}
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected an error wrapping %v, got %v", wantErr, err)
	}
}

func TestValidateLoginCodeShouldPropagateUnusedLoginCodesLookupErrors(t *testing.T) {
	wantErr := errors.New("boom: query failed")
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	fs.unusedErr = wantErr
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected an error wrapping %v, got %v", wantErr, err)
	}
}

func TestValidateLoginCodeShouldPropagateMarkUsedErrors(t *testing.T) {
	wantErr := errors.New("boom: update failed")
	fs := newStoreWithLoginCode("123456", time.Minute, nil)
	fs.markUsedErr = wantErr
	svc := NewService(fs, &fakeMailer{}, testSessionSecret)

	_, err := svc.ValidateLoginCode(context.Background(), matchedEmail, "123456")

	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected an error wrapping %v, got %v", wantErr, err)
	}
}

func TestEncodeSessionThenDecodeSessionShouldRoundTripTheSameParticipantID(t *testing.T) {
	svc := NewService(newMatchedFakeStore(), &fakeMailer{}, testSessionSecret)
	want := Session{ParticipantID: "participant-1", IssuedAt: time.Now().UTC().Add(-5 * time.Minute)}

	token := svc.EncodeSession(want)

	got, err := svc.DecodeSession(token)
	if err != nil {
		t.Fatalf("unexpected error decoding: %v", err)
	}

	if got.ParticipantID != want.ParticipantID {
		t.Errorf("expected ParticipantID %q, got %q", want.ParticipantID, got.ParticipantID)
	}
	// The token format only carries whole-second precision (design notes),
	// so compare at that granularity rather than requiring an exact Equal.
	if !got.IssuedAt.Equal(want.IssuedAt.Truncate(time.Second)) {
		t.Errorf("expected IssuedAt %v, got %v", want.IssuedAt.Truncate(time.Second), got.IssuedAt)
	}
}

func TestDecodeSessionShouldReturnErrSessionExpiredWhenIssuedAtIsOlderThan30Minutes(t *testing.T) {
	svc := NewService(newMatchedFakeStore(), &fakeMailer{}, testSessionSecret)
	token := svc.EncodeSession(Session{ParticipantID: "participant-1", IssuedAt: time.Now().UTC().Add(-31 * time.Minute)})

	_, err := svc.DecodeSession(token)

	if !errors.Is(err, ErrSessionExpired) {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

func TestDecodeSessionShouldSucceedWhenIssuedAtIsWithin30Minutes(t *testing.T) {
	svc := NewService(newMatchedFakeStore(), &fakeMailer{}, testSessionSecret)
	token := svc.EncodeSession(Session{ParticipantID: "participant-1", IssuedAt: time.Now().UTC().Add(-29 * time.Minute)})

	if _, err := svc.DecodeSession(token); err != nil {
		t.Errorf("expected no error for a session within the 30-minute window, got %v", err)
	}
}

// TestDecodeSessionShouldSucceedWhenIssuedAtIsAtTheSessionTimeoutBoundary locks
// in the exact-boundary semantics documented in the spec's Design Notes: a
// session is expired only once elapsed time is strictly greater than
// sessionTimeout, so a session that is still at (or just under) exactly that
// many minutes old must remain valid. A real, non-mocked clock can never
// observe an elapsed duration of precisely sessionTimeout at the instant
// DecodeSession runs - any real time passing between building the fixture and
// calling DecodeSession only pushes the session further past the boundary,
// never earlier - so this fixture backs off by a small, deterministic margin
// to stay reliably on the valid side without flaking under CI load, while
// still exercising the boundary far more tightly than the existing
// 29-minute case above. IssuedAt is truncated to a whole second first since
// the token format only carries whole-second precision (design notes) -
// otherwise EncodeSession's own truncation could silently eat into the
// margin and flip the result to expired.
func TestDecodeSessionShouldSucceedWhenIssuedAtIsAtTheSessionTimeoutBoundary(t *testing.T) {
	const boundaryMargin = time.Second
	now := time.Now().UTC().Truncate(time.Second)
	svc := NewService(newMatchedFakeStore(), &fakeMailer{}, testSessionSecret)
	token := svc.EncodeSession(Session{ParticipantID: "participant-1", IssuedAt: now.Add(-(sessionTimeout - boundaryMargin))})

	if _, err := svc.DecodeSession(token); err != nil {
		t.Errorf("expected no error for a session at the %v timeout boundary, got %v", sessionTimeout, err)
	}
}

func TestDecodeSessionShouldReturnErrorForATamperedToken(t *testing.T) {
	svc := NewService(newMatchedFakeStore(), &fakeMailer{}, testSessionSecret)
	token := svc.EncodeSession(Session{ParticipantID: "participant-1", IssuedAt: time.Now().UTC()})

	tampered := token[:len(token)-1] + "x"
	if tampered == token {
		tampered = token[:len(token)-1] + "y"
	}

	_, err := svc.DecodeSession(tampered)

	if err == nil {
		t.Fatal("expected an error for a tampered token")
	}
	if errors.Is(err, ErrSessionExpired) {
		t.Error("expected a tampered token to fail signature verification, not expiry")
	}
}

func TestDecodeSessionShouldReturnErrorWhenSignedWithADifferentSecret(t *testing.T) {
	signer := NewService(newMatchedFakeStore(), &fakeMailer{}, "secret-a")
	verifier := NewService(newMatchedFakeStore(), &fakeMailer{}, "secret-b")
	token := signer.EncodeSession(Session{ParticipantID: "participant-1", IssuedAt: time.Now().UTC()})

	if _, err := verifier.DecodeSession(token); err == nil {
		t.Error("expected an error when decoding a token signed with a different secret")
	}
}

func TestDecodeSessionShouldReturnErrorForAMalformedToken(t *testing.T) {
	svc := NewService(newMatchedFakeStore(), &fakeMailer{}, testSessionSecret)

	_, err := svc.DecodeSession("not-a-valid-token")

	if err == nil {
		t.Fatal("expected an error for a malformed token")
	}
}
