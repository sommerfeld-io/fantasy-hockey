package web

import (
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
