package scoring

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// internalPrefix is the import path prefix of this module's internal
// packages.
const internalPrefix = "github.com/sommerfeld-io/fantasy-hockey/internal/"

func TestScoringShouldImportNoInternalPackageButStore(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list package files: %v", err)
	}

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
			if strings.HasPrefix(path, internalPrefix) && path != internalPrefix+"store" {
				t.Errorf("%s imports %s; scoring may import only internal/store", file, path)
			}
		}
	}
}
