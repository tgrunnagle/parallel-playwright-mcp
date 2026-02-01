# Task DAG

## Design Document Reference

**Location**: `C:\Users\tgrun\Documents\GitHub\tgrunnagle\parallel-playwright-mcp\sunny-chasing-candy.md`

## Task Breakdown

| Task ID | Parent Task | Title | Description | Depends On | GitHub Issue # |
|---------|-------------|-------|-------------|------------|----------------|
| TASK-P001 | — | Project Foundation & Setup | Initialize Go module, directory structure, integrate mcp-go with streamable-http transport, basic server startup with health check | — | #3 |
| TASK-001 | TASK-P001 | Initialize Go module and dependencies | Create go.mod with module path, add mcp-go, playwright-go, and uuid dependencies | — | #2 |
| TASK-002 | TASK-P001 | Create project directory structure | Create cmd/server/, internal/browser/, internal/session/, internal/tools/, config/ directories with placeholder files | TASK-001 | #4 |
| TASK-003 | TASK-P001 | Implement basic MCP server with streamable-http | Create main.go with mcp-go server initialization, streamable-http transport on configurable host/port | TASK-002 | #9 |
| TASK-004 | TASK-P001 | Add server health check endpoint | Implement /health endpoint returning server status and version | TASK-003 | #13 |
| TASK-P002 | — | Browser Pool Implementation | Implement browser pool with lazy initialization for Chromium, Firefox, and WebKit; Playwright runtime management | TASK-P001 | #5 |
| TASK-005 | TASK-P002 | Define BrowserPool interface and types | Define BrowserPool interface, BrowserType enum, ContextOptions, PoolStats structs in internal/browser/pool.go | TASK-002 | #8 |
| TASK-006 | TASK-P002 | Implement Playwright runtime initialization | Implement Start() to initialize Playwright runtime, Stop() for cleanup | TASK-005 | #14 |
| TASK-007 | TASK-P002 | Implement lazy browser startup | Implement NewContext() with lazy browser launch on first request per browser type | TASK-006 | #26 |
| TASK-008 | TASK-P002 | Add multi-browser support | Extend pool to handle Chromium, Firefox, and WebKit with per-browser launch options | TASK-007 | #35 |
| TASK-009 | TASK-P002 | Implement pool statistics and monitoring | Implement Stats() returning running state, active contexts, total created/closed per browser type | TASK-008 | #39 |
| TASK-P003 | — | Browser Session Manager | Implement 1:N session model with session lifecycle (create, get, close, cleanup), MCP session ownership tracking | TASK-P002 | #7 |
| TASK-010 | TASK-P003 | Define session types and interfaces | Define BrowserSession struct, BrowserSessionManager interface, SessionOptions, SessionInfo in internal/session/ | TASK-005 | #15 |
| TASK-011 | TASK-P003 | Implement session creation | Implement CreateSession() with unique ID generation, MCP session ownership, browser context creation | TASK-010, TASK-007 | #36 |
| TASK-012 | TASK-P003 | Implement session retrieval and validation | Implement GetSession() with MCP session ownership validation | TASK-011 | #38 |
| TASK-013 | TASK-P003 | Implement session closing | Implement CloseSession() with browser context cleanup, CloseAllForMCP() for connection termination | TASK-012 | #43 |
| TASK-014 | TASK-P003 | Implement session cleanup and expiration | Implement Cleanup() for expired/orphaned sessions based on idle timeout | TASK-013 | #44 |
| TASK-015 | TASK-P003 | Implement session listing | Implement ListSessions() returning metadata (browser, URL, title, creation time) for MCP connection | TASK-012 | #42 |
| TASK-P004 | — | Session Management Tools | Implement session_create, session_list, session_close tools with mcp-go tool registration pattern | TASK-P003 | #11 |
| TASK-016 | TASK-P004 | Implement session_create tool | Register session_create with mcp-go, handle browserType, headless, viewport params, return sessionId | TASK-015 | #45 |
| TASK-017 | TASK-P004 | Implement session_list tool | Register session_list tool returning active sessions with metadata | TASK-016 | #49 |
| TASK-018 | TASK-P004 | Implement session_close tool | Register session_close tool, validate sessionId, cleanup resources | TASK-016 | #48 |
| TASK-P005 | — | Navigation Tools | Implement navigate, go_back, go_forward, reload tools with wait conditions and timeout support | TASK-P004 | #21 |
| TASK-019 | TASK-P005 | Implement navigate tool | Register navigate tool with url, waitUntil (load/domcontentloaded/networkidle), timeout params | TASK-018 | #52 |
| TASK-020 | TASK-P005 | Implement go_back tool | Register go_back tool for browser history navigation | TASK-019 | #58 |
| TASK-021 | TASK-P005 | Implement go_forward tool | Register go_forward tool for browser history navigation | TASK-019 | #60 |
| TASK-022 | TASK-P005 | Implement reload tool | Register reload tool with waitUntil option | TASK-019 | #59 |
| TASK-P006 | — | Interaction Tools | Implement click, type, fill, select_option, hover, press_key tools for user input simulation | TASK-P004 | #31 |
| TASK-023 | TASK-P006 | Implement click tool | Register click tool with selector, button (left/right/middle), clickCount params | TASK-018 | #53 |
| TASK-024 | TASK-P006 | Implement type tool | Register type tool with selector, text, delay (keystroke delay) params | TASK-023 | #62 |
| TASK-025 | TASK-P006 | Implement fill tool | Register fill tool for fast input field population (no keystroke events) | TASK-023 | #66 |
| TASK-026 | TASK-P006 | Implement select_option tool | Register select_option tool for dropdown selection by value/label/index | TASK-023 | #64 |
| TASK-027 | TASK-P006 | Implement hover tool | Register hover tool for mouse hover over element | TASK-023 | #63 |
| TASK-028 | TASK-P006 | Implement press_key tool | Register press_key tool with key and modifiers (Ctrl, Shift, Alt, Meta) | TASK-023 | #67 |
| TASK-P007 | — | Inspection Tools | Implement screenshot, extract_text, get_html, evaluate, query_selector tools for page content extraction | TASK-P004 | #30 |
| TASK-029 | TASK-P007 | Implement screenshot tool | Register screenshot tool with fullPage, selector, and viewport options, return base64 PNG | TASK-018 | #54 |
| TASK-030 | TASK-P007 | Implement extract_text tool | Register extract_text tool returning hierarchical text structure with tag names | TASK-029 | #65 |
| TASK-031 | TASK-P007 | Implement get_html tool | Register get_html tool with optional selector and outer (innerHTML vs outerHTML) option | TASK-029 | #61 |
| TASK-032 | TASK-P007 | Implement evaluate tool | Register evaluate tool for executing JavaScript expressions in page context | TASK-029 | #68 |
| TASK-033 | TASK-P007 | Implement query_selector tool | Register query_selector tool returning element info (tag, attributes, text, bounding box) | TASK-029 | #71 |
| TASK-034 | TASK-P007 | Implement get_accessibility_tree tool | Register get_accessibility_tree tool returning accessibility snapshot | TASK-029 | #70 |
| TASK-066 | TASK-P007 | Implement navigate_and_extract_text tool | Register navigate_and_extract_text tool that creates a temporary session, navigates to URL, extracts text, and cleans up session. Does not require sessionId. Params: url, waitUntil, selector (optional), browserType (optional) | TASK-030, TASK-019, TASK-016 | #77 |
| TASK-P008 | — | Network Logging | Implement network request/response capture with circular buffer and get_network_logs tool | TASK-P004 | #28 |
| TASK-035 | TASK-P008 | Implement NetworkLogBuffer | Create circular buffer for storing network log entries with configurable max size | TASK-010 | #25 |
| TASK-036 | TASK-P008 | Implement network request/response capture | Hook into page request/response events, capture method, URL, status, duration, sizes | TASK-035, TASK-011 | #40 |
| TASK-037 | TASK-P008 | Implement get_network_logs tool | Register get_network_logs tool with limit and filter (URL pattern, status code) options | TASK-036, TASK-018 | #56 |
| TASK-P009 | — | Multi-Tab Support | Implement tab_new, tab_list, tab_switch, tab_close tools for managing multiple tabs per session | TASK-P004 | #29 |
| TASK-038 | TASK-P009 | Extend session for multi-tab tracking | Add Pages map and ActiveTabID to BrowserSession, update session creation to track initial tab | TASK-013 | #46 |
| TASK-039 | TASK-P009 | Implement tab_new tool | Register tab_new tool creating new page in session, optional initial URL | TASK-038, TASK-018 | #57 |
| TASK-040 | TASK-P009 | Implement tab_list tool | Register tab_list tool returning tab IDs with URLs and titles | TASK-039 | #69 |
| TASK-041 | TASK-P009 | Implement tab_switch tool | Register tab_switch tool to change active tab by tabId | TASK-039 | #72 |
| TASK-042 | TASK-P009 | Implement tab_close tool | Register tab_close tool, handle closing active tab (switch to another) | TASK-039 | #75 |
| TASK-P010 | — | SSE Streaming & Progress Notifications | Implement SSE streaming for long-running operations and progress notifications | TASK-P005 | #33 |
| TASK-043 | TASK-P010 | Configure SSE transport for streaming responses | Enable SSE streaming in mcp-go server configuration for GET /mcp endpoint | TASK-003 | #20 |
| TASK-044 | TASK-P010 | Implement progress notifications for navigation | Emit progress notifications during page load (started, domcontentloaded, load complete) | TASK-043, TASK-019 | #73 |
| TASK-045 | TASK-P010 | Implement progress notifications for screenshots | Emit progress notifications for screenshot capture (capturing, encoding, complete) | TASK-044, TASK-029 | #76 |
| TASK-P011 | — | Console Log Capture | Capture and expose browser console logs per session | TASK-P003 | #12 |
| TASK-046 | TASK-P011 | Implement console log buffer | Create circular buffer for console log entries per session | TASK-010 | #23 |
| TASK-047 | TASK-P011 | Hook into page console events | Capture console.log/warn/error/info messages with timestamps and log level | TASK-046, TASK-011 | #41 |
| TASK-048 | TASK-P011 | Expose console logs via get_console_logs tool | Register get_console_logs tool with limit and level filter options | TASK-047, TASK-018 | #55 |
| TASK-P012 | — | Configuration System | Implement config file (YAML) and environment variable support for server, browser, and session settings | TASK-P001 | #6 |
| TASK-049 | TASK-P012 | Define configuration struct | Create Config struct matching config.yaml schema (server, browser, session, logging sections) | TASK-002 | #10 |
| TASK-050 | TASK-P012 | Implement YAML config file loading | Load config from config.yaml with sensible defaults when file missing | TASK-049 | #17 |
| TASK-051 | TASK-P012 | Implement environment variable overrides | Support MCP_HOST, MCP_PORT, MCP_BROWSER_HEADLESS, MCP_LOG_LEVEL env vars overriding config | TASK-050 | #22 |
| TASK-052 | TASK-P012 | Add configuration validation | Validate config values (port range, valid browser types, timeout ranges) with clear error messages | TASK-051 | #34 |
| TASK-P013 | — | Error Handling & Recovery | Implement comprehensive error codes, actionable error messages, and retry logic for transient failures | TASK-P005, TASK-P006, TASK-P007 | #32 |
| TASK-053 | TASK-P013 | Define custom error codes and types | Define error codes (-32001 to -32004) and custom error types for session, element, timeout, navigation errors | TASK-003 | #18 |
| TASK-054 | TASK-P013 | Implement error response formatting | Format errors with code, message, and data (context, suggestions) per MCP spec | TASK-053 | #24 |
| TASK-055 | TASK-P013 | Add retry logic for transient failures | Implement configurable retry for navigation timeouts and element not found (with backoff) | TASK-054, TASK-019 | #74 |
| TASK-056 | TASK-P013 | Implement timeout handling across tools | Ensure consistent timeout handling with context cancellation for all tools | TASK-055 | #79 |
| TASK-P014 | — | Graceful Shutdown & Cleanup | Implement clean shutdown with proper resource cleanup, session termination, and browser process management | TASK-P003 | #16 |
| TASK-057 | TASK-P014 | Implement signal handling | Handle SIGTERM, SIGINT signals to trigger graceful shutdown | TASK-003 | #19 |
| TASK-058 | TASK-P014 | Implement ordered shutdown sequence | Shutdown in order: stop accepting connections → close sessions → close browsers → stop Playwright | TASK-057, TASK-013, TASK-006 | #47 |
| TASK-059 | TASK-P014 | Add connection draining | Wait for in-flight requests to complete with configurable drain timeout | TASK-058 | #51 |
| TASK-060 | TASK-P014 | Ensure cleanup on panic | Add defer/recover to clean up resources on unexpected panics | TASK-058 | #50 |
| TASK-P015 | — | E2E Test Suite | Implement end-to-end tests for full automation workflows, parallel sessions, and error recovery scenarios | TASK-P005, TASK-P006, TASK-P007, TASK-P009 | #37 |
| TASK-061 | TASK-P015 | Set up E2E test infrastructure | Create e2e/ directory with test helpers, MCP client setup, test server lifecycle management | TASK-004 | #27 |
| TASK-062 | TASK-P015 | Test full automation workflow | E2E test: session_create → navigate → click → type → screenshot → extract_text → session_close | TASK-061, TASK-019, TASK-023, TASK-029 | #78 |
| TASK-063 | TASK-P015 | Test parallel sessions with different browsers | E2E test: Create 3 sessions (Chromium, Firefox, WebKit), parallel operations, verify isolation | TASK-062 | #80 |
| TASK-064 | TASK-P015 | Test multi-tab workflows | E2E test: Create session → open multiple tabs → switch between tabs → close tabs → verify state | TASK-063, TASK-039 | #82 |
| TASK-065 | TASK-P015 | Test error handling scenarios | E2E test: Invalid session, element not found, navigation timeout, verify error codes and messages | TASK-062, TASK-053 | #81 |
