package server

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// InvalidParametersError represents an error with invalid parameters
type InvalidParametersError struct {
	Message string
}

func (e *InvalidParametersError) Error() string {
	return e.Message
}

// NewInvalidParametersError creates a new InvalidParametersError
func NewInvalidParametersError(message string) *InvalidParametersError {
	return &InvalidParametersError{Message: message}
}

// ContentType represents the type of content in a prompt message.
type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeImage    ContentType = "image"
	ContentTypeAudio    ContentType = "audio"
	ContentTypeResource ContentType = "resource"
)

// PromptContent represents the content of a prompt message.
type PromptContent struct {
	Type     ContentType            `json:"type"`
	Text     string                 `json:"text,omitempty"`
	Data     string                 `json:"data,omitempty"`
	MimeType string                 `json:"mimeType,omitempty"`
	Resource map[string]interface{} `json:"resource,omitempty"`
}

// PromptTemplate represents a template for a prompt with a role and content.
type PromptTemplate struct {
	Role    string
	Content string
	// For storing template variables like {{var}}
	Variables []string
}

// PromptArgument represents an argument for a prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// Prompt represents a prompt registered with the server.
type Prompt struct {
	Name        string
	Description string
	Templates   []PromptTemplate
	Arguments   []PromptArgument
}

// System creates a system prompt template.
func System(content string) PromptTemplate {
	return PromptTemplate{Role: "system", Content: content}
}

// User creates a user prompt template.
func User(content string) PromptTemplate {
	return PromptTemplate{Role: "user", Content: content}
}

// Assistant creates an assistant prompt template.
func Assistant(content string) PromptTemplate {
	return PromptTemplate{Role: "assistant", Content: content}
}

// Prompt registers a prompt with the server.
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

// SubstituteVariables replaces variables in template content with values from variables map.
// Variables in the template should be in the format {{variable_name}}.
func SubstituteVariables(content string, variables map[string]interface{}) (string, error) {
	if len(variables) == 0 {
		return content, nil
	}

	// Regexp to find template variables {{variable}}
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	// Replace all matches with the corresponding variable values
	result := re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name (remove {{ and }})
		varName := strings.TrimSpace(match[2 : len(match)-2])

		// Look up variable value
		if value, exists := variables[varName]; exists {
			// Convert value to string
			switch v := value.(type) {
			case string:
				return v
			case nil:
				return ""
			default:
				// Try to convert to JSON
				if jsonValue, err := json.Marshal(v); err == nil {
					return string(jsonValue)
				}
				// Fallback to string representation
				return fmt.Sprintf("%v", v)
			}
		}

		// Variable not found, keep the original template marker
		return match
	})

	return result, nil
}

// ProcessPromptRequest processes a prompt request message and returns the result.
func (s *serverImpl) ProcessPromptRequest(ctx *Context) (interface{}, error) {
	// Extract params from the request
	if ctx.Request == nil || ctx.Request.Params == nil {
		return nil, NewInvalidParametersError("invalid prompt request: missing params")
	}

	// Parse params
	var params struct {
		Name      string                 `json:"name"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		return nil, NewInvalidParametersError(fmt.Sprintf("invalid prompt params: %s", err.Error()))
	}

	if params.Name == "" {
		return nil, NewInvalidParametersError("invalid prompt request: missing prompt name")
	}

	// Look up the prompt
	s.mu.RLock()
	prompt, exists := s.prompts[params.Name]
	s.mu.RUnlock()

	if !exists {
		return nil, NewInvalidParametersError(fmt.Sprintf("prompt not found: %s", params.Name))
	}

	// Validate required arguments
	missingArgs := []string{}
	for _, arg := range prompt.Arguments {
		if arg.Required {
			if params.Variables == nil || params.Variables[arg.Name] == nil {
				missingArgs = append(missingArgs, arg.Name)
			}
		}
	}

	if len(missingArgs) > 0 {
		return nil, NewInvalidParametersError(fmt.Sprintf("missing required arguments: %s", strings.Join(missingArgs, ", ")))
	}

	// Process the prompt templates by substituting variables
	var messages []map[string]interface{}
	for _, template := range prompt.Templates {
		// Substitute variables in the content
		content, err := SubstituteVariables(template.Content, params.Variables)
		if err != nil {
			return nil, fmt.Errorf("error substituting variables: %w", err)
		}

		// Create content object according to spec
		contentObj := map[string]interface{}{
			"type": "text",
			"text": content,
		}

		// Add to processed messages
		messages = append(messages, map[string]interface{}{
			"role":    template.Role,
			"content": contentObj,
		})
	}

	// Return the processed prompt in the format specified by the MCP specification
	result := map[string]interface{}{
		"description": prompt.Description,
		"messages":    messages,
	}

	return result, nil
}
