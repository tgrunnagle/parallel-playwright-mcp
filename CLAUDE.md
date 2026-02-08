# parallel-playwright-mcp

A Playwright MCP (Model Context Protocol) server implemented in Go that supports parallel browser sessions for AI-driven browser automation.

## Tech Stack

- **Language**: Go 1.25+
- **MCP Framework**: [mcp-go](https://github.com/mark3labs/mcp-go)
- **Browser Automation**: [playwright-go](https://github.com/playwright-community/playwright-go)
- **Task Runner**: [Task](https://taskfile.dev/)

## Code Entry Points

| File | Description |
|------|-------------|
| [cmd/server/main.go](cmd/server/main.go) | Server entry point - initializes browser pool, session manager, and MCP tools |
| [pkg/config/config.go](pkg/config/config.go) | Configuration loading from file/environment |

## Key Components

### Core Packages

| Package | Purpose |
|---------|---------|
| [pkg/browser/](pkg/browser/) | Browser pool management (`pool.go`) |
| [pkg/session/](pkg/session/) | Session lifecycle management (`manager.go`, `session.go`) |
| [pkg/tools/](pkg/tools/) | MCP tool implementations |

### MCP Tools (in `pkg/tools/`)

| File | Tools |
|------|-------|
| [session.go](pkg/tools/session.go) | `session_create`, `session_list`, `session_close` |
| [navigation.go](pkg/tools/navigation.go) | `navigate`, `go_back`, `go_forward`, `reload` |
| [interaction.go](pkg/tools/interaction.go) | `click`, `type`, `fill`, `select_option`, `hover`, `press_key` |
| [inspection.go](pkg/tools/inspection.go) | `get_console_logs` |

### Infrastructure

| Package | Purpose |
|---------|---------|
| [pkg/shutdown/](pkg/shutdown/) | Graceful shutdown coordination, request draining |
| [pkg/middleware/](pkg/middleware/) | HTTP middleware (panic recovery) |
| [pkg/errors/](pkg/errors/) | Standardized MCP error formatting |

## Validation Commands

Use `task` for all final validation. Run `task --list` to see available tasks.

```bash
# Full validation (lint + test + build)
task check

# Individual checks
task lint          # Run go vet and go fmt
task test          # Run unit tests
task test:coverage # Run tests with coverage report
task build         # Build the server binary

# Setup
task setup         # Install deps + Playwright browsers
task deps          # Download Go modules only

# Run
task run           # Build and run the server
```

## Architecture Notes

- **1:N Model**: One MCP session can manage multiple browser sessions
- **Browser Pool**: Lazy-initializes browser engines (Chromium, Firefox, WebKit)
- **Session Isolation**: Each browser session has its own context with separate cookies/storage
- **Graceful Shutdown**: Coordinated cleanup with request draining and emergency cleanup on panic

## Testing

```bash
task test                # Unit tests
task test:integration    # Integration tests (requires Playwright browsers installed)
```

Integration tests are tagged with `//go:build integration` and require browser installation via `task setup`.
