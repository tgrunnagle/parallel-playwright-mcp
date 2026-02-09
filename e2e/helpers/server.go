// Package helpers provides utilities for E2E testing of the Playwright MCP Server.
package helpers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

const (
	// DefaultStartupTimeout is the maximum time to wait for the server to start.
	DefaultStartupTimeout = 30 * time.Second

	// DefaultHealthPollInterval is the interval between health check polls.
	DefaultHealthPollInterval = 100 * time.Millisecond

	// DefaultShutdownTimeout is the maximum time to wait for graceful shutdown.
	DefaultShutdownTimeout = 10 * time.Second
)

// TestServer manages the MCP server lifecycle for E2E tests.
// It starts the server as a subprocess and provides methods to
// wait for readiness and perform clean shutdown.
type TestServer struct {
	t          *testing.T
	cmd        *exec.Cmd
	host       string
	port       int
	binaryPath string
	healthURL  string
	mcpURL     string

	mu      sync.Mutex
	started bool
	stopped bool
	stdout  *os.File
	stderr  *os.File
}

// TestServerOption configures a TestServer.
type TestServerOption func(*TestServer)

// WithBinaryPath sets a custom binary path for the server.
func WithBinaryPath(path string) TestServerOption {
	return func(s *TestServer) {
		s.binaryPath = path
	}
}

// NewTestServer creates a new test server instance.
// The server is not started until Start is called.
func NewTestServer(t *testing.T, opts ...TestServerOption) *TestServer {
	t.Helper()

	port := getFreePort(t)
	host := "127.0.0.1"

	s := &TestServer{
		t:    t,
		host: host,
		port: port,
	}

	for _, opt := range opts {
		opt(s)
	}

	// Determine binary path
	if s.binaryPath == "" {
		s.binaryPath = findServerBinary(t)
	}

	s.healthURL = fmt.Sprintf("http://%s:%d/health", host, port)
	s.mcpURL = fmt.Sprintf("http://%s:%d/mcp", host, port)

	return s
}

// Start launches the server subprocess and waits for it to be ready.
// It polls the /health endpoint until it responds with 200 OK.
func (s *TestServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("server already started")
	}

	// Build command with environment variables to configure the server
	s.cmd = exec.CommandContext(ctx, s.binaryPath)
	s.cmd.Env = append(os.Environ(),
		fmt.Sprintf("MCP_HOST=%s", s.host),
		fmt.Sprintf("MCP_PORT=%d", s.port),
		"MCP_BROWSER_HEADLESS=true",
		"MCP_LOG_LEVEL=info", // Use info level for cleaner test output
	)

	// Capture stdout/stderr for debugging
	var err error
	s.stdout, err = os.CreateTemp("", "e2e-server-stdout-*.log")
	if err != nil {
		return fmt.Errorf("failed to create stdout file: %w", err)
	}
	s.stderr, err = os.CreateTemp("", "e2e-server-stderr-*.log")
	if err != nil {
		s.stdout.Close()
		return fmt.Errorf("failed to create stderr file: %w", err)
	}
	s.cmd.Stdout = s.stdout
	s.cmd.Stderr = s.stderr

	if err := s.cmd.Start(); err != nil {
		s.stdout.Close()
		s.stderr.Close()
		return fmt.Errorf("failed to start server: %w", err)
	}

	s.started = true

	// Wait for server to be ready by polling /health
	if err := s.waitForHealthy(ctx); err != nil {
		// Server failed to start - try to stop it and capture logs
		s.stopLocked()
		return err
	}

	return nil
}

// waitForHealthy polls the /health endpoint until it responds or context expires.
func (s *TestServer) waitForHealthy(ctx context.Context) error {
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(DefaultHealthPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for server to be healthy: %w", ctx.Err())
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.healthURL, nil)
			if err != nil {
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				// Server not ready yet, continue polling
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

// Stop terminates the server subprocess and cleans up resources.
// It first sends SIGINT for graceful shutdown, then SIGKILL if timeout expires.
func (s *TestServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

// stopLocked performs the actual stop logic. Caller must hold s.mu.
func (s *TestServer) stopLocked() error {
	if !s.started || s.stopped {
		return nil
	}

	s.stopped = true

	// Signal the process to terminate gracefully
	if s.cmd.Process != nil {
		// On Windows, we use Kill directly as Interrupt is not supported
		if runtime.GOOS == "windows" {
			s.cmd.Process.Kill()
		} else {
			s.cmd.Process.Signal(os.Interrupt)
		}
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited
	case <-time.After(DefaultShutdownTimeout):
		// Force kill
		if s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
		<-done
	}

	// Log output files for debugging if test failed
	if s.t.Failed() {
		s.logServerOutput()
	}

	// Clean up temp files
	if s.stdout != nil {
		name := s.stdout.Name()
		s.stdout.Close()
		os.Remove(name)
	}
	if s.stderr != nil {
		name := s.stderr.Name()
		s.stderr.Close()
		os.Remove(name)
	}

	return nil
}

// logServerOutput logs the server's stdout/stderr for debugging.
func (s *TestServer) logServerOutput() {
	if s.stdout != nil {
		s.stdout.Seek(0, 0)
		content, _ := os.ReadFile(s.stdout.Name())
		if len(content) > 0 {
			s.t.Logf("Server stdout:\n%s", string(content))
		}
	}
	if s.stderr != nil {
		s.stderr.Seek(0, 0)
		content, _ := os.ReadFile(s.stderr.Name())
		if len(content) > 0 {
			s.t.Logf("Server stderr:\n%s", string(content))
		}
	}
}

// MCPURL returns the base URL for MCP requests.
func (s *TestServer) MCPURL() string {
	return s.mcpURL
}

// HealthURL returns the URL for health checks.
func (s *TestServer) HealthURL() string {
	return s.healthURL
}

// Host returns the server host.
func (s *TestServer) Host() string {
	return s.host
}

// Port returns the server port.
func (s *TestServer) Port() int {
	return s.port
}

// Addr returns the server address in host:port format.
func (s *TestServer) Addr() string {
	return fmt.Sprintf("%s:%d", s.host, s.port)
}

// getFreePort returns an available port for testing.
func getFreePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}

// findServerBinary locates the server binary, building it if necessary.
func findServerBinary(t *testing.T) string {
	t.Helper()

	// Get the project root by finding the go.mod file
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Walk up to find the project root
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			t.Fatalf("Could not find project root (go.mod)")
		}
		projectRoot = parent
	}

	// Binary name depends on platform
	binaryName := "playwright-mcp-server"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	binaryPath := filepath.Join(projectRoot, "bin", binaryName)

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Build it
		t.Logf("Building server binary at %s", binaryPath)
		cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/server")
		cmd.Dir = projectRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to build server: %v", err)
		}
	}

	return binaryPath
}
