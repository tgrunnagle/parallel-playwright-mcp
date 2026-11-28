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
| `navigate` | Navigate to URL | `sessionId`*, `url`*, `waitUntil`, `timeout`, `includeSnapshot` |
| `go_back` | Go back in history | `sessionId`*, `includeSnapshot` |
| `go_forward` | Go forward in history | `sessionId`*, `includeSnapshot` |
| `reload` | Reload current page | `sessionId`*, `waitUntil`, `includeSnapshot` |

### 5.3 Interaction Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `click` | Click an element | `sessionId`*, `selector`† or `ref`†, `button`, `clickCount`, `includeSnapshot` |
| `type` | Type text with keystrokes | `sessionId`*, `selector`† or `ref`†, `text`*, `delay`, `includeSnapshot` |
| `fill` | Fill input field (fast) | `sessionId`*, `selector`† or `ref`†, `value`*, `includeSnapshot` |
| `select_option` | Select dropdown option | `sessionId`*, `selector`† or `ref`†, `value`*, `includeSnapshot` |
| `hover` | Hover over element | `sessionId`*, `selector`† or `ref`†, `includeSnapshot` |
| `press_key` | Press keyboard key | `sessionId`*, `key`*, `selector` or `ref` (optional), `modifiers`, `includeSnapshot` |

†Exactly one of `selector` and `ref` is required. JSON Schema cannot express this through mcp-go's builders, so the tool descriptions state it and the handlers enforce it: both or neither returns `[-32602]` naming both parameters. `press_key` takes at most one; with neither, the key goes to the page.

**Element refs.** A `ref` is an element id from a snapshot (`get_accessibility_tree`, or an action's `includeSnapshot`): `e<n>`, or `f<k>e<n>` for an element inside an iframe or in a document loaded after the first. It is resolved with the Playwright selector `aria-ref=<ref>`, so it names exactly one element even among identical ones. A ref must match `^(f[0-9]+)?e[0-9]+$` before any locator is built, so it cannot chain or switch selector engines. Refs stay the same while the document is unchanged; after a navigation the new document's refs take a new prefix and old refs match nothing. The handler counts a ref's matches before acting, and zero returns `[-32002] Stale element ref` at once instead of waiting for the timeout.

**State summary.** Every action tool (§5.2 and §5.3) returns a summary of the page after the action. When the URL changed, the tool waits for `domcontentloaded`, bounded by the element timeout; it then waits a 150 ms settle window and reads:

```json
{"action": "click", "target": "ref=e3", "url": "http://localhost:8080/page2", "title": "Page Two",
 "navigated": true, "newConsoleErrors": ["boom from click"], "failedRequests": ["GET http://localhost:8080/api 500"]}
```

- `target` is the selector, or `ref=<ref>`; it is omitted for page-level actions. `url` is the final URL, after redirects for `navigate`. `title` is omitted when it cannot be read; that never fails the action.
- `navigated` is true when the URL changed, or when a navigation tool got a response. `httpStatus` is the main document's status, on navigation tools only.
- `newConsoleErrors` lists console errors, and `failedRequests` requests with status 400 or above or no response (`... (no response)`), logged since the action started. The console and network buffers keep a total-appended counter that only increases, so entries are found as new even on a full buffer. Request URLs lose their query string, fragment, and user info, since those may carry tokens.
- Each list holds at most 5 entries of at most 200 characters; entries left out are counted under `"more": {"newConsoleErrors": <n>, "failedRequests": <m>}`, omitted when nothing was cut.
- The result's structured content is the summary. Its text is the tool's one-line message, a newline, then the summary JSON, so hosts that pass only `content` still see it. Console messages or requests that arrive after the settle window appear in the next summary.

With `includeSnapshot: true`, the result appends a second content block holding the snapshot (§5.4) taken after the settle window, cut to the response budget; a cut snapshot adds a third block, `[truncated: <n> more characters; call get_accessibility_tree with offset=<k>]`. If the snapshot fails after a successful action, the second block reports the error and the result stays a success.

### 5.4 Inspection Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `screenshot` | Capture PNG screenshot | `sessionId`*, `fullPage`, `selector` |
| `extract_text` | Extract visible text as markdown, with form controls as inline tokens | `sessionId`*, `selector` (optional), `maxChars`, `offset` |
| `navigate_and_extract_text` | Navigate to a URL and extract visible text as markdown (no session required) | `url`*, `waitUntil`, `selector`, `browserType`, `maxChars`, `offset` |
| `get_html` | Get page HTML source | `sessionId`*, `selector` (optional), `outer`, `maxChars`, `offset` |
| `get_accessibility_tree` | Get an ARIA snapshot with element refs | `sessionId`*, `selector`, `depth`, `maxChars`, `offset` |
| `evaluate` | Execute JavaScript | `sessionId`*, `expression`*, `maxChars` |
| `query_selector` | Query element info | `sessionId`*, `selector`*, `all`, `limit`, `offset` |
| `get_links` | Get hyperlinks with IDs and labels | `sessionId`*, `url`, `waitUntil`, `timeout`, `limit`, `offset` |
| `get_console_logs` | Get captured console messages | `sessionId`*, `level`, `limit`, `offset` |
| `get_network_logs` | Get captured network requests | `sessionId`*, `limit`, `offset`, `filter` |

**ARIA snapshot.** `get_accessibility_tree` returns Playwright's AI-mode ARIA snapshot (`Page.AriaSnapshot` with `Mode: AriaSnapshotModeAi`): a compact YAML tree with one line per element, carrying its role, accessible name, states, ref, and form value.

```yaml
- main [ref=e2]:
  - heading "Sign in" [level=1] [ref=e3]
  - textbox "Email" [ref=e4]: a@b.c
  - textbox "Password" [ref=e5]: •••
  - checkbox "Remember me" [checked] [ref=e6]
  - button "Go" [ref=e7]
  - button "Go" [ref=e8]
```

- AI mode prints input values in clear text, passwords included. Each `textbox` line with a value is resolved through its ref, and the value is replaced with `•••` when the element's `type` is `password`, or when its type cannot be read. AI mode leaves refs off elements that are covered or zero-size but still prints their values, so a textbox without a ref always has its value replaced. Values on the inline form and on `- text:` child lines (used when a placeholder differs from the name) are both covered.
- Content hidden with `display:none` or `visibility:hidden` is left out. Content that is on screen but `aria-hidden` is kept, since an agent may need to act on it; this is how AI mode differs from the default ARIA snapshot.
- `selector` snapshots one element instead of the page. A selector matching nothing returns `[-32002]` at once instead of waiting for the timeout; one matching several returns the multi-match error (§6.1). `depth` limits the tree depth.
- The snapshot follows the text-tool budget below: `maxChars` and `offset` in characters, with the same truncation notice.

**Response budget and pagination.** Content tools return at most `tools.maxResponseChars` characters (Unicode code points, default 40000) in their content block per call. A second text content block, the pagination block, carries truncation notices or list metadata, so page content can never write into it.

- Text tools (`extract_text`, `navigate_and_extract_text`, `get_html`, `get_accessibility_tree`) take `maxChars` and `offset` in characters. Output within the budget is returned unchanged as one block. Cut output adds `[truncated: <n> more characters; call again with offset=<k>]`; an `offset` at or past the end returns an empty block and `[end of content: <total> characters]`.
- `evaluate` takes `maxChars` only, because paging would re-run the script. Cut output adds `[truncated: result JSON is incomplete; narrow the expression]`.
- `query_selector` without `all` returns one element object, with its string fields cut the same way when it exceeds the budget.
- List tools (`query_selector` with `all: true`, `get_links`, `get_console_logs`, `get_network_logs`) take `limit` (default 50) and `offset` in items. The first block stays the bare JSON array; the second is `{"total": <n>, "offset": <k>, "returned": <m>, "hasMore": <bool>}`. Trailing items that do not fit the budget are dropped, so continue with `offset + returned` while `hasMore` is true. A single item larger than the budget is returned alone with its string fields cut, or as `{"truncated": true}` when its keys alone exceed the budget, so `offset` always advances.
- The log tools select newest first: `offset` 0 is the newest entries and `offset` k skips the k newest. Items are returned oldest first, and `total` counts entries after the `level` or `filter` is applied. The buffers are live, so offsets and `total` can shift between calls.
- `maxChars` above the budget is clamped. A non-positive `maxChars` or `limit`, or a negative `offset`, returns `[-32602]` naming the parameter, before the page is touched.

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
  "description": "Extract visible text from page as markdown",
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
      },
      "maxChars": {
        "type": "number",
        "description": "Maximum characters to return (default and upper bound: the server's response budget)"
      },
      "offset": {
        "type": "number",
        "description": "Character offset to start from, for reading the next slice of truncated output (default: 0)"
      }
    }
  }
}
```

**Example extract_text output** (called with `maxChars: 270` on a longer page):

The page is rendered as markdown. Links and images carry absolute URLs, and form controls appear inline as `[<role> "<label>": <value>]`. Password fields show only `set` or `empty`, and iframes render as `[iframe "<title or src>"]`.

```json
{
  "content": [
    {
      "type": "text",
      "text": "# Welcome to Example\n\n[Home](https://example.com/) [About](https://example.com/about)\n\nSign in to continue.\n\nEmail [textbox \"Email\": a@b.c]\nPassword [password \"Password\": set]\n[checkbox \"Remember me\": unchecked] Remember me\n[button \"Sign in\"]\n\n## Recent posts\n\n- First i"
    },
    {
      "type": "text",
      "text": "[truncated: 1840 more characters; call again with offset=270]"
    }
  ]
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
| -32002 | Element Not Found | Selector or ref did not resolve to one element (see below) |
| -32003 | Timeout | Operation timed out |
| -32004 | Navigation Failed | Page load failed |
| -32005 | Interaction Failed | An element action failed for another reason |

Every tool error goes through `errors.FormatErrorForTool`, so its text starts with `[<code>]`. Missing or invalid parameters use -32602 and unexpected server-side failures use -32603. -32002 carries one of three messages:

- `Element not found`: the selector matched no element.
- `Stale element ref`: a ref is not on the current page, with reason `ref <r> is not on the current page` and suggestion `Take a new snapshot with get_accessibility_tree`. It is returned at once, without waiting for the timeout.
- `Selector matched <n> elements`: a strict mode violation, with the first 3 matches' tag and text (80 characters each) in the reason.

If retry is later wired to `retry.retryableErrors` (default `[-32002, -32003]`), the stale-ref and multi-match errors must be excluded: retrying them cannot succeed.

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

tools:
  maxResponseChars: 40000  # Response budget for content tools, in characters
```

### 7.1 Environment Variable Overrides

| Variable | Description | Default |
|----------|-------------|---------|
| `MCP_HOST` | Server bind address | `127.0.0.1` |
| `MCP_PORT` | Server port | `3000` |
| `MCP_BROWSER_HEADLESS` | Run browsers headless | `true` |
| `MCP_LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `MCP_MAX_RESPONSE_CHARS` | Response budget for content tools, in characters (`tools.maxResponseChars`; must be a positive integer) | `40000` |

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
| `github.com/mxschmitt/playwright-go` | Browser automation |
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
