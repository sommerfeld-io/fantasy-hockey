package store

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// dockerCommandTimeout bounds every docker invocation these tests make, so a
// hung pull or unreachable daemon fails fast with a legible per-test timeout
// instead of relying on go test's own overall binary timeout.
const dockerCommandTimeout = 60 * time.Second

// repoRootForYamllintTest returns this repo's root directory, found relative
// to this test file's own location (three levels up from
// src/internal/store/) rather than the process's working directory, so the
// result is correct regardless of where `go test` is invoked from.
func repoRootForYamllintTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to report this file's path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

// runYamllint checks every file in dir against the real cytopia/yamllint
// image and this repo's own .yamllint.yml - the same image and config
// task lint's lint-yaml service uses, never a reimplementation of its
// rules - failing t unless yamllint's exit code is 0 (its own convention:
// warnings alone still exit 0, only an error-level violation exits
// non-zero, matching task lint's non-strict gate).
func runYamllint(t *testing.T, dockerPath, dir string) {
	t.Helper()
	yamllintConfig := filepath.Join(repoRootForYamllintTest(t), ".yamllint.yml")

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, dockerPath, "run", "--rm", //nolint:gosec // dockerPath comes from exec.LookPath, not user input
		"-v", yamllintConfig+":/yamllint.yml:ro",
		"-v", dir+":/data:ro",
		"cytopia/yamllint:latest",
		"-c", "/yamllint.yml",
		"/data",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("yamllint reported error-level violations, or docker itself failed (image not cached/no network/daemon issue) - exit code convention: warnings alone still exit 0: %v\n%s", err, out)
	}
}

// dockerOrSkip returns the docker binary's path, skipping t when it isn't on
// PATH or its daemon isn't reachable, so `go test` still works for a
// contributor without a running Docker (rather than failing with a
// misleading "yamllint reported error-level violations" message); CI (which
// has Docker running) always exercises the yamllint tests for real.
func dockerOrSkip(t *testing.T) string {
	t.Helper()
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker not found on PATH, skipping yamllint compliance test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()
	if err := exec.CommandContext(ctx, dockerPath, "info").Run(); err != nil { //nolint:gosec // dockerPath comes from exec.LookPath, not user input
		t.Skip("docker daemon not reachable, skipping yamllint compliance test")
	}
	return dockerPath
}

// TestWriteLockedShouldProduceYamllintCompliantOutputForANestedListRow
// exercises the real store.writeLocked code path for both documented
// nested-list shapes that reproduce the bug this story fixes: team_ids (a
// division-playoff-teams row) and finalist_slugs (an award row) - a plain
// CreateLoginCode/SavePrediction-only file would reproduce neither.
func TestWriteLockedShouldProduceYamllintCompliantOutputForANestedListRow(t *testing.T) {
	dockerPath := dockerOrSkip(t)

	dir := t.TempDir()
	st, err := New(filepath.Join(dir, DataFileName))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	now := time.Now().UTC()

	if err := st.SaveDivisionPicks("basti", map[string][]string{
		"Atlantic": {"TOR", "BOS", "TBL"},
	}, nil, now); err != nil {
		t.Fatalf("SaveDivisionPicks() returned error: %v", err)
	}
	if err := st.SaveAwardPicks("basti", map[string][]string{
		AwardHart: {"a", "b", "c"},
	}, now); err != nil {
		t.Fatalf("SaveAwardPicks() returned error: %v", err)
	}

	runYamllint(t, dockerPath, dir)
}

// TestWriteLockedShouldProduceYamllintCompliantOutputForARowWithNoNestedLists
// is the should-not counterpart of the nested-list test above: proves the
// CompactSeqIndent switch (which changes every sequence's indent, not only
// nested ones) doesn't introduce a new problem for the common case of a
// document with no nested-list rows at all.
func TestWriteLockedShouldProduceYamllintCompliantOutputForARowWithNoNestedLists(t *testing.T) {
	dockerPath := dockerOrSkip(t)

	dir := t.TempDir()
	st, err := New(filepath.Join(dir, DataFileName))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	now := time.Now().UTC()
	if err := st.CreateLoginCode("basti", "deadbeefcafe", now); err != nil {
		t.Fatalf("CreateLoginCode() returned error: %v", err)
	}
	if err := st.SavePrediction("basti", KindCupChampion, "TOR", now); err != nil {
		t.Fatalf("SavePrediction() returned error: %v", err)
	}

	runYamllint(t, dockerPath, dir)
}
