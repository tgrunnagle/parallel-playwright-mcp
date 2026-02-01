# parallel-playwright-mcp

A Playwright MCP server that supports parallel browser sessions, implemented in Go.

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Task](https://taskfile.dev/installation/) (optional, for running tasks)

## Quick Start

### Using Task (recommended)

```bash
# Install dependencies and Playwright browsers
task setup

# Run tests
task test

# Build the server
task build

# Run the server
task run
```

### Manual Setup

```bash
# Download Go dependencies
go mod download

# Install Playwright browsers (required)
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps

# Build
go build -o bin/playwright-mcp-server ./cmd/server

# Run
./bin/playwright-mcp-server
```

## Available Tasks

Run `task --list` to see all available tasks:

| Task | Description |
|------|-------------|
| `task setup` | Install all dependencies including Playwright browsers |
| `task build` | Build the server binary |
| `task run` | Build and run the server |
| `task test` | Run unit tests |
| `task test:integration` | Run integration tests (requires Playwright) |
| `task test:coverage` | Run tests with coverage report |
| `task lint` | Run linting and static analysis |
| `task check` | Run all checks (lint, test, build) |
| `task clean` | Remove build artifacts |

## Installing Only Specific Browsers

If you only need certain browsers, you can install them individually:

```bash
# Install only Chromium
task playwright:install-chromium

# Or manually
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium
```

## Configuration

The server can be configured via `config.yaml` or environment variables. See `pkg/config/config.go` for available options.

## License

MIT
