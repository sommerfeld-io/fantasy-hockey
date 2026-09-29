package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// gameRulesRenderer renders GFM tables, strikethrough, and task lists -
// game-rules.md's scoring table needs the table extension, which goldmark's
// CommonMark core alone doesn't provide. Linkify, one of extension.GFM's
// four components, is deliberately left out: it would auto-convert a bare
// URL or email in prose into a real <a>/mailto: link, violating this page's
// documented "no links" convention (docs/game-rules.md's own header
// comment; epic-8 retrospective, item 61). This closes only that "bare
// URL/email in prose" vector - CommonMark core still supports explicit
// [text](url) link syntax and <https://...> autolinks regardless of which
// goldmark extensions are enabled; nothing here prevents those.
var gameRulesRenderer = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList))

// gameRulesFS embeds a generated copy of the game rules. docs/game-rules.md
// is the canonical source (edit it there); rules/game-rules.md is copied
// from it by `task docs:embed-game-rules` so go:embed - which can't reach
// outside this package's own directory tree - has something to embed. Since
// the copy is committed, neither the Dockerfile nor `task go:build` needs
// docs/ present to build the binary.
//
//go:embed rules/game-rules.md
var gameRulesFS embed.FS

// rulesHTML is rules/game-rules.md rendered to HTML once at package init -
// like templates in web.go, so a broken embed fails fast at startup rather
// than on the first request to the Rules tab. template.HTML disables
// html/template's usual auto-escaping, so this is safe only because
// gameRulesRenderer's default Unsafe: false rendering (goldmark v1.8.6,
// go.mod) strips raw HTML - block and inline alike - down to the literal
// comment "<!-- raw HTML omitted -->", and neutralizes a dangerous URL
// scheme (javascript:, vbscript:, file:, most data: - IsDangerousURL,
// goldmark's renderer/html package) wherever a URL can appear (link,
// image, autolink) down to an empty href/src - confirmed empirically
// (epic-8 retrospective, item 63) and locked in by
// TestGameRulesRendererShouldNeutralizeRawHTMLAndDangerousLinks
// (rules_test.go). The threat this guards is a hand-edit to
// docs/game-rules.md landing through a PR and a rebuild, not live runtime
// input (mustRenderGameRules's own comment below). Never call
// html.WithUnsafe() or enable any raw-HTML-permitting goldmark option on
// gameRulesRenderer without re-reviewing this invariant.
var rulesHTML = template.HTML(mustRenderGameRules())

// mustRenderGameRules reads and renders the embedded game rules markdown.
// Both failure modes here (a missing embed, a writer error from goldmark)
// can only be programmer errors caught at build/startup time, never runtime
// input - the embedded file is fixed at compile time.
func mustRenderGameRules() string {
	source, err := gameRulesFS.ReadFile("rules/game-rules.md")
	if err != nil {
		panic(fmt.Sprintf("web: read embedded game rules: %v", err))
	}
	var buf bytes.Buffer
	if err := gameRulesRenderer.Convert(source, &buf); err != nil {
		panic(fmt.Sprintf("web: render game rules markdown: %v", err))
	}
	return buf.String()
}
