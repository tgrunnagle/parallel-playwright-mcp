# Playwright MCP Server Design Document

## Overview

A Model Context Protocol (MCP) server written in Go that provides browser automation capabilities through Playwright, supporting parallel browsing sessions and the streamable-http transport protocol.

**Key Design Decisions:**
- **Multi-browser**: Supports Chromium, Firefox, and WebKit (configurable per session)
- **1:N Session Model**: A single MCP client can manage multiple independent browser sessions
- **Launch-only**: Server launches its own browser instances (no CDP connect mode)

---

## 1. Functional Requirements

### 1.1 Core Browser Automation

| Requirement | Description |
|-------------|-------------|
| **FR-1.1** | Navigate to URLs with configurable timeout and wait conditions |
| **FR-1.2** | Click elements using CSS selectors, XPath, or accessibility labels |
| **FR-1.3** | Type text into input fields with optional keystroke delay |
| **FR-1.4** | Take screenshots (full page, viewport, or element-specific) |
| **FR-1.5** | Execute JavaScript in page context and return results |
| **FR-1.6** | Extract page content (text, HTML, accessibility tree) |
| **FR-1.7** | Handle form interactions (select dropdowns, checkboxes, file uploads) |
| **FR-1.8** | Support keyboard and mouse events (hover, drag, key combinations) |

### 1.2 Session Management (1:N Model)

| Requirement | Description |
|-------------|-------------|
| **FR-2.1** | Single MCP connection can create/manage multiple browser sessions |
| **FR-2.2** | Each browser session is isolated (separate context, cookies, storage) |
| **FR-2.3** | Browser sessions identified by unique `sessionId` parameter in tool calls |
| **FR-2.4** | Support different browser types per session (chromium/firefox/webkit) |
| **FR-2.5** | Graceful session cleanup on explicit close, MCP disconnect, or timeout |
| **FR-2.6** | List active sessions with metadata (browser, URL, title, creation time) |
| **FR-2.7** | Session state persists across tool calls within the same MCP connection |

### 1.3 Multi-Tab Support

| Requirement | Description |
|-------------|-------------|
| **FR-3.1** | Create new tabs within a session |
| **FR-3.2** | Switch between tabs in a session |
| **FR-3.3** | Close specific tabs |
| **FR-3.4** | Execute operations targeting specific tabs |

### 1.4 MCP Protocol Compliance

| Requirement | Description |
|-------------|-------------|
| **FR-4.1** | Implement MCP initialization handshake with capability negotiation |
| **FR-4.2** | Expose browser tools via `tools/list` and `tools/call` |
| **FR-4.3** | Support streamable-http transport with session management |
| **FR-4.4** | Handle JSON-RPC 2.0 requests, responses, and notifications |
| **FR-4.5** | Support request cancellation via `notifications/cancellation` |
| **FR-4.6** | Emit progress notifications for long-running operations |

### 1.5 Observability

| Requirement | Description |
|-------------|-------------|
| **FR-5.1** | Capture and expose browser console logs |
| **FR-5.2** | Report network request/response metadata |
| **FR-5.3** | Provide error details with actionable context |
| **FR-5.4** | Support structured logging for server operations |

---

## 2. Non-Functional Requirements

| Requirement | Description |
|-------------|-------------|
| **NFR-1** | Handle 10+ concurrent browser sessions on standard hardware |
| **NFR-2** | Sub-100ms latency for simple operations (click, type) |
| **NFR-3** | Graceful degradation under resource pressure |
| **NFR-4** | Clean shutdown with proper resource cleanup |
| **NFR-5** | Configuration via environment variables and config file |

---

## 3. Architecture Overview

### 3.1 Two-Level Session Model

The server uses a two-level session model:

| Level | ID | Scope | Lifecycle |
|-------|-----|-------|-----------|
| **MCP Session** | `Mcp-Session-Id` header | Transport connection | Created on `initialize`, ends on `DELETE /mcp` |
| **Browser Session** | `sessionId` tool param | Isolated browser context | Created via `session_create`, explicit or auto-cleanup |

**Example: One MCP client managing three parallel browser sessions:**
```
MCP Connection (Mcp-Session-Id: "conn-abc123")
├── Browser Session "sess-001" (Chromium) → github.com
├── Browser Session "sess-002" (Firefox)  → localhost:3000
└── Browser Session "sess-003" (Chromium) → docs.example.com
```

### 3.2 System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         MCP Clients                             │
└─────────────────────────┬───────────────────────────────────────┘
                          │ HTTP (streamable-http)
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    HTTP Transport Layer                         │
│    POST /mcp (requests)  │  GET /mcp (SSE)  │  DELETE /mcp      │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    MCP Protocol Handler                         │
│         JSON-RPC Router  │  MCP Session Store  │  Capabilities  │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Tool Registry                              │
│   session_create │ navigate │ click │ screenshot │ evaluate     │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│               Browser Session Manager                           │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Per MCP-Connection Session Pool                          │  │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │  │
│  │  │ sess-001    │ │ sess-002    │ │ sess-003    │          │  │
│  │  │ Chromium    │ │ Firefox     │ │ WebKit      │          │  │
│  │  │ Context     │ │ Context     │ │ Context     │          │  │
│  │  │ └─Pages     │ │ └─Pages     │ │ └─Pages     │          │  │
│  │  └─────────────┘ └─────────────┘ └─────────────┘          │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────┬───────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Browser Pool                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                 Playwright Runtime                       │    │
│  │    ┌──────────┐   ┌──────────┐   ┌──────────┐           │    │
│  │    │ Chromium │   │ Firefox  │   │ WebKit   │           │    │
│  │    │ (shared) │   │ (shared) │   │ (shared) │           │    │
│  │    └──────────┘   └──────────┘   └──────────┘           │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

**Key Points:**
- Browser instances (Chromium, Firefox, WebKit) are shared across all sessions
- Each browser session gets an isolated BrowserContext (cookies, storage, cache)
- Sessions within same browser type share the process but are fully isolated
- Browsers are lazily started on first session request for that type

---

## 4. Component Details

### 4.1 MCP Server (using mark3labs/mcp-go)

Uses the `mcp-go` library for protocol handling, transport, and tool registration.

```go
// cmd/server/main.go

import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

func main() {
    // Create MCP server
    s := server.NewMCPServer(
        "playwright-mcp",
        "1.0.0",
        server.WithToolCapabilities(true),
    )

    // Register tools
    sessionMgr := session.NewManager(browserPool)

    s.AddTool(tools.SessionCreate(sessionMgr))
    s.AddTool(tools.SessionClose(sessionMgr))
    s.AddTool(tools.SessionList(sessionMgr))
    s.AddTool(tools.Navigate(sessionMgr))
    s.AddTool(tools.Click(sessionMgr))
    s.AddTool(tools.ExtractText(sessionMgr))
    s.AddTool(tools.Screenshot(sessionMgr))
    // ... register all tools

    // Start with streamable HTTP transport
    httpServer := server.NewStreamableHTTPServer(s,
        server.WithAddress("127.0.0.1:3000"),
    )
    httpServer.Start(ctx)
}
```

### 4.2 Tool Registration Pattern

Each tool is defined using mcp-go's tool builder pattern.

```go
// tools/navigation.go

func Navigate(mgr *session.Manager) mcp.Tool {
    return mcp.NewTool("navigate",
        mcp.WithDescription("Navigate to a URL in the browser session"),
        mcp.WithString("sessionId", mcp.Required(), mcp.Description("Browser session ID")),
        mcp.WithString("url", mcp.Required(), mcp.Description("URL to navigate to")),
        mcp.WithString("waitUntil", mcp.Enum("load", "domcontentloaded", "networkidle")),
        mcp.WithNumber("timeout", mcp.Description("Timeout in milliseconds")),
    ).WithHandler(func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        sessionID := req.Params.Arguments["sessionId"].(string)
        url := req.Params.Arguments["url"].(string)

        sess, ok := mgr.GetSession(sessionID)
        if !ok {
            return mcp.NewToolResultError("Session not found"), nil
        }

        page := sess.ActivePage()
        if _, err := page.Goto(url); err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }

        return mcp.NewToolResultText(fmt.Sprintf("Navigated to %s", url)), nil
    })
}
```

### 4.3 Browser Session Manager

Manages browser sessions within an MCP connection (1:N model).

```go
// session/manager.go

type BrowserSessionManager interface {
    // CreateSession creates a new browser session for an MCP connection
    CreateSession(ctx context.Context, mcpSessionID string, opts SessionOptions) (*BrowserSession, error)

    // GetSession retrieves a browser session (validates MCP ownership)
    GetSession(mcpSessionID, browserSessionID string) (*BrowserSession, bool)

    // CloseSession terminates a specific browser session
    CloseSession(ctx context.Context, mcpSessionID, browserSessionID string) error

    // ListSessions returns all browser sessions for an MCP connection
    ListSessions(mcpSessionID string) []*SessionInfo

    // CloseAllForMCP closes all browser sessions when MCP connection ends
    CloseAllForMCP(ctx context.Context, mcpSessionID string) error

    // Cleanup removes expired/orphaned sessions
    Cleanup(ctx context.Context) error
}

type BrowserSession struct {
    ID            string                     // Browser session ID (e.g., "sess-001")
    MCPSessionID  string                     // Owning MCP connection
    BrowserType   BrowserType                // chromium, firefox, webkit
    Context       playwright.BrowserContext
    Pages         map[string]playwright.Page // tabID -> Page
    ActiveTabID   string
    NetworkLogs   *NetworkLogBuffer          // Circular buffer of recent requests
    CreatedAt     time.Time
    LastAccess    time.Time
    Metadata      map[string]any
}

type NetworkLogBuffer struct {
    Entries []NetworkLogEntry
    MaxSize int // Default: 100 entries
}

type NetworkLogEntry struct {
    Timestamp    time.Time
    Method       string // GET, POST, etc.
    URL          string
    Status       int
    Duration     time.Duration
    RequestSize  int64
    ResponseSize int64
}

type BrowserType string

const (
    BrowserChromium BrowserType = "chromium"
    BrowserFirefox  BrowserType = "firefox"
    BrowserWebKit   BrowserType = "webkit"
)

type SessionOptions struct {
    BrowserType  BrowserType
    Headless     bool
    Viewport     *Viewport
    UserAgent    string
    Locale       string
    TimezoneID   string
    ExtraHeaders map[string]string
    Timeout      time.Duration
}
```

### 4.4 Browser Pool

Manages browser instances for all three browser types. Browsers are lazily started on first use.

```go
// browser/pool.go

type BrowserPool interface {
    // NewContext creates an isolated browser context for the specified browser type
    NewContext(ctx context.Context, browserType BrowserType, opts ContextOptions) (playwright.BrowserContext, error)

    // Start initializes the Playwright runtime (browsers started lazily)
    Start(ctx context.Context) error

    // Stop closes all browsers and the Playwright runtime
    Stop(ctx context.Context) error

    // Stats returns pool statistics per browser type
    Stats() PoolStats
}

type ContextOptions struct {
    Headless     bool
    Viewport     *Viewport
    UserAgent    string
    Locale       string
    TimezoneID   string
    Permissions  []string
    ExtraHeaders map[string]string
}

type PoolStats struct {
    Browsers map[BrowserType]BrowserStats
}

type BrowserStats struct {
    Running        bool
    ActiveContexts int
    TotalCreated   int
    TotalClosed    int
}
```

---

## 5. Tool Definitions

> **Note:** All tools except `session_create` and `session_list` require `sessionId` parameter.

### 5.1 Session Management Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `session_create` | Create new browser session | `browserType`, `headless`, `viewport` (all optional) |
| `session_close` | Close a browser session | `sessionId`* |
| `session_list` | List active sessions | - |

*Required parameter

### 5.2 Navigation Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `navigate` | Navigate to URL | `sessionId`*, `url`*, `waitUntil`, `timeout` |
| `go_back` | Go back in history | `sessionId`* |
| `go_forward` | Go forward in history | `sessionId`* |
| `reload` | Reload current page | `sessionId`*, `waitUntil` |

### 5.3 Interaction Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `click` | Click an element | `sessionId`*, `selector`*, `button`, `clickCount` |
| `type` | Type text with keystrokes | `sessionId`*, `selector`*, `text`*, `delay` |
| `fill` | Fill input field (fast) | `sessionId`*, `selector`*, `value`* |
| `select_option` | Select dropdown option | `sessionId`*, `selector`*, `value`* |
| `hover` | Hover over element | `sessionId`*, `selector`* |
| `press_key` | Press keyboard key | `sessionId`*, `key`*, `modifiers` |

### 5.4 Inspection Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `screenshot` | Capture PNG screenshot | `sessionId`*, `fullPage`, `selector` |
| `extract_text` | Extract visible text with page structure hierarchy | `sessionId`*, `selector` (optional) |
| `navigate_and_extract_text` | Navigate to a URL and extract visible text with page structure hierarchy (no session required) | `url`*, `waitUntil`, `timeout` |
| `get_html` | Get page HTML source | `sessionId`*, `selector` (optional), `outer` |
| `get_accessibility_tree` | Get accessibility snapshot | `sessionId`* |
| `evaluate` | Execute JavaScript | `sessionId`*, `expression`* |
| `query_selector` | Query element info | `sessionId`*, `selector`*, `all` |
| `get_network_logs` | Get captured network requests | `sessionId`*, `limit`, `filter` |

### 5.5 Tab Management Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `tab_new` | Create new tab | `sessionId`*, `url` |
| `tab_list` | List tabs in session | `sessionId`* |
| `tab_switch` | Switch active tab | `sessionId`*, `tabId`* |
| `tab_close` | Close a tab | `sessionId`*, `tabId`* |

### 5.6 Example Tool Schemas

**session_create:**
```json
{
  "name": "session_create",
  "description": "Create a new isolated browser session",
  "inputSchema": {
    "type": "object",
    "properties": {
      "browserType": {
        "type": "string",
        "enum": ["chromium", "firefox", "webkit"],
        "default": "chromium",
        "description": "Browser engine to use"
      },
      "headless": {
        "type": "boolean",
        "default": true,
        "description": "Run browser in headless mode"
      },
      "viewport": {
        "type": "object",
        "description": "Optional viewport size (defaults to 1280x720)",
        "properties": {
          "width": { "type": "integer" },
          "height": { "type": "integer" }
        }
      }
    }
  }
}
```

All parameters are optional with sensible defaults:
- `browserType`: "chromium"
- `headless`: true
- `viewport`: 1280x720

**navigate:**
```json
{
  "name": "navigate",
  "description": "Navigate to a URL in the browser session",
  "inputSchema": {
    "type": "object",
    "required": ["sessionId", "url"],
    "properties": {
      "sessionId": {
        "type": "string",
        "description": "Browser session ID"
      },
      "url": {
        "type": "string",
        "description": "URL to navigate to"
      },
      "waitUntil": {
        "type": "string",
        "enum": ["load", "domcontentloaded", "networkidle"],
        "default": "load"
      },
      "timeout": {
        "type": "integer",
        "default": 30000,
        "description": "Timeout in milliseconds"
      }
    }
  }
}
```

**extract_text:**
```json
{
  "name": "extract_text",
  "description": "Extract visible text from page in a hierarchical structure representing the page layout",
  "inputSchema": {
    "type": "object",
    "required": ["sessionId"],
    "properties": {
      "sessionId": {
        "type": "string",
        "description": "Browser session ID"
      },
      "selector": {
        "type": "string",
        "description": "CSS selector to extract from (defaults to body)"
      }
    }
  }
}
```

**Example extract_text output:**
```json
{
  "content": [{
    "type": "text",
    "text": {
      "tag": "body",
      "children": [
        {
          "tag": "h1",
          "text": "Welcome to Example"
        },
        {
          "tag": "nav",
          "children": [
            { "tag": "a", "text": "Home" },
            { "tag": "a", "text": "About" },
            { "tag": "a", "text": "Contact" }
          ]
        },
        {
          "tag": "main",
          "children": [
            { "tag": "p", "text": "This is the main content..." },
            {
              "tag": "ul",
              "children": [
                { "tag": "li", "text": "First item" },
                { "tag": "li", "text": "Second item" }
              ]
            }
          ]
        }
      ]
    }
  }]
}
```

---

## 6. Error Handling

### 6.1 Error Codes

| Code | Name | Description |
|------|------|-------------|
| -32700 | Parse Error | Invalid JSON |
| -32600 | Invalid Request | Invalid JSON-RPC |
| -32601 | Method Not Found | Unknown method |
| -32602 | Invalid Params | Invalid parameters |
| -32603 | Internal Error | Server error |
| -32001 | Session Not Found | Invalid session ID |
| -32002 | Element Not Found | Selector not found |
| -32003 | Timeout | Operation timed out |
| -32004 | Navigation Failed | Page load failed |

### 6.2 Error Response Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32002,
    "message": "Element not found",
    "data": {
      "selector": "#missing-button",
      "timeout": 5000,
      "suggestion": "Verify the selector or increase timeout"
    }
  }
}
```

---

## 7. Configuration

```yaml
# config.yaml
server:
  host: "127.0.0.1"
  port: 3000

browser:
  # Default browser if not specified in session_create
  defaultType: "chromium"
  headless: true
  slowMo: 0
  # Per-browser launch options
  chromium:
    args: ["--disable-gpu"]
  firefox:
    args: []
  webkit:
    args: []

session:
  maxPerConnection: 10    # Max browser sessions per MCP connection
  maxTotal: 50            # Max total browser sessions across all connections
  defaultTimeout: 30s
  idleTimeout: 5m         # Auto-close idle sessions

logging:
  level: "info"
  format: "json"
```

### 7.1 Environment Variable Overrides

| Variable | Description | Default |
|----------|-------------|---------|
| `MCP_HOST` | Server bind address | `127.0.0.1` |
| `MCP_PORT` | Server port | `3000` |
| `MCP_BROWSER_HEADLESS` | Run browsers headless | `true` |
| `MCP_LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |

---

## 8. Project Structure

```
playwright-mcp/
├── cmd/
│   └── server/
│       └── main.go           # Entry point, mcp-go server setup
├── internal/
│   ├── browser/
│   │   └── pool.go           # Browser instance pool (Chromium, Firefox, WebKit)
│   ├── session/
│   │   ├── manager.go        # Browser session manager (1:N model)
│   │   ├── session.go        # BrowserSession struct and methods
│   │   └── network.go        # Network log capture
│   └── tools/
│       ├── session.go        # session_create, session_close, session_list
│       ├── navigation.go     # navigate, go_back, go_forward, reload
│       ├── interaction.go    # click, type, fill, select_option, hover, press_key
│       ├── inspection.go     # screenshot, extract_text, get_html, evaluate
│       └── tabs.go           # tab_new, tab_list, tab_switch, tab_close
├── config/
│   └── config.go             # Configuration loading
├── go.mod
├── go.sum
└── README.md
```

**Note:** No custom protocol or transport code needed - `mcp-go` handles all MCP protocol details.

---

## 9. Implementation Phases

### Phase 1: Foundation
- [ ] Project setup (go.mod, directory structure)
- [ ] Integrate `mcp-go` library with streamable-http transport
- [ ] Initialize Playwright runtime and browser pool
- [ ] Basic server startup with health check

### Phase 2: Browser Infrastructure
- [ ] Browser pool with lazy initialization (Chromium, Firefox, WebKit)
- [ ] Browser session manager (1:N model)
- [ ] Session lifecycle (create, get, close, cleanup)
- [ ] Session tools: `session_create`, `session_list`, `session_close`

### Phase 3: Core Automation Tools
- [ ] Navigation: `navigate`, `go_back`, `go_forward`, `reload`
- [ ] Interaction: `click`, `type`, `fill`, `select_option`, `hover`, `press_key`
- [ ] Inspection: `screenshot`, `get_content`, `evaluate`, `query_selector`
- [ ] Network logging: capture requests, `get_network_logs` tool

### Phase 4: Advanced Features
- [ ] Multi-tab support: `tab_new`, `tab_list`, `tab_switch`, `tab_close`
- [ ] SSE streaming for progress notifications
- [ ] Console log capture
- [ ] Error recovery and retries

### Phase 5: Production Readiness
- [ ] Comprehensive error handling with actionable messages
- [ ] Configuration file and environment variable support
- [ ] Graceful shutdown and resource cleanup
- [ ] Documentation and examples

---

## 10. Verification Plan

### 10.1 Unit Tests
- JSON-RPC message parsing
- Session manager operations
- Tool argument validation

### 10.2 Integration Tests
- MCP protocol compliance (initialize handshake, capability negotiation)
- Session lifecycle (create → use → close)
- Multi-browser session management

### 10.3 E2E Tests
- Full automation workflow (session → navigate → interact → screenshot → close)
- Parallel sessions with different browsers
- Error handling and recovery

### 10.4 Manual Validation
- **MCP Inspector**: Connect and test tools interactively
- **Claude Desktop**: Configure as MCP server and test with Claude

---

## 11. Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/mark3labs/mcp-go` | MCP protocol implementation (JSON-RPC, transport, tools) |
| `github.com/playwright-community/playwright-go` | Browser automation |
| `github.com/google/uuid` | Session ID generation |

---

## 12. Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Browser support | Chromium, Firefox, WebKit | Maximum compatibility |
| Session model | 1:N (MCP → many browser sessions) | Flexibility for parallel workflows |
| Browser mode | Launch only (no CDP connect) | Simpler, more isolated |
| Screenshot format | PNG only | Universal, lossless |
| Network logging | Basic request/response capture | Useful for debugging without complexity |
