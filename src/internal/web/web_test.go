package web

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// rfc1123Pattern matches the timestamp format handleHome renders.
var rfc1123Pattern = regexp.MustCompile(`[A-Za-z]{3}, \d{2} [A-Za-z]{3} \d{4} \d{2}:\d{2}:\d{2} [A-Za-z]{3,4}`)

func TestNewServerShouldReturnOKForTheHomePage(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	NewServer().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestNewServerShouldShowTheApplicationNameOnTheHomePage(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	NewServer().ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Fantasy Hockey") {
		t.Errorf("expected body to contain %q, got %q", "Fantasy Hockey", rec.Body.String())
	}
}

func TestNewServerShouldShowTheCurrentDateAndTimeOnTheHomePage(t *testing.T) {
	before := time.Now().UTC()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	NewServer().ServeHTTP(rec, req)

	after := time.Now().UTC()
	match := rfc1123Pattern.FindString(rec.Body.String())
	if match == "" {
		t.Fatalf("expected body to contain an RFC1123 timestamp, got %q", rec.Body.String())
	}

	got, err := time.Parse(time.RFC1123, match)
	if err != nil {
		t.Fatalf("failed to parse timestamp %q: %v", match, err)
	}
	if got.Before(before.Add(-time.Second)) || got.After(after.Add(time.Second)) {
		t.Errorf("expected the shown timestamp to be close to the current time, got %v", got)
	}
}

func TestNewServerShouldReturn404ForAnUnknownPath(t *testing.T) {
	req := httptest.NewRequest("GET", "/unknown", nil)
	rec := httptest.NewRecorder()

	NewServer().ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("expected status 404 for an unknown path, got %d", rec.Code)
	}
}
