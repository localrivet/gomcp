package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/mcp"
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

// Content type constants define the supported content types for prompts.
const (
	// ContentTypeImage is used for image content, which requires an imageUrl
	ContentTypeImage ContentType = "image"

	// ContentTypeAudio is used for audio content, which requires audio data
	ContentTypeAudio ContentType = "audio"

	// ContentTypeResource is used for referencing resources by URI
	ContentTypeResource ContentType = "resource"
)

// PromptTemplate represents a template for a prompt with a role and content.
// Templates can contain variables in the format {{variable}} which are
// substituted when the prompt is rendered.
// Only "user" and "assistant" roles are supported per MCP specification.
type PromptTemplate struct {
	// Role defines who is speaking in this template ("user" or "assistant")
	Role string

	// Content contains the template text with variables in {{variable}} format
	Content string
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
// The templates parameter contains one or more PromptTemplate instances.
func (s *serverImpl) Prompt(name string, description string, templates ...PromptTemplate) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		s.logger.Error("prompt name cannot be empty")
		return s
	}

	if len(templates) == 0 {
		s.logger.Error("at least one template must be provided")
		return s
	}

	// Templates are already PromptTemplate instances, no conversion needed
	promptTemplates := make([]PromptTemplate, len(templates))
	copy(promptTemplates, templates)

	// Extract variables from templates for argument extraction
	arguments := extractArguments(promptTemplates)

	s.prompts[name] = &Prompt{
		Name:        name,
		Description: description,
		Templates:   promptTemplates,
		Arguments:   arguments,
	}

	// Mark prompts as changed for potential notifications
	s.capabilityCache.MarkPromptsChanged()

	// Release the lock before sending notification to avoid deadlock
	s.mu.Unlock()

	// Send simple notification if client is already initialized
	s.sendCapabilityNotification("prompts")

	// Re-acquire lock to satisfy defer unlock
	s.mu.Lock()

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
	// Initialize with empty slice to ensure JSON marshals to [] instead of null
	arguments := make([]PromptArgument, 0)
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
	var prompts = make([]PromptInfo, 0)
	var nextCursor string

	// Convert prompts to the expected format
	i := 0
	for name, prompt := range s.prompts {
		// If we have a cursor, skip until we find it
		if cursor != "" && name <= cursor {
			continue
		}

		// Add the prompt to the result
		promptInfo := PromptInfo{
			Name:        prompt.Name,
			Description: prompt.Description,
			Arguments:   prompt.Arguments, // Always include arguments field, even if empty
		}

		prompts = append(prompts, promptInfo)

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = name
			break
		}
	}

	// Return the list of prompts using structured response
	return NewPromptListResponse(prompts, nextCursor), nil
}

// SubstituteVariables replaces all {{variable}} patterns in the content string
// with their corresponding values from the variables map.
// If a variable is missing, the placeholder is left unchanged.
func SubstituteVariables(content string, variables map[string]interface{}) (string, error) {
	if variables == nil {
		return content, nil
	}

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
			// Leave the placeholder unchanged if variable is missing
			continue
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
		return nil, NewInvalidParametersError(fmt.Sprintf("prompt not found: %s", promptName))
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
	renderedTemplates := make([]PromptMessage, 0, len(prompt.Templates))
	for _, template := range prompt.Templates {
		// Substitute variables in the content
		renderedContent, err := SubstituteVariables(template.Content, args)
		if err != nil {
			return nil, err
		}

		// Create a message from the template with proper content format
		renderedTemplates = append(renderedTemplates, PromptMessage{
			Role: template.Role,
			Content: PromptContent{
				Type: ContentTypeText,
				Text: renderedContent,
			},
		})
	}

	// Emit prompt executed event
	type PromptExecutedEvent struct {
		Operation  string         `json:"operation"`
		PromptName string         `json:"promptName"`
		Arguments  map[string]any `json:"arguments"`
		ExecutedAt time.Time      `json:"executedAt"`
		Success    bool           `json:"success"`
		Templates  int            `json:"templateCount"`
		Metadata   map[string]any `json:"metadata,omitempty"`
	}

	go func() {
		events.Publish[PromptExecutedEvent](s.events, events.TopicPromptExecuted, PromptExecutedEvent{
			Operation:  "prompts/get",
			PromptName: promptName,
			Arguments:  args,
			ExecutedAt: time.Now(),
			Success:    true,
			Templates:  len(renderedTemplates),
		})
	}()

	// Return the rendered prompt with description using structured response
	return NewPromptGetResponse(prompt.Description, renderedTemplates), nil
}

// SendPromptsListChangedNotification sends a notification to inform clients that the prompt list has changed.
// This is called when prompts are added, removed, or updated, allowing clients to refresh their available prompts.
func (s *serverImpl) SendPromptsListChangedNotification() error {
	// Create the notification using structured type
	notification := mcp.NewNotification("notifications/prompts/list_changed", nil)

	// Marshal the notification to JSON
	notificationBytes, err := notification.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Check if the server is initialized (minimize lock scope)
	s.mu.RLock()
	initialized := s.initialized
	transport := s.transport
	s.mu.RUnlock()

	// If the server is not initialized, queue the notification for later
	if !initialized {
		s.capabilityCache.QueueNotification(notificationBytes)
		s.logger.Debug("queued prompts/list_changed notification for after initialization")
		return nil
	}

	// Send the notification through the configured transport (no mutex needed for this)
	if transport != nil {
		if err := transport.Send(notificationBytes); err != nil {
			s.logger.Error("failed to send notification", "error", err)
			return fmt.Errorf("failed to send notification: %w", err)
		}
	} else {
		s.logger.Warn("no transport configured, skipping notification")
	}

	s.logger.Debug("sent prompts/list_changed notification")
	return nil
}
