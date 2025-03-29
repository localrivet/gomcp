package main

import (
	"fmt"

	mcp "github.com/localrivet/gomcp"
)

// calculatorToolDefinition defines the calculator tool.
var calculatorToolDefinition = mcp.ToolDefinition{
	Name:        "calculator",
	Description: "Performs basic arithmetic operations (add, subtract, multiply, divide).",
	InputSchema: mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]mcp.PropertyDetail{
			"operand1":  {Type: "number", Description: "The first number."},
			"operand2":  {Type: "number", Description: "The second number."},
			"operation": {Type: "string", Description: "The operation ('add', 'subtract', 'multiply', 'divide')."},
		},
		Required: []string{"operand1", "operand2", "operation"},
	},
	OutputSchema: mcp.ToolOutputSchema{
		Type:        "number",
		Description: "The result of the calculation.",
	},
}

// executeCalculator performs the calculation based on the arguments.
// Returns the result (float64) or an error payload.
func executeCalculator(args map[string]interface{}) (interface{}, *mcp.ErrorPayload) {
	// Extract and validate arguments
	op1Arg, ok1 := args["operand1"]
	op2Arg, ok2 := args["operand2"]
	opStrArg, ok3 := args["operation"]

	if !ok1 || !ok2 || !ok3 {
		return nil, &mcp.ErrorPayload{Code: "InvalidArgument", Message: "Missing required arguments (operand1, operand2, operation)"}
	}

	// MCP schema uses "number", which maps to float64 in Go's json unmarshalling
	op1, ok1 := op1Arg.(float64)
	op2, ok2 := op2Arg.(float64)
	opStr, ok3 := opStrArg.(string)

	if !ok1 || !ok2 || !ok3 {
		// More specific error message
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
		return nil, &mcp.ErrorPayload{Code: "InvalidArgument", Message: errMsg}
	}

	// Perform calculation
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
			return nil, &mcp.ErrorPayload{Code: "CalculationError", Message: "Division by zero"}
		}
		result = op1 / op2
	default:
		return nil, &mcp.ErrorPayload{Code: "InvalidArgument", Message: fmt.Sprintf("Invalid operation '%s'. Use 'add', 'subtract', 'multiply', or 'divide'.", opStr)}
	}

	return result, nil
}
