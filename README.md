# parallel-playwright-mcp

A Playwright MCP server that supports parallel browser sessions, implemented in Go.

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- [Task](https://taskfile.dev/installation/)

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

## Configuration

The server can be configured via `config.yaml` or environment variables. See `pkg/config/config.go` for available options.
