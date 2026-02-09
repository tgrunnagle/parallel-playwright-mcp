# E2E Test Infrastructure

This directory contains end-to-end tests for the Playwright MCP Server. These tests validate complete workflows from MCP client connection through browser automation to session cleanup.

## Directory Structure

```
e2e/
├── README.md              # This file
├── e2e_test.go            # Main E2E test file with example tests
├── helpers/
│   ├── server.go          # Test server lifecycle management
│   ├── client.go          # MCP client for test interactions
│   └── fixtures.go        # Test fixture management
└── fixtures/
    ├── simple.html        # Basic HTML page for interaction tests
    ├── form.html          # Form elements for fill/submit tests
    └── navigation.html    # Multi-page navigation test fixture
```

## Running E2E Tests

### Prerequisites

1. **Install Playwright browsers** (if not already installed):
   ```bash
   task setup
   ```

2. **Build the server binary**:
   ```bash
   task build
   ```

### Running Tests

Run all E2E tests:
```bash
task test:e2e
```

Or run manually with Go:
```bash
go test -v -tags=e2e ./e2e/...
```

Run a specific test:
```bash
go test -v -tags=e2e -run TestServerStartsAndAcceptsConnections ./e2e/...
```

### Running Tests in Parallel

The E2E tests support parallel execution. Each test uses its own server instance on a dynamically allocated port:
```bash
go test -v -tags=e2e -parallel=4 ./e2e/...
```

## Writing E2E Tests

### Basic Test Structure

```go
//go:build e2e

package e2e

import (
    "context"
    "testing"
    "time"

    "github.com/tgrunnagle/parallel-playwright-mcp/e2e/helpers"
)

func TestMyFeature(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // Start test server
    server := helpers.NewTestServer(t)
    if err := server.Start(ctx); err != nil {
        t.Fatalf("failed to start server: %v", err)
    }
    defer server.Stop()

    // Create MCP client
    client := helpers.NewMCPClient(server.MCPURL())
    if _, err := client.Initialize(ctx); err != nil {
        t.Fatalf("failed to initialize MCP client: %v", err)
    }

    // Create browser session
    sessionID, err := client.CreateSession(ctx, "chromium", true)
    if err != nil {
        t.Fatalf("failed to create session: %v", err)
    }
    defer client.CloseSession(ctx, sessionID)

    // ... your test logic ...
}
```

### Using Test Fixtures

#### Static Fixtures

Use the `FixtureServer` to serve HTML files from the `fixtures/` directory:

```go
fixtures := helpers.NewFixtureServer(t)
defer fixtures.Close()

url := fixtures.URL("simple.html")
client.Navigate(ctx, sessionID, url)
```

#### Inline Fixtures

For tests that need specific HTML content without creating files:

```go
html := `<!DOCTYPE html>
<html>
<body><h1 id="title">Test Page</h1></body>
</html>`

fixtures := helpers.NewInlineFixtureServer(html)
defer fixtures.Close()

client.Navigate(ctx, sessionID, fixtures.URL())
```

#### Multi-Page Fixtures

For tests that need multiple pages with different content:

```go
pages := map[string]string{
    "/page1": `<html><body>Page 1</body></html>`,
    "/page2": `<html><body>Page 2</body></html>`,
}

fixtures := helpers.NewMultiPageFixtureServer(pages)
defer fixtures.Close()

client.Navigate(ctx, sessionID, fixtures.URL("/page1"))
```

### Calling MCP Tools

Use the `MCPClient` to call MCP tools:

```go
// Generic tool call
result, err := client.CallTool(ctx, "tool_name", map[string]any{
    "param1": "value1",
    "param2": 123,
})
if err != nil {
    t.Fatalf("tool call failed: %v", err)
}

// Convenience methods
sessionID, err := client.CreateSession(ctx, "chromium", true)
err = client.Navigate(ctx, sessionID, "https://example.com")
err = client.CloseSession(ctx, sessionID)
```

### Test Timeouts

Set appropriate timeouts for your tests:

```go
// Per-test timeout
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// Custom HTTP client timeout for slow operations
client := helpers.NewMCPClient(server.MCPURL(),
    helpers.WithTimeout(2*time.Minute))
```

## Component Reference

### TestServer

Manages the MCP server lifecycle:

| Method | Description |
|--------|-------------|
| `NewTestServer(t *testing.T)` | Creates a new test server instance |
| `Start(ctx context.Context)` | Starts the server and waits for readiness |
| `Stop()` | Stops the server and cleans up resources |
| `MCPURL()` | Returns the MCP endpoint URL |
| `HealthURL()` | Returns the health check URL |
| `Port()` | Returns the server port |

### MCPClient

Interacts with the MCP server:

| Method | Description |
|--------|-------------|
| `NewMCPClient(baseURL string)` | Creates a new MCP client |
| `Initialize(ctx)` | Performs MCP initialization handshake |
| `CallTool(ctx, name, args)` | Calls an MCP tool |
| `ListTools(ctx)` | Lists available tools |
| `CreateSession(ctx, browserType, headless)` | Creates a browser session |
| `Navigate(ctx, sessionID, url)` | Navigates to a URL |
| `CloseSession(ctx, sessionID)` | Closes a browser session |
| `Close(ctx)` | Closes the MCP session |

### FixtureServer

Serves test HTML fixtures:

| Method | Description |
|--------|-------------|
| `NewFixtureServer(t)` | Creates and starts fixture server |
| `URL(name string)` | Returns URL for a fixture file |
| `Close()` | Stops the fixture server |

## Best Practices

1. **Always use `defer` for cleanup**: Ensure servers and sessions are properly closed:
   ```go
   defer server.Stop()
   defer client.CloseSession(ctx, sessionID)
   ```

2. **Set appropriate timeouts**: Browser operations can be slow, use generous timeouts.

3. **Use parallel tests carefully**: Mark parallel-safe tests with `t.Parallel()`.

4. **Check for errors**: Always check return values from client operations.

5. **Log useful information**: Use `t.Logf()` for debugging information.

## Troubleshooting

### Server fails to start

- Ensure the server binary is built: `task build`
- Check that Playwright browsers are installed: `task setup`
- Look for port conflicts in the error message

### Tests timeout

- Increase the test timeout
- Check that the fixture server is accessible
- Verify network connectivity

### Browser session creation fails

- Ensure Playwright browsers are installed
- Check that headless mode is enabled for CI environments
- Look at server logs for detailed error messages

### Debug mode

To see server output during tests, the server logs are captured and printed when a test fails. For always-on logging, modify the server startup to redirect output:

```go
server := helpers.NewTestServer(t)
// Server stdout/stderr are captured to temp files
// and logged when tests fail
```
