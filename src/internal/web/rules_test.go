package web

import (
	"bytes"
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

// TestRulesHTMLShouldNotContainLinks covers a review finding (epic-8
// retrospective, item 61): docs/game-rules.md's own header comment
// documents a "no links" convention, but extension.GFM (which
// gameRulesRenderer used to enable wholesale) bundles extension.Linkify,
// which auto-converts a bare URL or email in prose into a real <a> tag -
// no markdown link syntax required. Nothing previously caught that gap.
// This proves only that Linkify specifically stays out of
// gameRulesRenderer: explicit [text](url) markdown links and
// <https://...> autolinks are CommonMark core and would still render
// regardless of which goldmark extensions are enabled - this test says
// nothing about those.
//
// The first case asserts against the rendering of the real, canonical
// docs/game-rules.md (via the embedded rulesHTML), which only demonstrates
// the fix holds for today's file content - that file happens to contain no
// bare URL or email, so by itself it would still pass even if Linkify were
// reintroduced. The second case pins the fix independent of
// docs/game-rules.md's content by rendering a literal string containing a
// bare URL and email directly through gameRulesRenderer.Convert.
func TestRulesHTMLShouldNotContainLinks(t *testing.T) {
	if strings.Contains(string(rulesHTML), "<a ") {
		t.Errorf(`expected rulesHTML to contain no "<a " tags, got %q`, rulesHTML)
	}

	var buf bytes.Buffer
	if err := gameRulesRenderer.Convert([]byte("See https://example.com or foo@example.com."), &buf); err != nil {
		t.Fatalf("gameRulesRenderer.Convert: %v", err)
	}
	if strings.Contains(buf.String(), "<a ") {
		t.Errorf(`expected rendering a bare URL/email to contain no "<a " tags, got %q`, buf.String())
	}
}

// TestRulesHTMLShouldRenderLists covers the "Before the season"/"Playoffs"
// bullet list under "## The two phases" in docs/game-rules.md. Bullet-list
// parsing is CommonMark core, unaffected by any of gameRulesRenderer's
// Table/Strikethrough/TaskList/Linkify extensions, so this is a plain
// regression-safety check on the real embedded rulesHTML, not evidence
// tied to the Linkify removal (epic-8 retrospective, item 61).
func TestRulesHTMLShouldRenderLists(t *testing.T) {
	for _, want := range []string{"<ul>", "<li>"} {
		if !strings.Contains(string(rulesHTML), want) {
			t.Errorf("expected rulesHTML to contain %q, got %q", want, rulesHTML)
		}
	}
}

// TestGameRulesRendererShouldNeutralizeRawHTMLAndDangerousLinks covers a
// review finding (epic-8 retrospective, item 63): rulesHTML is wrapped in
// template.HTML, which disables html/template's usual auto-escaping, so
// its safety rests entirely on gameRulesRenderer's default Unsafe: false
// behavior - raw HTML (block and inline) gets stripped to the literal
// "<!-- raw HTML omitted -->" rather than passed through, and a dangerous
// URL scheme (goldmark's IsDangerousURL: javascript:, vbscript:, file:,
// most data:) gets neutralized to an empty href/src wherever a URL can
// appear - a link, an image, or an autolink - rather than rendered live.
// No live vulnerability exists today (docs/game-rules.md contains none of
// these), but nothing previously locked this invariant in, so a future
// goldmark option change (e.g. html.WithUnsafe()) could silently turn this
// into a stored-XSS sink with no test catching it. Every case here was
// confirmed against the real renderer before being asserted.
func TestGameRulesRendererShouldNeutralizeRawHTMLAndDangerousLinks(t *testing.T) {
	tests := []struct {
		name           string
		markdown       string
		mustNotContain string
		mustContain    string
	}{
		{"block-level raw HTML", "<script>alert('xss')</script>\n", "<script>", "<!-- raw HTML omitted -->"},
		{"inline raw HTML", "before <script>alert('xss')</script> after\n", "<script>", "<!-- raw HTML omitted -->"},
		{"javascript: link", "[click](javascript:alert(1))\n", "javascript:", `href=""`},
		{"vbscript: link", "[click](vbscript:alert(1))\n", "vbscript:", `href=""`},
		{"file: link", "[click](file:///etc/passwd)\n", "file:", `href=""`},
		{"data: link", "[click](data:text/html,evil)\n", "data:text/html", `href=""`},
		{"javascript: image src", "![x](javascript:alert(1))\n", "javascript:", `src=""`},
		// The autolink's own text renders as the literal string
		// "javascript:alert(1)" (safe, unclickable) - a bare "javascript:"
		// check would wrongly fail on that safe text, so this checks the
		// href attribute specifically.
		{"javascript: autolink", "<javascript:alert(1)>\n", `href="javascript`, `href=""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := gameRulesRenderer.Convert([]byte(tt.markdown), &buf); err != nil {
				t.Fatalf("gameRulesRenderer.Convert: %v", err)
			}
			got := buf.String()

			if strings.Contains(got, tt.mustNotContain) {
				t.Errorf("expected output to not contain %q, got %q", tt.mustNotContain, got)
			}
			if !strings.Contains(got, tt.mustContain) {
				t.Errorf("expected output to contain %q, got %q", tt.mustContain, got)
			}
		})
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
