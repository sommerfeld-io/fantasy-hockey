package web

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRulesHTMLShouldRenderTheEmbeddedMarkdownToHTML(t *testing.T) {
	for _, want := range []string{"<h1>Game Rules</h1>", "<h2>Scoring</h2>", "<table>"} {
		if !strings.Contains(string(rulesHTML), want) {
			t.Errorf("expected rulesHTML to contain %q, got %q", want, rulesHTML)
		}
	}
}

func TestRulesHTMLShouldNotContainRawMarkdownSyntax(t *testing.T) {
	for _, unwanted := range []string{"## ", "**"} {
		if strings.Contains(string(rulesHTML), unwanted) {
			t.Errorf("expected rulesHTML to contain no raw markdown syntax %q, got %q", unwanted, rulesHTML)
		}
	}
}

// repoRootForRulesTest resolves the repository root from this file's own
// path (three ".." up from src/internal/web/, mirroring
// internal/store/yamllint_test.go's own repoRootForYamllintTest), so this
// test can read docs/game-rules.md directly - outside the tree this
// package's embed directive reaches - without depending on the working
// directory a test binary happens to run from.
func repoRootForRulesTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to report this file's path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

// TestEmbeddedGameRulesShouldMatchTheCanonicalSource covers a review
// finding (epic-8 retrospective): docs/game-rules.md is the canonical
// source (rules.go's own doc comment), and rules/game-rules.md is a
// committed copy `task docs:embed-game-rules` is supposed to keep in sync
// - but that sync only runs inside the root `task lint`, which neither
// `task go:build`/`task go:run` (this repo's own agent-mandated
// verification loop) nor CI's `lint` job (which calls the docker-compose
// lint services directly, never `task lint`) ever invoke, and CI's
// pipeline triggers ignore `docs/**`/`**.md` entirely. Nothing previously
// caught the two files silently diverging; this test does, every time
// `task go:test` runs (locally, in CI's sonar-analysis job, and under this
// repo's own `task go:run` verification step).
func TestEmbeddedGameRulesShouldMatchTheCanonicalSource(t *testing.T) {
	canonicalPath := filepath.Join(repoRootForRulesTest(t), "docs", "game-rules.md")
	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("read canonical %s: %v", canonicalPath, err)
	}

	embedded, err := gameRulesFS.ReadFile("rules/game-rules.md")
	if err != nil {
		t.Fatalf("read embedded rules/game-rules.md: %v", err)
	}

	if string(canonical) != string(embedded) {
		t.Errorf("rules/game-rules.md has drifted from docs/game-rules.md - run `task docs:embed-game-rules` (or `task lint`) and commit the regenerated copy")
	}
}
