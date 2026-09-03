package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunShouldPrintTheValueReturnedByNow(t *testing.T) {
	var buf bytes.Buffer
	stubNow := func() string { return "2026-09-03T12:00:00Z" }

	run(&buf, stubNow)

	if !strings.Contains(buf.String(), "2026-09-03T12:00:00Z") {
		t.Errorf("expected output to contain the value returned by now, got %q", buf.String())
	}
}

func TestRunShouldNotPrintAnUnrelatedValue(t *testing.T) {
	var buf bytes.Buffer
	stubNow := func() string { return "2026-09-03T12:00:00Z" }

	run(&buf, stubNow)

	if strings.Contains(buf.String(), "not-the-configured-value") {
		t.Errorf("expected output not to contain an unrelated value, got %q", buf.String())
	}
}

func TestRunShouldTerminateOutputWithANewline(t *testing.T) {
	var buf bytes.Buffer
	stubNow := func() string { return "2026-09-03T12:00:00Z" }

	run(&buf, stubNow)

	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected output to end with a newline, got %q", buf.String())
	}
}
