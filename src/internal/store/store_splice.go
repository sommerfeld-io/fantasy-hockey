package store

import (
	"regexp"
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// lenientTopLevelKeys are the two top-level sections whose shape errors AC2
// tolerates instead of aborting startup, confined by line range - never a
// general "the app never fails to parse" policy (Boundaries & Constraints).
// These names correspond to store_results.go's results/AwardFinalists handling.
var lenientTopLevelKeys = []string{"results", "award_finalists"}

// typeErrorLinePattern matches a *yaml.TypeError message's leading line
// number (e.g. "line 7: cannot unmarshal ..." - Design Notes' "stable,
// long-documented line %d: message format").
var typeErrorLinePattern = regexp.MustCompile(`^line (\d+):`)

// parseErrorLine extracts the line number msg (one entry of a
// *yaml.TypeError's Errors) leads with, if it's in the expected format.
func parseErrorLine(msg string) (int, bool) {
	m := typeErrorLinePattern.FindStringSubmatch(msg)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// duplicateKeyErrorSuffix is go.yaml.in/yaml/v3's stable, long-documented
// suffix for a duplicate-mapping-key *yaml.TypeError entry (decode.go:
// "line %d: mapping key %#v already defined at line %d") - distinct from
// an ordinary "cannot unmarshal ..." type-mismatch message.
const duplicateKeyErrorSuffix = "already defined at line"

// isDuplicateKeyError reports whether msg (one entry of a *yaml.TypeError's
// Errors) is a duplicate-key error rather than an ordinary type mismatch -
// epic-7 retro finding: unlike a type mismatch, a duplicate key zeroes
// every field at that same mapping level, not just the duplicated one, so
// a tolerated duplicate-key message needs its own caveat about siblings.
func isDuplicateKeyError(msg string) bool {
	return strings.Contains(msg, duplicateKeyErrorSuffix)
}

// namedLenientRange is one of results:'s or award_finalists:'s own line
// ranges in the raw document, tagged with which top-level key it's for - so
// a tolerated shape error's warning can name which section it came from,
// not just that it was tolerated (review finding, spec-7-4).
type namedLenientRange struct {
	key        string
	start, end int
}

// lenientRanges returns raw's line range for each of lenientTopLevelKeys
// that's actually present - shared by typeErrorConfinedToLenientSections
// (which only needs the ranges) and sectionForLine (which also needs to
// know which key each range belongs to).
func lenientRanges(raw *yaml.Node) []namedLenientRange {
	mapping := topLevelMapping(raw)
	var ranges []namedLenientRange
	for _, key := range lenientTopLevelKeys {
		if node, ok := mappingValue(mapping, key); ok {
			ranges = append(ranges, namedLenientRange{key: key, start: node.Line, end: maxLine(node)})
		}
	}
	return ranges
}

// typeErrorConfinedToLenientSections reports whether every line typeErr
// names falls within results:'s or award_finalists:'s own line range in
// raw's top-level mapping (AC2) - a message whose line can't be parsed, or
// that falls outside both ranges (including when neither section is
// present in raw at all), fails the check, so New returns the original
// error unchanged.
func typeErrorConfinedToLenientSections(typeErr *yaml.TypeError, raw *yaml.Node) bool {
	ranges := lenientRanges(raw)
	for _, msg := range typeErr.Errors {
		line, ok := parseErrorLine(msg)
		if !ok || sectionForLine(line, ranges) == "" {
			return false
		}
	}
	return true
}

// sectionForLine returns the key of the range line falls within, or "" if
// it falls within none - used once typeErrorConfinedToLenientSections has
// already confirmed every line is confined, to label each tolerated
// message with the specific section it came from.
func sectionForLine(line int, ranges []namedLenientRange) string {
	for _, r := range ranges {
		if line >= r.start && line <= r.end {
			return r.key
		}
	}
	return ""
}

// maxLine returns the greatest Line value anywhere in n's own subtree
// (itself and every descendant) - a mapping or sequence node's own Line
// only marks where its content starts, so this is needed to find where it
// ends, bounding a section's full line range.
func maxLine(n *yaml.Node) int {
	max := n.Line
	for _, c := range n.Content {
		if l := maxLine(c); l > max {
			max = l
		}
	}
	return max
}

// topLevelMapping returns n's top-level mapping node: the DocumentNode's
// first child when n was produced by unmarshaling a full file's bytes, or n
// itself when n was produced by Node.Encode (which never wraps its result
// in a DocumentNode, unlike Unmarshal).
func topLevelMapping(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

// mappingValue returns key's value node within mapping's Content, and
// whether key was present at all. mapping being nil or not a MappingNode
// (a shape error already reported elsewhere) reports key absent.
func mappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}

// mappingKeys returns every key in mapping's Content, in the file's own
// order - already deterministic (unlike a Go map's iteration order), since
// it comes straight from the parsed document. mapping being nil or not a
// MappingNode returns nil.
func mappingKeys(mapping *yaml.Node) []string {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(mapping.Content)/2)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keys = append(keys, mapping.Content[i].Value)
	}
	return keys
}

// spliceNamedValueLocked replaces key's value node within mapping's Content
// with value, appending a new "key: value" pair at mapping's end when key
// isn't present yet (a hand-seeded fixture, or a pre-spec-7-4 file, that
// omits login_codes: or predictions: entirely still needs a write to
// succeed). It returns a restore closure that undoes exactly this splice -
// reassigning the old value node back in place, or truncating the appended
// pair - so writeLocked can roll every splice back in reverse order if a
// later step fails. Callers must hold s.mu for writing.
func spliceNamedValueLocked(mapping *yaml.Node, key string, value *yaml.Node) (restore func()) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			old := mapping.Content[i+1]
			mapping.Content[i+1] = value
			return func() { mapping.Content[i+1] = old }
		}
	}

	var keyNode yaml.Node
	keyNode.SetString(key)
	mapping.Content = append(mapping.Content, &keyNode, value)
	return func() { mapping.Content = mapping.Content[:len(mapping.Content)-2] }
}
