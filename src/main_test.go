package main

import (
	"strings"
	"testing"
)

func fakeGetenv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func TestResolveDatabaseURLShouldPreferTheFlagWhenBothAreSet(t *testing.T) {
	getenv := fakeGetenv(map[string]string{"DATABASE_URL": "postgres://env"})

	got, err := resolveDatabaseURL([]string{"--database-url=postgres://flag"}, getenv)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "postgres://flag" {
		t.Errorf("expected the flag value to win, got %q", got)
	}
}

func TestResolveDatabaseURLShouldUseTheEnvVarWhenOnlyItIsSet(t *testing.T) {
	getenv := fakeGetenv(map[string]string{"DATABASE_URL": "postgres://env"})

	got, err := resolveDatabaseURL(nil, getenv)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "postgres://env" {
		t.Errorf("expected the env var value, got %q", got)
	}
}

func TestResolveDatabaseURLShouldUseTheFlagWhenOnlyItIsSet(t *testing.T) {
	getenv := fakeGetenv(nil)

	got, err := resolveDatabaseURL([]string{"--database-url=postgres://flag"}, getenv)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "postgres://flag" {
		t.Errorf("expected the flag value, got %q", got)
	}
}

func TestResolveDatabaseURLShouldFailFastWhenNeitherIsSet(t *testing.T) {
	getenv := fakeGetenv(nil)

	_, err := resolveDatabaseURL(nil, getenv)

	if err == nil {
		t.Fatal("expected an error when neither DATABASE_URL nor --database-url is set")
	}
}

func TestRequireEnvShouldReturnTheValueWhenSet(t *testing.T) {
	getenv := fakeGetenv(map[string]string{"SESSION_SECRET": "super-secret"})

	got, err := requireEnv(getenv, "SESSION_SECRET")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "super-secret" {
		t.Errorf("expected %q, got %q", "super-secret", got)
	}
}

func TestRequireEnvShouldFailFastWhenUnset(t *testing.T) {
	getenv := fakeGetenv(nil)

	_, err := requireEnv(getenv, "SESSION_SECRET")

	if err == nil {
		t.Fatal("expected an error when the environment variable is unset")
	}
	if !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Errorf("expected the error to name the missing variable, got %q", err.Error())
	}
}

func TestReadParticipantsShouldReturnAllThreeSlotsWhenFullySet(t *testing.T) {
	getenv := fakeGetenv(map[string]string{
		"PARTICIPANT_1_NAME": "Basti", "PARTICIPANT_1_EMAIL": "basti@example.com",
		"PARTICIPANT_2_NAME": "Sadl", "PARTICIPANT_2_EMAIL": "sadl@example.com",
		"PARTICIPANT_3_NAME": "Tobbi", "PARTICIPANT_3_EMAIL": "tobbi@example.com",
	})

	got, err := readParticipants(getenv)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(got))
	}
	if got[0].slot != 1 || got[0].name != "Basti" || got[0].email != "basti@example.com" {
		t.Errorf("unexpected first participant: %+v", got[0])
	}
}

func TestReadParticipantsShouldFailFastWhenAnyVarIsMissing(t *testing.T) {
	getenv := fakeGetenv(map[string]string{
		"PARTICIPANT_1_NAME": "Basti", "PARTICIPANT_1_EMAIL": "basti@example.com",
		"PARTICIPANT_2_NAME": "Sadl", // PARTICIPANT_2_EMAIL missing
		"PARTICIPANT_3_NAME": "Tobbi", "PARTICIPANT_3_EMAIL": "tobbi@example.com",
	})

	_, err := readParticipants(getenv)

	if err == nil {
		t.Fatal("expected an error when a PARTICIPANT_* variable is missing")
	}
	if !strings.Contains(err.Error(), "PARTICIPANT_2_EMAIL") {
		t.Errorf("expected the error to name the missing variable, got %q", err.Error())
	}
}

func TestReadParticipantsShouldFailFastWhenTwoSlotsShareAnEmail(t *testing.T) {
	getenv := fakeGetenv(map[string]string{
		"PARTICIPANT_1_NAME": "Basti", "PARTICIPANT_1_EMAIL": "same@example.com",
		"PARTICIPANT_2_NAME": "Sadl", "PARTICIPANT_2_EMAIL": "SAME@example.com",
		"PARTICIPANT_3_NAME": "Tobbi", "PARTICIPANT_3_EMAIL": "tobbi@example.com",
	})

	_, err := readParticipants(getenv)

	if err == nil {
		t.Fatal("expected an error when two participant slots share an email")
	}
}
