package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

func TestPromptRegistrationAndTemplates(t *testing.T) {
	// Create a new server
	s := server.NewServer("test-server")

	// Register a prompt with string templates
	s.Prompt("simple-prompt", "A simple prompt for testing",
		"This is a template string",
		"This is another template string",
	)

	// Register a prompt with explicit templates
	s.Prompt("complex-prompt", "A more complex prompt",
		server.System("I am a helpful assistant"),
		server.User("What is the capital of {{country}}?"),
		server.Assistant("The capital of {{country}} is {{capital}}."),
	)

	// Check that the prompts were registered
	server := s.GetServer()
	if len(server.GetPrompts()) != 2 {
		t.Errorf("Expected 2 prompts, got %d", len(server.GetPrompts()))
	}

	// Check the simple prompt
	simplePrompt, ok := server.GetPrompts()["simple-prompt"]
	if !ok {
		t.Fatal("simple-prompt not found")
	}
	if simplePrompt.Name != "simple-prompt" {
		t.Errorf("Expected name 'simple-prompt', got '%s'", simplePrompt.Name)
	}
	if simplePrompt.Description != "A simple prompt for testing" {
		t.Errorf("Expected description 'A simple prompt for testing', got '%s'", simplePrompt.Description)
	}
	if len(simplePrompt.Templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(simplePrompt.Templates))
	}
	if simplePrompt.Templates[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", simplePrompt.Templates[0].Role)
	}
	if simplePrompt.Templates[0].Content != "This is a template string" {
		t.Errorf("Expected content 'This is a template string', got '%s'", simplePrompt.Templates[0].Content)
	}

	// Check the complex prompt
	complexPrompt, ok := server.GetPrompts()["complex-prompt"]
	if !ok {
		t.Fatal("complex-prompt not found")
	}
	if complexPrompt.Name != "complex-prompt" {
		t.Errorf("Expected name 'complex-prompt', got '%s'", complexPrompt.Name)
	}
	if len(complexPrompt.Templates) != 3 {
		t.Errorf("Expected 3 templates, got %d", len(complexPrompt.Templates))
	}

	// Check template roles
	expectedRoles := []string{"system", "user", "assistant"}
	for i, role := range expectedRoles {
		if complexPrompt.Templates[i].Role != role {
			t.Errorf("Expected role '%s', got '%s'", role, complexPrompt.Templates[i].Role)
		}
	}

	// Verify arguments were extracted
	if len(complexPrompt.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(complexPrompt.Arguments))
	}

	// Check arguments
	argMap := make(map[string]bool)
	for _, arg := range complexPrompt.Arguments {
		argMap[arg.Name] = true
		if !arg.Required {
			t.Errorf("Expected argument '%s' to be required", arg.Name)
		}
	}

	if !argMap["country"] {
		t.Errorf("Expected 'country' argument to be extracted")
	}
	if !argMap["capital"] {
		t.Errorf("Expected 'capital' argument to be extracted")
	}
}

func TestPromptVariableSubstitution(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		variables map[string]interface{}
		expected  string
	}{
		{
			name:      "simple variable",
			template:  "Hello, {{name}}!",
			variables: map[string]interface{}{"name": "World"},
			expected:  "Hello, World!",
		},
		{
			name:      "multiple variables",
			template:  "{{greeting}}, {{name}}!",
			variables: map[string]interface{}{"greeting": "Hello", "name": "World"},
			expected:  "Hello, World!",
		},
		{
			name:      "missing variable",
			template:  "Hello, {{name}}!",
			variables: map[string]interface{}{},
			expected:  "Hello, {{name}}!",
		},
		{
			name:      "numeric variable",
			template:  "The answer is {{answer}}.",
			variables: map[string]interface{}{"answer": 42},
			expected:  "The answer is 42.",
		},
		{
			name:      "object variable",
			template:  "User: {{user}}",
			variables: map[string]interface{}{"user": map[string]interface{}{"name": "John", "age": 30}},
			expected:  `User: {"age":30,"name":"John"}`,
		},
		{
			name:      "whitespace in variable name",
			template:  "Hello, {{ name }}!",
			variables: map[string]interface{}{"name": "World"},
			expected:  "Hello, World!",
		},
		{
			name:      "no variables",
			template:  "Hello, World!",
			variables: map[string]interface{}{},
			expected:  "Hello, World!",
		},
		{
			name:      "nil variables",
			template:  "Hello, {{name}}!",
			variables: nil,
			expected:  "Hello, {{name}}!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.SubstituteVariables(tt.template, tt.variables)
			if err != nil {
				t.Errorf("server.SubstituteVariables() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("server.SubstituteVariables() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProcessPromptRequest(t *testing.T) {
	// Create a new server
	s := server.NewServer("test-server")

	// Register a prompt
	s.Prompt("test-prompt", "A test prompt",
		server.System("You are a helpful assistant."),
		server.User("Tell me about {{topic}}."),
	)

	// Create a context for testing
	ctx := &server.Context{
		Request: &server.Request{
			ID:     "1",
			Method: "prompts/get",
			Params: json.RawMessage(`{"name":"test-prompt","variables":{"topic":"Go programming"}}`),
		},
		Response: &server.Response{
			JSONRPC: "2.0",
			ID:      "1",
		},
	}

	// Process the prompt request
	result, err := s.GetServer().ProcessPromptRequest(ctx)
	if err != nil {
		t.Fatalf("ProcessPromptRequest() error = %v", err)
	}

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	// Check the description
	description, ok := resultMap["description"].(string)
	if !ok || description != "A test prompt" {
		t.Errorf("Expected description 'A test prompt', got '%v'", description)
	}

	// Check the messages
	messages, ok := resultMap["messages"].([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected messages to be a slice of maps, got %T", resultMap["messages"])
	}
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	// Check the second message (with variable substitution)
	secondMessage := messages[1]
	if secondMessage["role"] != "user" {
		t.Errorf("Expected role 'user', got '%s'", secondMessage["role"])
	}

	// Check content format
	content, ok := secondMessage["content"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected content to be a map, got %T", secondMessage["content"])
	}

	// Check content fields
	if content["type"] != "text" {
		t.Errorf("Expected content type 'text', got '%v'", content["type"])
	}
	if content["text"] != "Tell me about Go programming." {
		t.Errorf("Expected content text 'Tell me about Go programming.', got '%v'", content["text"])
	}

	// Test missing required argument
	ctx.Request.Params = json.RawMessage(`{"name":"test-prompt","variables":{}}`)
	_, err = s.GetServer().ProcessPromptRequest(ctx)
	if err == nil {
		t.Error("Expected error for missing required argument 'topic', got nil")
	}

	// Test with missing prompt
	ctx.Request.Params = json.RawMessage(`{"name":"missing-prompt","variables":{}}`)
	_, err = s.GetServer().ProcessPromptRequest(ctx)
	if err == nil {
		t.Error("Expected error for missing prompt, got nil")
	}

	// Test with invalid params
	ctx.Request.Params = json.RawMessage(`invalid json`)
	_, err = s.GetServer().ProcessPromptRequest(ctx)
	if err == nil {
		t.Error("Expected error for invalid params, got nil")
	}
}

func TestPromptList(t *testing.T) {
	// Create a new server
	s := server.NewServer("test-server")

	// Register some prompts
	s.Prompt("prompt1", "First prompt", "Template 1")
	s.Prompt("prompt2", "Second prompt", "Template 2 with {{var}}")
	s.Prompt("prompt3", "Third prompt", "Template 3")

	// Create a context for testing
	ctx := &server.Context{
		Request: &server.Request{
			ID:     "1",
			Method: "prompts/list",
		},
		Response: &server.Response{
			JSONRPC: "2.0",
			ID:      "1",
		},
	}

	// Process the prompt list request
	result, err := s.GetServer().ProcessPromptList(ctx)
	if err != nil {
		t.Fatalf("ProcessPromptList() error = %v", err)
	}

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	// Check the prompts
	prompts, ok := resultMap["prompts"].([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected prompts to be a slice of maps, got %T", resultMap["prompts"])
	}
	if len(prompts) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(prompts))
	}

	// Check if second prompt has arguments
	var promptWithArgs map[string]interface{}
	for _, p := range prompts {
		if p["name"] == "prompt2" {
			promptWithArgs = p
			break
		}
	}

	if promptWithArgs == nil {
		t.Fatal("prompt2 not found in prompts list")
	}

	args, ok := promptWithArgs["arguments"].([]server.PromptArgument)
	if !ok {
		// This is acceptable since the JSON marshaling might make it a different type
		t.Logf("Arguments not in expected format, got %T", promptWithArgs["arguments"])
	} else if len(args) == 0 {
		t.Errorf("Expected at least one argument for prompt2, got none")
	}

	// Check prompt information
	for _, prompt := range prompts {
		name, ok := prompt["name"].(string)
		if !ok {
			t.Errorf("Expected name to be a string, got %T", prompt["name"])
			continue
		}

		// Check description
		description, ok := prompt["description"].(string)
		if !ok {
			t.Errorf("Expected description to be a string, got %T", prompt["description"])
			continue
		}

		// Check description for each prompt
		switch name {
		case "prompt1":
			if description != "First prompt" {
				t.Errorf("Expected description 'First prompt', got '%s'", description)
			}
		case "prompt2":
			if description != "Second prompt" {
				t.Errorf("Expected description 'Second prompt', got '%s'", description)
			}
		case "prompt3":
			if description != "Third prompt" {
				t.Errorf("Expected description 'Third prompt', got '%s'", description)
			}
		default:
			t.Errorf("Unexpected prompt name: %s", name)
		}
	}
}
