package acceptance_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sommerfeld-io/fantasy-hockey/internal/auth"
	"github.com/sommerfeld-io/fantasy-hockey/internal/store"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// relativeDeadlinePattern parses the Gherkin-friendly deadline phrases the
// features' Given steps accept ("in 5 days", "in 3 hours", "1 day ago"),
// keeping every scenario's deadline relative to the moment it runs rather
// than a hardcoded date that would eventually go stale.
var relativeDeadlinePattern = regexp.MustCompile(`^(?:in (\d+) (hour|hours|day|days)|(\d+) (hour|hours|day|days) ago)$`)

// parseRelativeDeadline resolves phrase against now into an absolute UTC
// deadline.
func parseRelativeDeadline(phrase string, now time.Time) (time.Time, error) {
	m := relativeDeadlinePattern.FindStringSubmatch(phrase)
	if m == nil {
		return time.Time{}, fmt.Errorf("unrecognized relative deadline %q", phrase)
	}

	var amount, unit string
	sign := 1
	if m[1] != "" {
		amount, unit = m[1], m[2]
	} else {
		amount, unit = m[3], m[4]
		sign = -1
	}

	n, err := strconv.Atoi(amount)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse amount %q: %w", amount, err)
	}

	step := time.Hour
	if strings.HasPrefix(unit, "day") {
		step = 24 * time.Hour
	}

	return now.Add(time.Duration(sign*n) * step), nil
}

// seedHeader is the fantasy-hockey.yml preamble every seeded fixture starts
// with: the season plus the one player the scenario signs in as.
func seedHeader(playerID, playerName string) string {
	return fmt.Sprintf("season: \"2026-27\"\nplayers:\n    - id: %s\n      name: %s\n      email: basti@example.com\n", playerID, playerName)
}

// newScenarioDataFile creates a fresh temp directory named after prefix and
// returns the data file path inside it (not yet written).
func newScenarioDataFile(prefix string) string {
	dir, err := os.MkdirTemp("", "fantasy-hockey-"+prefix+"-*")
	if err != nil {
		panic(fmt.Sprintf("create temp dir: %v", err))
	}
	return filepath.Join(dir, store.DataFileName)
}

// writeSeededStore writes seed to dataFile and opens a store on it, so
// store.New's own parsing runs end-to-end against the seeded document.
func writeSeededStore(dataFile, seed string) (*store.Store, error) {
	if err := os.WriteFile(dataFile, []byte(seed), 0o600); err != nil {
		return nil, fmt.Errorf("seed data file: %w", err)
	}
	st, err := store.New(dataFile)
	if err != nil {
		return nil, fmt.Errorf("store.New: %w", err)
	}
	return st, nil
}

// newSeededStore bootstraps a store backed by seed in a new temp directory
// named after prefix, for scenarios whose fixture is fixed up front. It
// returns the data file's path so the caller can remove its directory with
// removeScenarioDataFile once the scenario ends.
func newSeededStore(prefix, seed string) (*store.Store, string) {
	dataFile := newScenarioDataFile(prefix)
	st, err := writeSeededStore(dataFile, seed)
	if err != nil {
		panic(err.Error())
	}
	return st, dataFile
}

// removeScenarioDataFile best-effort removes dataFile's temp directory.
func removeScenarioDataFile(dataFile string) {
	_ = os.RemoveAll(filepath.Dir(dataFile))
}

// lazyFixture is the embeddable scenario state for features whose Given
// steps keep appending fixtures before the first request: the data file,
// store and server are only built (ensureReady) when a step first needs a
// running server. seedBody supplies everything after seedHeader; afterStore,
// when non-nil, runs once the store exists but before the server starts
// (e.g. to save prior picks). Every request carries playerID's session
// cookie, signed with secret, and never follows a redirect.
type lazyFixture struct {
	dataFile   string
	playerID   string
	playerName string
	secret     string
	seedBody   func() (string, error)
	afterStore func(*store.Store) error

	st           *store.Store
	server       *httptest.Server
	lastStatus   int
	lastBody     string
	lastLocation string
}

// newLazyFixture creates a lazyFixture with its own temp directory named
// after prefix.
func newLazyFixture(prefix, playerID, playerName, secret string, seedBody func() (string, error), afterStore func(*store.Store) error) lazyFixture {
	return lazyFixture{
		dataFile:   newScenarioDataFile(prefix),
		playerID:   playerID,
		playerName: playerName,
		secret:     secret,
		seedBody:   seedBody,
		afterStore: afterStore,
	}
}

// ensureReady writes the seed, opens the store, runs afterStore and starts
// the real production web.NewServer handler - once per scenario.
func (f *lazyFixture) ensureReady() error {
	if f.server != nil {
		return nil
	}

	body, err := f.seedBody()
	if err != nil {
		return err
	}
	st, err := writeSeededStore(f.dataFile, seedHeader(f.playerID, f.playerName)+body)
	if err != nil {
		return err
	}
	f.st = st

	if f.afterStore != nil {
		if err := f.afterStore(st); err != nil {
			return err
		}
	}

	f.server = httptest.NewServer(web.NewServer(st, noopSender, f.secret))
	return nil
}

// close stops the server (if it was ever started) and removes the
// scenario's temp directory.
func (f *lazyFixture) close() {
	if f.server != nil {
		f.server.Close()
	}
	removeScenarioDataFile(f.dataFile)
}

// do requests method+path carrying the signed-in player's session cookie,
// with an optional urlencoded form body, without following any redirect,
// and records the result.
func (f *lazyFixture) do(method, path, body string) error {
	if err := f.ensureReady(); err != nil {
		return err
	}

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, f.server.URL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.AddCookie(auth.IssueSessionCookie(f.playerID, f.secret))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	f.lastStatus = resp.StatusCode
	f.lastLocation = resp.Header.Get("Location")
	f.lastBody = string(respBody)
	return nil
}
