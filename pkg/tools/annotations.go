package tools

import "github.com/mark3labs/mcp-go/mcp"

// toolHints are a tool's MCP annotation hints. mcp.NewTool defaults every tool
// to readOnly false, destructive true, idempotent false, and openWorld true, so
// every tool sets all four explicitly. Clients may use the hints to decide what
// to auto-approve: a tool that can submit a form or trigger a page action stays
// destructive.
type toolHints struct {
	// readOnly is true when the tool does not change the page or the session.
	readOnly bool
	// destructive is true when the tool may make a change it cannot undo. It is
	// meaningful only when readOnly is false; read-only tools set it false.
	destructive bool
	// idempotent is true when repeating the call with the same arguments has no
	// further effect. It is meaningful only when readOnly is false.
	idempotent bool
	// openWorld is true when the tool reaches or reads web content, which this
	// server does not control.
	openWorld bool
}

// Hint sets shared by groups of tools.
var (
	// readOnlyLocalHints is for tools that read server-held state only.
	readOnlyLocalHints = toolHints{readOnly: true}
	// readOnlyPageHints is for tools that read the page without changing it.
	readOnlyPageHints = toolHints{readOnly: true, openWorld: true}
	// pageChangeHints is for tools that load or move between pages but do not
	// submit anything on the user's behalf.
	pageChangeHints = toolHints{openWorld: true}
	// pageActionHints is for tools that act on the page and can submit forms
	// or trigger page actions.
	pageActionHints = toolHints{destructive: true, openWorld: true}
	// sessionCreateHints is for session_create: it adds a session each call, so
	// it is not idempotent, but it removes nothing and loads no page.
	sessionCreateHints = toolHints{}
	// sessionCloseHints is for session_close: it discards the session's state for
	// good, and closing a closed session changes nothing further.
	sessionCloseHints = toolHints{destructive: true, idempotent: true}
)

// option returns the tool option that sets all four hints.
func (h toolHints) option() mcp.ToolOption {
	return func(t *mcp.Tool) {
		t.Annotations.ReadOnlyHint = mcp.ToBoolPtr(h.readOnly)
		t.Annotations.DestructiveHint = mcp.ToBoolPtr(h.destructive)
		t.Annotations.IdempotentHint = mcp.ToBoolPtr(h.idempotent)
		t.Annotations.OpenWorldHint = mcp.ToBoolPtr(h.openWorld)
	}
}
