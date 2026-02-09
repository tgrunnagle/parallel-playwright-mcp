package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	// DefaultClientTimeout is the default timeout for HTTP requests.
	// Set high enough to accommodate browser operations.
	DefaultClientTimeout = 120 * time.Second

	// MCPProtocolVersion is the MCP protocol version used in initialize requests.
	MCPProtocolVersion = "2024-11-05"
)

// MCPClient provides methods for interacting with the MCP server.
type MCPClient struct {
	baseURL    string
	httpClient *http.Client
	sessionID  string // MCP session ID from Mcp-Session-Id header
	requestID  atomic.Int64
}

// MCPClientOption configures an MCPClient.
type MCPClientOption func(*MCPClient)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) MCPClientOption {
	return func(c *MCPClient) {
		c.httpClient = client
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) MCPClientOption {
	return func(c *MCPClient) {
		c.httpClient.Timeout = timeout
	}
}

// NewMCPClient creates a new MCP test client.
func NewMCPClient(baseURL string, opts ...MCPClientOption) *MCPClient {
	c := &MCPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: DefaultClientTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// InitializeParams contains parameters for the initialize request.
type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      ClientInfo     `json:"clientInfo"`
}

// ClientInfo contains client identification information.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult contains the result of the initialize request.
type InitializeResult struct {
	ServerInfo   ServerInfo     `json:"serverInfo"`
	Capabilities map[string]any `json:"capabilities"`
}

// ServerInfo contains server identification information.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolCallParams contains parameters for a tools/call request.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in a tool result.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // For base64 image data
}

// Initialize performs the MCP initialization handshake.
func (c *MCPClient) Initialize(ctx context.Context) (*InitializeResult, error) {
	params := InitializeParams{
		ProtocolVersion: MCPProtocolVersion,
		Capabilities:    map[string]any{},
		ClientInfo: ClientInfo{
			Name:    "e2e-test-client",
			Version: "1.0.0",
		},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("initialize request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	return &result, nil
}

// CallTool invokes an MCP tool and returns the result.
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	params := ToolCallParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("tool call request failed: %w", err)
	}

	if resp.Error != nil {
		return &ToolResult{
			IsError: true,
			Content: []ContentBlock{{Type: "text", Text: resp.Error.Error()}},
		}, resp.Error
	}

	var result ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	return &result, nil
}

// ListTools retrieves the list of available tools from the server.
func (c *MCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("list tools request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools list: %w", err)
	}

	return result.Tools, nil
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// Close terminates the MCP session.
func (c *MCPClient) Close(ctx context.Context) error {
	if c.sessionID == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create close request: %w", err)
	}

	req.Header.Set("Mcp-Session-Id", c.sessionID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("close request failed: %w", err)
	}
	defer resp.Body.Close()

	// 2xx status codes indicate success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("close request failed with status %d", resp.StatusCode)
}

// SessionID returns the MCP session ID.
func (c *MCPClient) SessionID() string {
	return c.sessionID
}

// sendRequest sends a JSON-RPC request to the MCP server.
func (c *MCPClient) sendRequest(ctx context.Context, method string, params any) (*JSONRPCResponse, error) {
	reqID := c.requestID.Add(1)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Store session ID from response header
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.sessionID = sessionID
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &rpcResp, nil
}

// CreateSession is a convenience method to create a browser session.
func (c *MCPClient) CreateSession(ctx context.Context, browserType string, headless bool) (string, error) {
	args := map[string]any{
		"browserType": browserType,
		"headless":    headless,
	}

	result, err := c.CallTool(ctx, "session_create", args)
	if err != nil {
		return "", err
	}

	if result.IsError || len(result.Content) == 0 {
		return "", fmt.Errorf("session_create failed: %v", result.Content)
	}

	// Extract session ID from response text
	// The response is typically "Created session: sess-xyz"
	text := result.Content[0].Text
	var sessionID string
	if _, err := fmt.Sscanf(text, "Created session: %s", &sessionID); err != nil {
		// Try to extract from JSON if it's a structured response
		var structured struct {
			SessionID string `json:"sessionId"`
		}
		if jsonErr := json.Unmarshal([]byte(text), &structured); jsonErr == nil {
			sessionID = structured.SessionID
		} else {
			return "", fmt.Errorf("failed to extract session ID from: %s", text)
		}
	}

	return sessionID, nil
}

// Navigate is a convenience method to navigate to a URL.
// Uses domcontentloaded wait condition for faster navigation.
func (c *MCPClient) Navigate(ctx context.Context, sessionID, url string) error {
	args := map[string]any{
		"sessionId": sessionID,
		"url":       url,
		"waitUntil": "domcontentloaded", // Use domcontentloaded for faster tests
	}

	result, err := c.CallTool(ctx, "navigate", args)
	if err != nil {
		return err
	}

	if result.IsError {
		return fmt.Errorf("navigate failed: %v", result.Content)
	}

	return nil
}

// CloseSession is a convenience method to close a browser session.
func (c *MCPClient) CloseSession(ctx context.Context, sessionID string) error {
	args := map[string]any{
		"sessionId": sessionID,
	}

	result, err := c.CallTool(ctx, "session_close", args)
	if err != nil {
		return err
	}

	if result.IsError {
		return fmt.Errorf("session_close failed: %v", result.Content)
	}

	return nil
}
