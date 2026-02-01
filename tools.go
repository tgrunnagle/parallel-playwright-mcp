//go:build tools
// +build tools

// Package tools pins tool and library dependencies for go mod.
// This file is not compiled into the binary.
package tools

import (
	_ "github.com/google/uuid"
	_ "github.com/mark3labs/mcp-go/mcp"
	_ "github.com/playwright-community/playwright-go"
)
