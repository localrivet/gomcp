// This file defines the "calculator" tool for the example MCP server.
package main

import (
	"context"
	"fmt"

	"github.com/localrivet/gomcp/protocol" // Use protocol package
	"github.com/localrivet/gomcp/server"   // Needed for ToolHandlerFunc type matching
)

// calculatorToolDefinition defines the structure and schema for the calculator tool.
var calculatorToolDefinition = protocol.Tool{ // Use protocol.Tool
	Name:        "calculator",
	Description: "Performs basic arithmetic operations (add, subtract, multiply, divide).",
	InputSchema: protocol.ToolInputSchema{ // Use protocol.ToolInputSchema
		Type: "object",
		Properties: map[string]protocol.PropertyDetail{ // Use protocol.PropertyDetail
			"operand1":  {Type: "number", Description: "The first number."},
			"operand2":  {Type: "number", Description: "The second number."},
			"operation": {Type: "string", Description: "The operation ('add', 'subtract', 'multiply', 'divide')."},
		},
		Required: []string{"operand1", "operand2", "operation"},
	},
	// Annotations: protocol.ToolAnnotations{}, // Optional
}

// executeCalculator contains the actual logic for the calculator tool.
// It now matches the ToolHandlerFunc signature.
func executeCalculator(ctx context.Context, progressToken *protocol.ProgressToken, args map[string]interface{}) (content []protocol.Content, isError bool) { // Use protocol types
	// Helper to create error response content
	newErrorContent := func(msg string) []protocol.Content { // Use protocol.Content
		return []protocol.Content{protocol.TextContent{Type: "text", Text: msg}} // Use protocol.TextContent
	}

	// --- Argument Extraction and Type Validation ---
	op1Arg, ok1 := args["operand1"]
	op2Arg, ok2 := args["operand2"]
	opStrArg, ok3 := args["operation"]

	// Check if all required arguments are present.
	if !ok1 || !ok2 || !ok3 {
		return newErrorContent("Missing required arguments (operand1, operand2, operation)"), true // isError = true
	}

	// Validate the types of the arguments.
	op1, ok1 := op1Arg.(float64)
	op2, ok2 := op2Arg.(float64)
	opStr, ok3 := opStrArg.(string)

	// Check if type assertions were successful.
	if !ok1 || !ok2 || !ok3 {
		errMsg := "Invalid argument types:"
		if !ok1 {
			errMsg += " operand1 must be a number;"
		}
		if !ok2 {
			errMsg += " operand2 must be a number;"
		}
		if !ok3 {
			errMsg += " operation must be a string;"
		}
		return newErrorContent(errMsg), true // isError = true
	}
	// --- End Argument Validation ---

	// --- Perform Calculation ---
	var result float64
	switch opStr {
	case "add":
		result = op1 + op2
	case "subtract":
		result = op1 - op2
	case "multiply":
		result = op1 * op2
	case "divide":
		if op2 == 0 {
			return newErrorContent("Division by zero"), true // isError = true
		}
		result = op1 / op2
	default:
		return newErrorContent(fmt.Sprintf("Invalid operation '%s'. Use 'add', 'subtract', 'multiply', or 'divide'.", opStr)), true // isError = true
	}
	// --- End Calculation ---

	resultContent := protocol.TextContent{Type: "text", Text: fmt.Sprintf("%f", result)} // Use protocol.TextContent
	return []protocol.Content{resultContent}, false                                      // Use protocol.Content
}

// Ensure executeCalculator matches the expected handler type
var _ server.ToolHandlerFunc = executeCalculator
