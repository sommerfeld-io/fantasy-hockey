package standings

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// internalPrefix is the import path prefix of this module's internal
// packages.
const internalPrefix = "github.com/sommerfeld-io/fantasy-hockey/internal/"

// internalImports lists every internal/ package this package's non-test
// files import.
func internalImports(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list package files: %v", err)
	}

	var imports []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, imp := range parsed.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("unquote import %s in %s: %v", imp.Path.Value, file, err)
			}
			if strings.HasPrefix(path, internalPrefix) {
				imports = append(imports, path)
			}
		}
	}
	return imports
}

func TestStandingsShouldImportScoringForEveryPointValue(t *testing.T) {
	if !slices.Contains(internalImports(t), internalPrefix+"scoring") {
		t.Errorf("expected standings to import internal/scoring, the only home of point calculation")
	}
}
