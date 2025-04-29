package server

import (
	"context"
	"fmt"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/util/schema"
)

// --- Helper Functions ---

// Basic function types
type ToolFunc[T any] func(T) (protocol.Content, error)
type PromptFunc[T any] func(T) (protocol.PromptMessage, error)
type ResourceFunc func() (protocol.ResourceContents, error)

// Middleware types
type ToolMiddleware[T any] func(ToolFunc[T]) ToolFunc[T]

// Response helpers to hide protocol details
func Text(s string) protocol.Content {
	return protocol.TextContent{Type: "text", Text: s}
}

func Message(role string, content string) protocol.PromptMessage {
	return protocol.PromptMessage{
		Role:    role,
		Content: []protocol.Content{Text(content)},
	}
}

// AddTool provides a simpler API for adding tools
func AddTool[T any](
	srv *Server,
	name string,
	description string,
	handler ToolFunc[T],
) error {
	wrappedHandler := func(ctx context.Context, _ interface{}, arguments any) ([]protocol.Content, bool) {
		args, errContent, isErr := schema.HandleArgs[T](arguments)
		if isErr {
			return errContent, true
		}

		if args == nil {
			return []protocol.Content{Text("Error: nil arguments")}, true
		}

		content, err := handler(*args)
		if err != nil {
			return []protocol.Content{Text(fmt.Sprintf("Error: %v", err))}, true
		}

		return []protocol.Content{content}, false
	}

	return srv.RegisterTool(protocol.Tool{
		Name:        name,
		Description: description,
		InputSchema: schema.FromStruct(*new(T)),
	}, wrappedHandler)
}

// AddPrompt provides a simpler API for adding prompts
func AddPrompt[T any](
	srv *Server,
	name string,
	description string,
	handler PromptFunc[T],
) error {
	return srv.RegisterPrompt(protocol.Prompt{
		URI:         fmt.Sprintf("prompt://%s", name),
		Title:       name,
		Description: description,
		Messages:    []protocol.PromptMessage{},
	})
}

// AddResource provides a simpler API for adding resources
func AddResource(
	srv *Server,
	uri string,
	kind string,
	description string,
	version string,
	handler ResourceFunc,
) error {
	return srv.RegisterResource(protocol.Resource{
		URI:         uri,
		Kind:        kind,
		Title:       kind,
		Description: description,
		Version:     version,
	})
}
