package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/server"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
)

// testSessionSecret is set on every test that needs resolveConfig to
// succeed but doesn't itself exercise SESSION_SECRET's value.
const testSessionSecret = "test-session-secret"

func TestResolveConfigShouldApplyDefaultsWithNoArgsOrEnv(t *testing.T) {
	t.Setenv("DATA_FILE", "")
	t.Setenv("SESSION_SECRET", testSessionSecret)

	cfg, err := resolveConfig(nil)
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.port != server.DefaultPort {
		t.Errorf("expected default port %d, got %d", server.DefaultPort, cfg.port)
	}
	if cfg.dataFile != store.DataFileName {
		t.Errorf("expected default data file %q, got %q", store.DataFileName, cfg.dataFile)
	}
	if cfg.secret != testSessionSecret {
		t.Errorf("expected secret %q, got %q", testSessionSecret, cfg.secret)
	}
}

func TestResolveConfigShouldParsePortAndDataFileFlagsTogetherOnTheSharedFlagSet(t *testing.T) {
	t.Setenv("DATA_FILE", "")
	t.Setenv("SESSION_SECRET", testSessionSecret)

	tests := []struct {
		name     string
		args     []string
		wantPort int
	}{
		{"long form --port", []string{"--port=9090", "--data-file=/tmp/x.yml"}, 9090},
		{"shorthand -p", []string{"-p", "9091", "--data-file=/tmp/x.yml"}, 9091},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := resolveConfig(tt.args)
			if err != nil {
				t.Fatalf("resolveConfig returned error: %v", err)
			}
			if cfg.port != tt.wantPort {
				t.Errorf("expected port %d, got %d", tt.wantPort, cfg.port)
			}
			if cfg.dataFile != "/tmp/x.yml" {
				t.Errorf("expected data file %q, got %q", "/tmp/x.yml", cfg.dataFile)
			}
		})
	}
}

func TestResolveConfigShouldFallBackToDataFileEnvWhenNoFlagIsGiven(t *testing.T) {
	t.Setenv("DATA_FILE", "/tmp/env.yml")
	t.Setenv("SESSION_SECRET", testSessionSecret)

	cfg, err := resolveConfig(nil)
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.dataFile != "/tmp/env.yml" {
		t.Errorf("expected data file %q, got %q", "/tmp/env.yml", cfg.dataFile)
	}
}

func TestResolveConfigShouldReturnAnErrorForAnUnrecognizedFlag(t *testing.T) {
	t.Setenv("SESSION_SECRET", testSessionSecret)

	if _, err := resolveConfig([]string{"--not-a-real-flag"}); err == nil {
		t.Fatal("expected an error for an unrecognized flag, got nil")
	}
}

func TestResolveConfigShouldPreferDataFileFlagOverEnvWhenBothAreSet(t *testing.T) {
	t.Setenv("DATA_FILE", "/tmp/env.yml")
	t.Setenv("SESSION_SECRET", testSessionSecret)

	cfg, err := resolveConfig([]string{"--data-file=/tmp/flag.yml"})
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.dataFile != "/tmp/flag.yml" {
		t.Errorf("expected the --data-file flag to win over DATA_FILE, got %q", cfg.dataFile)
	}
}

func TestResolveConfigShouldReturnAnErrorWhenSessionSecretIsUnset(t *testing.T) {
	t.Setenv("SESSION_SECRET", "")

	if _, err := resolveConfig(nil); err == nil {
		t.Fatal("expected an error when SESSION_SECRET is unset, got nil")
	}
}

func TestResolveConfigShouldReturnAnErrorWhenSessionSecretIsWhitespaceOnly(t *testing.T) {
	t.Setenv("SESSION_SECRET", "   ")

	if _, err := resolveConfig(nil); err == nil {
		t.Fatal("expected an error when SESSION_SECRET is whitespace-only, got nil")
	}
}

func TestOpenStoreShouldWarnAboutAMalformedResultAndStillSucceed(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	seed := "season: \"2026-27\"\nresults:\n    presidents_trophy: YYY\n    stanley_cup_winner: XXX\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	st, err := openStore(path, logger)

	if err != nil || st == nil {
		t.Fatalf("openStore = %v, %v, want a store and no error", st, err)
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "XXX") {
		t.Errorf("expected a warning naming the bad entry, got %q", logs.String())
	}
	if got := strings.Count(logs.String(), "level=WARN"); got != 2 {
		t.Errorf("expected one warning per bad entry (2), got %d in %q", got, logs.String())
	}
}

func TestOpenStoreShouldNotWarnForAWellFormedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	seed := "season: \"2026-27\"\nteams:\n    - {id: FLA, name: Florida Panthers, conference: Eastern, division: Atlantic}\nresults:\n    presidents_trophy: FLA\n    stanley_cup_winner: FLA\n"
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	if _, err := openStore(path, logger); err != nil {
		t.Fatalf("openStore returned error: %v", err)
	}
	if logs.Len() != 0 {
		t.Errorf("expected no log output, got %q", logs.String())
	}
}

func TestOpenStoreShouldReturnAnErrorForAnUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), store.DataFileName)
	if err := os.WriteFile(path, []byte("not: valid: yaml: at all"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := openStore(path, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))); err == nil {
		t.Fatal("expected an error for invalid YAML, got nil")
	}
}
