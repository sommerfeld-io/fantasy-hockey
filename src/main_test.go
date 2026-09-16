package main

import (
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
