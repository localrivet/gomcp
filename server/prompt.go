package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// InvalidParametersError represents an error with invalid parameters
// for prompt rendering or template variable substitution.
type InvalidParametersError struct {
	// Message contains the error description
	Message string
}

// Error returns the error message string.
// This method implements the error interface.
func (e *InvalidParametersError) Error() string {
	return e.Message
}

// NewInvalidParametersError creates a new InvalidParametersError with the given message.
// This is used when prompt parameters are missing or invalid.
func NewInvalidParametersError(message string) *InvalidParametersError {
	return &InvalidParametersError{Message: message}
}

// ContentType represents the type of content in a prompt message.
// Different content types have different required fields and rendering behaviors.
type ContentType string

// Content type constants define the supported content types for prompts.
const (
	// ContentTypeText is used for plain text content
	ContentTypeText ContentType = "text"

	// ContentTypeImage is used for image content, which requires an imageUrl
	ContentTypeImage ContentType = "image"

	// ContentTypeAudio is used for audio content, which requires audio data
	ContentTypeAudio ContentType = "audio"

	// ContentTypeResource is used for referencing resources by URI
	ContentTypeResource ContentType = "resource"
)

// PromptContent represents the content of a prompt message.
// It defines a block of content with a specific type and associated data.
type PromptContent struct {
	// Type specifies the kind of content (text, image, audio, resource)
	Type ContentType `json:"type"`

	// Text contains the text content when Type is ContentTypeText
	Text string `json:"text,omitempty"`

	// Data contains binary data encoded as base64 for non-text content
	Data string `json:"data,omitempty"`

	// MimeType specifies the format of the Data field
	MimeType string `json:"mimeType,omitempty"`

	// Resource contains resource reference information when Type is ContentTypeResource
	Resource map[string]interface{} `json:"resource,omitempty"`
}

// PromptTemplate represents a template for a prompt with a role and content.
// Templates can contain variables in the format {{variable}} which are
// substituted when the prompt is rendered.
type PromptTemplate struct {
	// Role defines who is speaking in this template (system, user, assistant)
	Role string

	// Content contains the template text with variables in {{variable}} format
	Content string

	// Variables holds the variable names extracted from the Content
	Variables []string
}

// PromptArgument represents an argument for a prompt.
// Arguments are defined by variable names in prompt templates.
type PromptArgument struct {
	// Name is the identifier for the argument, matching {{name}} in templates
	Name string `json:"name"`

	// Description explains what the argument is for
	Description string `json:"description"`

	// Required indicates whether the argument must be provided
	Required bool `json:"required"`
}

// Prompt represents a prompt registered with the server.
// A prompt is a named collection of templates that can be rendered with
// provided variable values.
type Prompt struct {
	// Name is the unique identifier for this prompt
	Name string

	// Description explains what the prompt is for
	Description string

	// Templates are the ordered sequence of message templates that make up the prompt
	Templates []PromptTemplate

	// Arguments are the parameters that can be passed when rendering the prompt
	Arguments []PromptArgument
}

// System creates a system prompt template.
// System prompts provide context or instructions to the language model.
func System(content string) PromptTemplate {
	return PromptTemplate{Role: "system", Content: content}
}

// User creates a user prompt template.
// User prompts represent messages from the user to the language model.
func User(content string) PromptTemplate {
	return PromptTemplate{Role: "user", Content: content}
}

// Assistant creates an assistant prompt template.
// Assistant prompts represent previous or example responses from the language model.
func Assistant(content string) PromptTemplate {
	return PromptTemplate{Role: "assistant", Content: content}
}

// Prompt registers a prompt with the server.
// The function returns the server instance to allow for method chaining.
// The name parameter is used as the identifier for the prompt.
// The description parameter explains what the prompt does.
// The templates parameter is a list of prompt templates that make up the prompt.
func (s *serverImpl) Prompt(name string, description string, templates ...interface{}) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		s.logger.Error("prompt name cannot be empty")
		return s
	}

	var promptTemplates []PromptTemplate
	for _, template := range templates {
		// Convert to proper template type based on type
		switch t := template.(type) {
		case PromptTemplate:
			// Already a PromptTemplate
			promptTemplates = append(promptTemplates, t)
		case string:
			// String is treated as a user prompt
			promptTemplates = append(promptTemplates, User(t))
		default:
			// Try to convert to JSON string
			if jsonStr, err := json.Marshal(t); err == nil {
				promptTemplates = append(promptTemplates, User(string(jsonStr)))
			} else {
				s.logger.Warn("failed to convert template to prompt", "error", err)
			}
		}
	}

	// Extract variables from templates for argument extraction
	arguments := extractArguments(promptTemplates)

	s.prompts[name] = &Prompt{
		Name:        name,
		Description: description,
		Templates:   promptTemplates,
		Arguments:   arguments,
	}

	// Send notification that prompts list has changed
	s.sendNotification("notifications/prompts/list_changed", nil)

	return s
}

// extractArguments extracts variable names from templates and creates arguments list.
// It uses a regular expression to find all {{variable}} patterns in the templates
// and creates a corresponding list of required arguments.
func extractArguments(templates []PromptTemplate) []PromptArgument {
	variableMap := make(map[string]bool)
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	// Collect all unique variable names
	for _, template := range templates {
		matches := re.FindAllStringSubmatch(template.Content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				varName := strings.TrimSpace(match[1])
				variableMap[varName] = true
			}
		}
	}

	// Convert to PromptArgument slice
	var arguments []PromptArgument
	for varName := range variableMap {
		arguments = append(arguments, PromptArgument{
			Name:        varName,
			Description: fmt.Sprintf("Value for %s", varName),
			Required:    true, // Default to required
		})
	}

	return arguments
}

// ProcessPromptList processes a prompt list request.
// This method handles requests for listing available prompts, supporting
// pagination through an optional cursor parameter.
// The response includes prompt metadata such as name, description, and arguments.
func (s *serverImpl) ProcessPromptList(ctx *Context) (interface{}, error) {
	// Get pagination cursor if provided
	var cursor string
	if ctx.Request.Params != nil {
		var params struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		cursor = params.Cursor
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// For now, we'll use a simple pagination that returns all prompts
	const maxPageSize = 50
	var prompts = make([]map[string]interface{}, 0)
	var nextCursor string

	// Convert prompts to the expected format
	i := 0
	for name, prompt := range s.prompts {
		// If we have a cursor, skip until we find it
		if cursor != "" && name <= cursor {
			continue
		}

		// Add the prompt to the result
		promptInfo := map[string]interface{}{
			"name":        prompt.Name,
			"description": prompt.Description,
		}

		// Include arguments if available
		if len(prompt.Arguments) > 0 {
			promptInfo["arguments"] = prompt.Arguments
		}

		prompts = append(prompts, promptInfo)

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = name
			break
		}
	}

	// Return the list of prompts
	result := map[string]interface{}{
		"prompts": prompts,
	}

	// Only add nextCursor if there are more results
	if nextCursor != "" {
		result["nextCursor"] = nextCursor
	}

	return result, nil
}

// SubstituteVariables replaces all {{variable}} patterns in the content string
// with their corresponding values from the variables map.
// Returns an error if a required variable is missing from the map.
func SubstituteVariables(content string, variables map[string]interface{}) (string, error) {
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	result := content
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		varName := strings.TrimSpace(match[1])
		varValue, exists := variables[varName]

		if !exists {
			return "", NewInvalidParametersError(fmt.Sprintf("missing required variable: %s", varName))
		}

		// Convert the value to string
		var valueStr string
		switch v := varValue.(type) {
		case string:
			valueStr = v
		case nil:
			valueStr = ""
		default:
			// Try to JSON encode complex values
			if jsonBytes, err := json.Marshal(v); err == nil {
				valueStr = string(jsonBytes)
			} else {
				valueStr = fmt.Sprintf("%v", v)
			}
		}

		// Replace the variable in the template
		placeholder := match[0]
		result = strings.Replace(result, placeholder, valueStr, -1)
	}

	return result, nil
}

// ProcessPromptRequest processes a prompt request.
// This method handles requests for rendering a prompt with provided arguments.
// It looks up the named prompt, validates the arguments, substitutes variables,
// and returns the rendered prompt as a formatted response.
func (s *serverImpl) ProcessPromptRequest(ctx *Context) (interface{}, error) {
	// Get the prompt name and arguments from params
	if ctx.Request.Params == nil {
		return nil, errors.New("missing params in prompt request")
	}

	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	promptName := params.Name
	if promptName == "" {
		return nil, errors.New("missing prompt name")
	}

	args := params.Arguments
	if args == nil {
		args = make(map[string]interface{})
	}

	// Find the prompt
	s.mu.RLock()
	prompt, exists := s.prompts[promptName]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("prompt not found: %s", promptName)
	}

	// Validate required arguments
	for _, arg := range prompt.Arguments {
		if arg.Required {
			if _, exists := args[arg.Name]; !exists {
				return nil, NewInvalidParametersError(fmt.Sprintf("missing required argument: %s", arg.Name))
			}
		}
	}

	// Render the prompt templates
	renderedTemplates := make([]map[string]interface{}, 0, len(prompt.Templates))
	for _, template := range prompt.Templates {
		// Substitute variables in the content
		renderedContent, err := SubstituteVariables(template.Content, args)
		if err != nil {
			return nil, err
		}

		// Create a message from the template
		renderedTemplates = append(renderedTemplates, map[string]interface{}{
			"role":    template.Role,
			"content": renderedContent,
		})
	}

	// Return the rendered prompt
	return map[string]interface{}{
		"messages": renderedTemplates,
	}, nil
}
