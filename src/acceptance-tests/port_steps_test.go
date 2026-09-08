package acceptance_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/sommerfeld-io/fantasy-hockey/internal/server"
	"github.com/sommerfeld-io/fantasy-hockey/internal/web"
)

// portScenarioState holds the fixtures and results for one port-configuration
// scenario. A fresh instance is created per scenario so state never leaks
// between runs.
type portScenarioState struct {
	port   int
	logs   *bytes.Buffer
	cancel context.CancelFunc
	done   chan error
}

func newPortScenarioState() *portScenarioState {
	return &portScenarioState{}
}

func (s *portScenarioState) close() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
}

// theApplicationStartsWithArguments resolves the port from the given
// command-line arguments (space-separated, e.g. "--port 19091") and starts
// the real production server.Run against internal/web's handler, capturing
// its startup log so a later step can assert on it.
func (s *portScenarioState) theApplicationStartsWithArguments(argLine string) error {
	var args []string
	if strings.TrimSpace(argLine) != "" {
		args = strings.Fields(argLine)
	}

	port, err := server.ResolvePort(args)
	if err != nil {
		return fmt.Errorf("resolve port: %w", err)
	}
	s.port = port

	s.logs = &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewTextHandler(s.logs, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan error, 1)
	go func() { s.done <- server.Run(ctx, port, web.NewServer()) }()

	return s.waitUntilListening()
}

func (s *portScenarioState) theApplicationStartsWithNoArguments() error {
	return s.theApplicationStartsWithArguments("")
}

// waitUntilListening polls the resolved port until a TCP connection succeeds
// or a short deadline passes.
func (s *portScenarioState) waitUntilListening() error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.port), 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for the server to listen on port %d", s.port)
}

func (s *portScenarioState) itListensOnPort(port int) error {
	if s.port != port {
		return fmt.Errorf("expected the application to resolve port %d, got %d", port, s.port)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", s.port))
	if err != nil {
		return fmt.Errorf("get http://127.0.0.1:%d/: %w", s.port, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected HTTP 200 from port %d, got %d", s.port, resp.StatusCode)
	}
	return nil
}

func (s *portScenarioState) itLogsThePortItIsListeningOn() error {
	want := fmt.Sprintf("port=%d", s.port)
	if !strings.Contains(s.logs.String(), want) {
		return fmt.Errorf("expected the startup log to clearly show %q, got %q", want, s.logs.String())
	}
	return nil
}

// InitializePortScenario registers the port-configuration step definitions
// with GoDog.
func InitializePortScenario(ctx *godog.ScenarioContext) {
	s := newPortScenarioState()
	ctx.After(func(gctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		s.close()
		return gctx, nil
	})

	ctx.Step(`^the application starts with no arguments$`, s.theApplicationStartsWithNoArguments)
	ctx.Step(`^the application starts with arguments "([^"]*)"$`, s.theApplicationStartsWithArguments)
	ctx.Step(`^it listens on port (\d+)$`, s.itListensOnPort)
	ctx.Step(`^it logs the port it is listening on$`, s.itLogsThePortItIsListeningOn)
}
