package v20241105

import (
	"fmt"
	"strings"
)

// PromptDefinition represents an MCP prompt definition
type PromptDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Template    []PromptElement `json:"template"`
	Variables   []PromptVar     `json:"variables,omitempty"`
	Metadata    PromptMetadata  `json:"metadata,omitempty"`
}

// PromptElement represents an element in a prompt template
type PromptElement struct {
	Role     string                 `json:"role,omitempty"`
	Content  []ContentElement       `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ContentElement represents a content element in a prompt
type ContentElement struct {
	Type     string                 `json:"type"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PromptVar represents a variable in a prompt template
type PromptVar struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
}

// PromptMetadata represents metadata for a prompt
type PromptMetadata struct {
	Version     string                 `json:"version,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// ValidatePromptDefinition validates a prompt definition
func ValidatePromptDefinition(prompt PromptDefinition) error {
	if prompt.Name == "" {
		return ErrInvalidPromptDefinition("prompt name is required")
	}
	if prompt.Description == "" {
		return ErrInvalidPromptDefinition("prompt description is required")
	}
	if len(prompt.Template) == 0 {
		return ErrInvalidPromptDefinition("prompt template is required")
	}

	// Extract template variables
	templateVars := extractTemplateVars(prompt)
	declaredVars := make(map[string]bool)

	for _, variable := range prompt.Variables {
		declaredVars[variable.Name] = true
	}

	// Check if all template variables are declared
	for varName := range templateVars {
		if !declaredVars[varName] {
			return ErrInvalidPromptDefinition(fmt.Sprintf("template variable '%s' not declared in variables", varName))
		}
	}

	return nil
}

// extractTemplateVars extracts variables from a prompt template
// Variables are enclosed in {{ }} like {{variable_name}}
func extractTemplateVars(prompt PromptDefinition) map[string]bool {
	vars := make(map[string]bool)

	for _, element := range prompt.Template {
		for _, content := range element.Content {
			// Find all variable references in the content
			text := content.Content
			for {
				start := strings.Index(text, "{{")
				if start == -1 {
					break
				}

				end := strings.Index(text[start:], "}}")
				if end == -1 {
					break
				}

				varName := strings.TrimSpace(text[start+2 : start+end])
				vars[varName] = true

				// Continue searching after this variable
				text = text[start+end+2:]
			}
		}
	}

	return vars
}

// RenderPrompt renders a prompt with the given variables
func RenderPrompt(prompt PromptDefinition, variables map[string]interface{}) ([]PromptElement, error) {
	result := make([]PromptElement, len(prompt.Template))

	// Apply default values for variables
	defaultVars := make(map[string]string)
	for _, v := range prompt.Variables {
		if v.Default != "" {
			defaultVars[v.Name] = v.Default
		}
	}

	// Copy the template
	for i, element := range prompt.Template {
		renderedContent := make([]ContentElement, len(element.Content))

		for j, content := range element.Content {
			renderedText := content.Content

			// Replace variables
			for varName, varValue := range variables {
				strValue := fmt.Sprintf("%v", varValue)
				renderedText = strings.ReplaceAll(renderedText, "{{"+varName+"}}", strValue)
			}

			// Replace any remaining variables with defaults
			for varName, defaultValue := range defaultVars {
				renderedText = strings.ReplaceAll(renderedText, "{{"+varName+"}}", defaultValue)
			}

			renderedContent[j] = ContentElement{
				Type:     content.Type,
				Content:  renderedText,
				Metadata: content.Metadata,
			}
		}

		result[i] = PromptElement{
			Role:     element.Role,
			Content:  renderedContent,
			Metadata: element.Metadata,
		}
	}

	return result, nil
}

// ErrInvalidPromptDefinition represents an error for an invalid prompt definition
type ErrInvalidPromptDefinition string

func (e ErrInvalidPromptDefinition) Error() string {
	return string(e)
}
