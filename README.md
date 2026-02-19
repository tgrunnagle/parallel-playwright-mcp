# parallel-playwright-mcp

A [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server for browser automation via [Playwright](https://playwright.dev/), implemented in Go. Designed for AI agents that need to drive multiple browser sessions in parallel — each session has its own isolated context with separate cookies, storage, and navigation history.

Key capabilities:

- **Parallel sessions** — one MCP connection can manage many independent browser sessions across Chromium, Firefox, and WebKit
- **Full browser control** — navigate, click, type, screenshot, extract text, evaluate JavaScript, and more
- **Streamable HTTP transport** — communicates over HTTP using the MCP Streamable HTTP protocol, no stdio piping required
- **Configurable** — YAML config with environment variable overrides for host, port, timeouts, browser flags, and logging
- **Docker-ready** — single-stage Dockerfile bundles the server binary and all Playwright browser dependencies

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Task](https://taskfile.dev/installation/) (recommended task runner)

## Quick Start

```bash
# Install dependencies and Playwright browsers
task setup

# Run the server (builds automatically)
task run
```

The server starts on `http://127.0.0.1:3000` by default.

### Docker

```bash
# Build image and run container
task docker:run

# Run with a minimal tool set
task docker:run:min-tools

# Run with custom config
task docker:run MCP_CONFIG_PATH=./custom.yaml
```

### Manual Setup

```bash
go mod download
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps
go build -o bin/playwright-mcp-server ./cmd/server
./bin/playwright-mcp-server
```

## MCP Tools

### Session Management

| Tool | Description |
|------|-------------|
| `session_create` | Create a new isolated browser session |
| `session_list` | List all active browser sessions for this connection |
| `session_close` | Close a browser session and release its resources |

### Navigation

| Tool | Description |
|------|-------------|
| `navigate` | Navigate to a URL |
| `go_back` | Go back in browser history |
| `go_forward` | Go forward in browser history |
| `reload` | Reload the current page |

### Interaction

| Tool | Description |
|------|-------------|
| `click` | Click an element on the page |
| `type` | Type text into an element with keystroke events |
| `fill` | Fill an input field with text (fast, no keystroke events) |
| `select_option` | Select an option from a dropdown |
| `hover` | Hover over an element |
| `press_key` | Press a keyboard key or key combination |

### Inspection & Extraction

| Tool | Description |
|------|-------------|
| `screenshot` | Capture a PNG screenshot of the page or a specific element |
| `extract_text` | Extract visible text from the page in hierarchical structure |
| `get_html` | Get HTML source of the page or an element |
| `evaluate` | Execute a JavaScript expression in the page context |
| `query_selector` | Query element info by CSS selector |
| `get_accessibility_tree` | Get an accessibility snapshot of the page |
| `get_links` | Get all hyperlinks from the page with IDs and labels |
| `get_console_logs` | Retrieve browser console log messages |
| `get_network_logs` | Retrieve captured network request/response logs |
| `navigate_and_extract_text` | Navigate to a URL and extract visible text as markdown (creates a temporary session) |

Tools can be restricted at startup with the `enabled_tools` config option or `MCP_ENABLED_TOOLS` environment variable.

## Task Targets

| Target | Description |
|--------|-------------|
| `task setup` | Install Go deps and Playwright browsers |
| `task run` | Build and run the server |
| `task build` | Build the server binary to `bin/` |
| `task test` | Run unit tests |
| `task test:e2e` | Run E2E tests (requires Playwright browsers) |
| `task lint` | Run `go vet` and `go fmt` |
| `task check` | Run lint + test + build |
| `task docker:build` | Build the Docker image |
| `task docker:run` | Build and run in Docker |
| `task clean` | Remove build artifacts |

Run `task --list` for the complete list.

## Configuration

The server loads configuration from `config.yaml` (or the path in `MCP_CONFIG_PATH`) with environment variable overrides:

| Variable | Override | Default |
|----------|---------|---------|
| `MCP_HOST` | Server bind address | `127.0.0.1` |
| `MCP_PORT` | Server port | `3000` |
| `MCP_BROWSER_HEADLESS` | Headless mode | `true` |
| `MCP_LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `MCP_TIMEOUT_DEFAULT` | Default timeout (ms) | `30000` |
| `MCP_TIMEOUT_NAVIGATION` | Navigation timeout (ms) | `30000` |
| `MCP_TIMEOUT_ELEMENT` | Element wait timeout (ms) | `5000` |
| `MCP_ENABLED_TOOLS` | Comma-separated list of enabled tools | all tools |

See [config.yaml](config.yaml) for the full configuration reference.
