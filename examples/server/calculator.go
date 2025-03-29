// This file defines the "calculator" tool for the example MCP server.
package main

import (
	"fmt"

	mcp "github.com/localrivet/gomcp"
)

// calculatorToolDefinition defines the structure and schema for the calculator tool
// according to the MCP specification.
var calculatorToolDefinition = mcp.ToolDefinition{
	Name:        "calculator",
	Description: "Performs basic arithmetic operations (add, subtract, multiply, divide).",
	InputSchema: mcp.ToolInputSchema{
		Type: "object", // Input is expected to be a JSON object
		Properties: map[string]mcp.PropertyDetail{
			// Defines the expected parameters, their types, and descriptions
			"operand1":  {Type: "number", Description: "The first number."},
			"operand2":  {Type: "number", Description: "The second number."},
			"operation": {Type: "string", Description: "The operation ('add', 'subtract', 'multiply', 'divide')."},
		},
		Required: []string{"operand1", "operand2", "operation"}, // All parameters are mandatory
	},
	OutputSchema: mcp.ToolOutputSchema{
		Type:        "number", // The tool returns a single number
		Description: "The result of the calculation.",
	},
}

// executeCalculator contains the actual logic for the calculator tool.
// It takes the arguments received from the client (as map[string]interface{})
// and performs validation and the calculation.
// It returns the result (interface{}, typically float64 here) and an optional *mcp.ErrorPayload.
// If a non-nil ErrorPayload is returned, the server handler will send an MCP Error message.
func executeCalculator(args map[string]interface{}) (interface{}, *mcp.ErrorPayload) {
	// --- Argument Extraction and Type Validation ---
	// Safely extract arguments from the map.
	op1Arg, ok1 := args["operand1"]
	op2Arg, ok2 := args["operand2"]
	opStrArg, ok3 := args["operation"]

	// Check if all required arguments are present.
	if !ok1 || !ok2 || !ok3 {
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: "Missing required arguments (operand1, operand2, operation)"}
	}

	// Validate the types of the arguments.
	// MCP schema uses "number", which typically unmarshals to float64 in Go.
	op1, ok1 := op1Arg.(float64)
	op2, ok2 := op2Arg.(float64)
	opStr, ok3 := opStrArg.(string)

	// Check if type assertions were successful.
	if !ok1 || !ok2 || !ok3 {
		// Provide a more specific error message about type mismatches.
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
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: errMsg} // Use MCP code
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
		// Handle division by zero specifically.
		if op2 == 0 {
			// Use a general execution error code
			return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPToolExecutionError, Message: "Division by zero"}
		}
		result = op1 / op2
	default:
		// Handle unknown operation strings.
		return nil, &mcp.ErrorPayload{Code: mcp.ErrorCodeMCPInvalidArgument, Message: fmt.Sprintf("Invalid operation '%s'. Use 'add', 'subtract', 'multiply', or 'divide'.", opStr)} // Use MCP code
	}
	// --- End Calculation ---

	// Return the successful result and a nil error payload.
	return result, nil
}
