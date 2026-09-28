package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// gameRulesRenderer renders GFM (tables included) - game-rules.md's scoring
// table needs the table extension, which goldmark's CommonMark core alone
// doesn't provide.
var gameRulesRenderer = goldmark.New(goldmark.WithExtensions(extension.GFM))

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
// than on the first request to the Rules tab.
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
