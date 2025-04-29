package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// ---------------------------------------------------------------------------
// Payload types
// ---------------------------------------------------------------------------

// ReviewArgs represents the input for code review
type ReviewArgs struct {
	Code     string `json:"code" description:"Code to review"`
	Language string `json:"language" description:"Programming language of the code"`
}

// DocArgs represents the input for documentation generation
type DocArgs struct {
	Code     string `json:"code" description:"Code to document"`
	Language string `json:"language" description:"Programming language of the code"`
	Style    string `json:"style" description:"Documentation style (e.g., 'jsdoc', 'godoc', 'numpy')"`
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func analyzeCode(code, language string) []string {
	var issues []string

	// Simple example checks - in real implementation, use proper parsers
	if strings.Contains(code, "TODO") {
		issues = append(issues, "Contains TODO comments that should be addressed")
	}
	if strings.Contains(code, "console.log") && language == "javascript" {
		issues = append(issues, "Debug console.log statements should be removed in production")
	}
	if strings.Contains(code, "var") && (language == "javascript" || language == "typescript") {
		issues = append(issues, "Consider using 'const' or 'let' instead of 'var'")
	}

	return issues
}

func formatReviewResponse(issues []string) string {
	if len(issues) == 0 {
		return "✅ No issues found in the code review."
	}

	var response strings.Builder
	response.WriteString("🔍 Code Review Results:\n\n")
	for i, issue := range issues {
		response.WriteString(fmt.Sprintf("%d. ⚠️ %s\n", i+1, issue))
	}
	return response.String()
}

func generateDocs(code, language, style string) string {
	// Simple example - in real implementation, use language-specific parsers
	switch style {
	case "jsdoc":
		return "/**\n * " + strings.ReplaceAll(code, "\n", "\n * ") + "\n */"
	case "godoc":
		return "// " + strings.ReplaceAll(code, "\n", "\n// ")
	default:
		return "# " + strings.ReplaceAll(code, "\n", "\n# ")
	}
}

func main() {
	// Create the MCP server instance
	srv := server.NewServer("code-assistant")

	// ---------------------------------------------------------------------
	// Tool: review
	// ---------------------------------------------------------------------
	err := server.AddTool(
		srv,
		"review",
		"Review code for best practices and potential issues",
		func(args ReviewArgs) (protocol.Content, error) {
			// In a real implementation, this would use a more sophisticated analysis
			issues := analyzeCode(args.Code, args.Language)
			return server.Text(formatReviewResponse(issues)), nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to add review tool: %v", err)
	}

	// ---------------------------------------------------------------------
	// Tool: document
	// ---------------------------------------------------------------------
	err = server.AddTool(
		srv,
		"document",
		"Generate documentation for code",
		func(args DocArgs) (protocol.Content, error) {
			// In a real implementation, this would use language-specific parsers
			docs := generateDocs(args.Code, args.Language, args.Style)
			return server.Text(docs), nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to add document tool: %v", err)
	}

	// ---------------------------------------------------------------------
	// Prompt: review-prompt
	// ---------------------------------------------------------------------
	err = server.AddPrompt(
		srv,
		"review-prompt",
		"Code review prompt",
		func(args ReviewArgs) (protocol.PromptMessage, error) {
			return server.Message("assistant",
				"I'll review your code for:\n"+
					"- Best practices\n"+
					"- Potential bugs\n"+
					"- Performance issues\n"+
					"- Security concerns\n"+
					"Please provide your code and specify the language."), nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to add review prompt: %v", err)
	}

	// ---------------------------------------------------------------------
	// Resource: doc-examples
	// ---------------------------------------------------------------------
	err = server.AddResource(
		srv,
		"doc-examples",
		"text/markdown",
		"Documentation Examples",
		"1.0",
		func() (protocol.ResourceContents, error) {
			return protocol.TextResourceContents{
				ContentType: "text/markdown",
				Content: `# Documentation Examples

## JSDoc Style
` + "```js" + `
/**
 * Calculates the sum of two numbers
 * @param {number} a - First number
 * @param {number} b - Second number
 * @returns {number} Sum of a and b
 */
` + "```" + `

## GoDoc Style
` + "```go" + `
// Add returns the sum of two integers.
// It takes two parameters:
//   - a: the first integer
//   - b: the second integer
func Add(a, b int) int
` + "```",
			}, nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to add resource: %v", err)
	}

	// ---------------------------------------------------------------------
	// Start the server (stdio transport)
	// ---------------------------------------------------------------------
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
