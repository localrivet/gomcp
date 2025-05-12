package server_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/localrivet/gomcp/server"
	"github.com/stretchr/testify/assert"
	// Add necessary imports later
)

// TODO: Test RegisterResource
func TestRegistry_RegisterResource(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test GetResource
func TestRegistry_GetResource(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test ResourceRegistry
func TestRegistry_ResourceRegistry(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test AddRoot
func TestRegistry_AddRoot(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test GetRoots
func TestRegistry_GetRoots(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test AddPrompt
func TestRegistry_AddPrompt(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test GetPrompt
func TestRegistry_GetPrompt(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test GetPrompts
func TestRegistry_GetPrompts(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test AddTool (validation)
func TestRegistry_Tool_Valid(t *testing.T) {
	r := server.NewRegistry()

	// Set up a callback to verify it gets called
	callbackCalled := false
	r.SetToolChangedCallback(func() {
		callbackCalled = true
	})

	// Define a valid tool handler function with a struct similar to the one in the example
	handler := func(ctx *server.Context, args struct {
		Topic string `json:"topic"`
	}) (interface{}, error) {
		return fmt.Sprintf("Fact about %s", args.Topic), nil
	}

	// Register the tool
	toolName := "fact"
	toolDesc := "Get a fact about a topic"
	r.Tool(toolName, toolDesc, handler)

	// Verify the tool was registered
	tools := r.GetTools()
	assert.Len(t, tools, 1, "Should have registered exactly one tool")

	// Check the registered tool has correct properties
	tool := tools[0]
	assert.Equal(t, toolName, tool.Name)
	assert.Equal(t, toolDesc, tool.Description)

	// Check that properties are present in the schema
	assert.Equal(t, "object", tool.InputSchema.Type, "Schema should have type 'object'")

	// The key issue: verify properties are not empty
	assert.NotEmpty(t, tool.InputSchema.Properties, "Schema properties should not be empty")

	// Verify the specific property we expect
	topicProp, exists := tool.InputSchema.Properties["topic"]
	assert.True(t, exists, "Schema should have a 'topic' property")
	assert.Equal(t, "string", topicProp.Type, "The topic property should be of type string")

	// Verify the required field contains our property
	assert.Contains(t, tool.InputSchema.Required, "topic", "The topic field should be required")

	// Verify handler was stored and can be retrieved
	handlerFn, exists := r.GetToolHandler(toolName)
	assert.True(t, exists, "Handler should be registered")
	assert.NotNil(t, handlerFn, "Handler function should not be nil")

	// Verify callback was called
	assert.True(t, callbackCalled, "Tool changed callback should have been called")
}

func TestRegistry_Tool_Invalid(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test GetToolHandler
func TestRegistry_GetToolHandler(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test ToolRegistry
func TestRegistry_ToolRegistry(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test SetPromptChangedCallback
func TestRegistry_SetPromptChangedCallback(t *testing.T) { t.Skip("Test not implemented") }

// TODO: Test SetResourceChangedCallback
func TestRegistry_SetResourceChangedCallback(t *testing.T) { t.Skip("Test not implemented") }

// --- Resource Template Tests ---

func TestRegistry_AddResourceTemplate_Success(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{id}"
	handler := func(ctx *server.Context, id string) (string, error) {
		return fmt.Sprintf("Item: %s", id), nil
	}

	err := r.AddResourceTemplate(pattern, handler)
	assert.NoError(t, err)

	templateRegistry := r.TemplateRegistry()
	info, ok := templateRegistry[pattern]
	assert.True(t, ok, "Template should be registered")
	assert.Equal(t, pattern, info.Pattern)
	assert.NotNil(t, info.HandlerFn)
	assert.NotNil(t, info.Matcher)
	assert.Equal(t, 0, info.ContextArgIndex)
	assert.Len(t, info.Params, 1)
	assert.Equal(t, "id", info.Params[0].Name)
	assert.Equal(t, 1, info.Params[0].HandlerIndex)
	assert.Equal(t, reflect.TypeOf(""), info.Params[0].HandlerType) // Check type is string
}

func TestRegistry_AddResourceTemplate_InvalidPattern(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{{id}"
	handler := func(ctx *server.Context, id string) (string, error) { return id, nil }

	err := r.AddResourceTemplate(pattern, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URI template pattern")
}

func TestRegistry_AddResourceTemplate_DuplicatePattern(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{id}"
	handler1 := func(ctx *server.Context, id string) (string, error) { return id, nil }
	handler2 := func(ctx *server.Context, id int) (int, error) { return id, nil }

	err1 := r.AddResourceTemplate(pattern, handler1)
	assert.NoError(t, err1)

	err2 := r.AddResourceTemplate(pattern, handler2)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "is already registered")
}

func TestRegistry_AddResourceTemplate_InvalidHandler_NotFunc(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{id}"
	handler := "not a function"

	err := r.AddResourceTemplate(pattern, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a function")
}

func TestRegistry_AddResourceTemplate_InvalidHandler_NoArgs(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{id}"
	handler := func() (string, error) { return "", nil }

	err := r.AddResourceTemplate(pattern, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must accept at least context.Context or *server.Context as the first argument")
}

func TestRegistry_AddResourceTemplate_InvalidHandler_NoContext(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{id}"
	handler := func(id string) (string, error) { return id, nil }

	err := r.AddResourceTemplate(pattern, handler)
	// Assert that the error message indicates the specific issue
	// Updated expected message based on modified check in AddResourceTemplate
	expectedErrorSubstr := "must accept context.Context, *server.Context, or interface{} as the first argument"
	assert.ErrorContains(t, err, expectedErrorSubstr)
}

func TestRegistry_AddResourceTemplate_InvalidHandler_WrongReturnType(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{id}"
	handler := func(ctx *server.Context, id string) string { return id }

	err := r.AddResourceTemplate(pattern, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must return exactly two values")
}

func TestRegistry_AddResourceTemplate_InvalidHandler_WrongErrorType(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{id}"
	handler := func(ctx *server.Context, id string) (string, int) { return id, 0 }

	err := r.AddResourceTemplate(pattern, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must return error as the second value")
}

func TestRegistry_AddResourceTemplate_ParamCountMismatch(t *testing.T) {
	r := server.NewRegistry()
	pattern := "test://item/{id}/{name}"                                               // 2 params in pattern
	handler := func(ctx *server.Context, id string) (string, error) { return id, nil } // 1 param in handler

	err := r.AddResourceTemplate(pattern, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parameters (excluding Context), but template expects")
}

// TODO: Add test for TemplateRegistry() getter method?
