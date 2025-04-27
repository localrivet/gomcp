// Package tool provides utilities for creating and registering MCP tools.
package tool

import (
	"context"
	"reflect"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/util/schema"
)

// ToolHandler is the interface that tool implementations should satisfy.
type ToolHandler interface {
	// Tool returns the tool definition.
	Tool() protocol.Tool
	// Handler returns the tool's handler function.
	// progressToken is interface{} to accept string or number per spec.
	Handler() func(ctx context.Context, progressToken interface{}, arguments any) ([]protocol.Content, bool)
}

// BaseTool provides a base implementation of ToolHandler.
type BaseTool struct {
	name        string
	description string
	// progressToken is interface{} to accept string or number per spec.
	handler func(ctx context.Context, progressToken interface{}, arguments any) ([]protocol.Content, bool)
}

// NewBaseTool creates a new base tool with the given name and description.
func NewBaseTool(name, description string) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
	}
}

// WithHandler sets the tool's handler function.
// progressToken is interface{} to accept string or number per spec.
func (t *BaseTool) WithHandler(handler func(ctx context.Context, progressToken interface{}, arguments any) ([]protocol.Content, bool)) *BaseTool {
	t.handler = handler
	return t
}

// Tool implements ToolHandler.Tool.
func (t *BaseTool) Tool() protocol.Tool {
	return protocol.Tool{
		Name:        t.name,
		Description: t.description,
		InputSchema: schema.FromStruct(reflect.TypeOf(t).Elem()),
	}
}

// Handler implements ToolHandler.Handler.
// progressToken is interface{} to accept string or number per spec.
func (t *BaseTool) Handler() func(ctx context.Context, progressToken interface{}, arguments any) ([]protocol.Content, bool) {
	return t.handler
}
