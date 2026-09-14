package main

import (
	"testing"

	"github.com/sommerfeld-io/fantasy-hockey/internal/server"
)

func TestResolveConfigShouldApplyDefaultsWithNoArgsOrEnv(t *testing.T) {
	t.Setenv("DATA_FILE", "")

	cfg, err := resolveConfig(nil)
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.port != server.DefaultPort {
		t.Errorf("expected default port %d, got %d", server.DefaultPort, cfg.port)
	}
	if cfg.dataFile != defaultDataFile {
		t.Errorf("expected default data file %q, got %q", defaultDataFile, cfg.dataFile)
	}
}

func TestResolveConfigShouldParsePortAndDataFileFlagsTogetherOnTheSharedFlagSet(t *testing.T) {
	t.Setenv("DATA_FILE", "")

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

	cfg, err := resolveConfig(nil)
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.dataFile != "/tmp/env.yml" {
		t.Errorf("expected data file %q, got %q", "/tmp/env.yml", cfg.dataFile)
	}
}

func TestResolveConfigShouldPreferDataFileFlagOverEnvWhenBothAreSet(t *testing.T) {
	t.Setenv("DATA_FILE", "/tmp/env.yml")

	cfg, err := resolveConfig([]string{"--data-file=/tmp/flag.yml"})
	if err != nil {
		t.Fatalf("resolveConfig returned error: %v", err)
	}
	if cfg.dataFile != "/tmp/flag.yml" {
		t.Errorf("expected the --data-file flag to win over DATA_FILE, got %q", cfg.dataFile)
	}
}
