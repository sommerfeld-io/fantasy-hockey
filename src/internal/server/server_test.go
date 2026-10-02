package server

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestResolvePortShouldReturnTheDefaultWhenNoArgsAreGiven(t *testing.T) {
	got, err := ResolvePort(nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != DefaultPort {
		t.Errorf("expected the default port %d, got %d", DefaultPort, got)
	}
}

func TestResolvePortShouldUseTheLongFlagWhenGiven(t *testing.T) {
	got, err := ResolvePort([]string{"--port", "9091"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 9091 {
		t.Errorf("expected port 9091, got %d", got)
	}
}

func TestResolvePortShouldUseTheShortFlagWhenGiven(t *testing.T) {
	got, err := ResolvePort([]string{"-p", "9092"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 9092 {
		t.Errorf("expected port 9092, got %d", got)
	}
}

func TestResolvePortShouldFailOnAnInvalidValue(t *testing.T) {
	_, err := ResolvePort([]string{"--port", "not-a-number"})

	if err == nil {
		t.Fatal("expected an error for a non-numeric --port value")
	}
}

func TestAddrShouldFormatThePortAsAListenAddress(t *testing.T) {
	got := Addr(9091)

	if got != ":9091" {
		t.Errorf("expected %q, got %q", ":9091", got)
	}
}

// freePort asks the OS for a currently unused TCP port on 127.0.0.1.
func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// waitUntilListening polls port until a TCP connection succeeds or t fails
// the test after a short deadline.
func waitUntilListening(t *testing.T, port int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for the server to listen on port %d", port)
}

func TestRunShouldServeOnTheGivenPortAndLogIt(t *testing.T) {
	port := freePort(t)

	var logs bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(restore) })

	ctx, cancel := context.WithCancel(context.Background())
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	done := make(chan error, 1)
	go func() { done <- Run(ctx, port, handler) }()

	waitUntilListening(t, port)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatalf("unexpected error requesting the server: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", resp.StatusCode)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("unexpected error from Run: %v", err)
	}

	wantLog := fmt.Sprintf("port=%d", port)
	if !strings.Contains(logs.String(), wantLog) {
		t.Errorf("expected startup log to contain %q, got %q", wantLog, logs.String())
	}
}

func TestRunShouldReturnAnErrorWhenThePortIsAlreadyInUse(t *testing.T) {
	port := freePort(t)
	occupied, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("occupy port %d: %v", port, err)
	}
	defer occupied.Close()

	err = Run(context.Background(), port, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	if err == nil {
		t.Fatal("expected an error when the port is already in use")
	}
}
