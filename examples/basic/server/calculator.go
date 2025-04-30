// This file defines the "calculator" tool for the example MCP server.
package main

import (
	"errors"
	"fmt"

	"github.com/localrivet/gomcp/protocol"
)

// CalculatorArgs defines the arguments for the calculator tool.
// Struct tags are used by the AddTool helper to generate the schema.
type CalculatorArgs struct {
	Operand1  float64 `json:"operand1" description:"The first number." required:"true"`
	Operand2  float64 `json:"operand2" description:"The second number." required:"true"`
	Operation string  `json:"operation" description:"The operation ('add', 'subtract', 'multiply', 'divide')." required:"true"`
}

// executeCalculator contains the actual logic for the calculator tool.
func executeCalculator(args CalculatorArgs) (protocol.Content, error) {

	// Argument parsing and validation is handled by the AddTool helper based on CalculatorArgs struct tags.

	// --- Perform Calculation ---
	var result float64
	switch args.Operation {
	case "add":
		result = args.Operand1 + args.Operand2
	case "subtract":
		result = args.Operand1 - args.Operand2
	case "multiply":
		result = args.Operand1 * args.Operand2
	case "divide":
		if args.Operand2 == 0 {
			// Return error instead of (content, true)
			return nil, errors.New("division by zero")
		}
		result = args.Operand1 / args.Operand2
	default:
		// Return error instead of (content, true)
		return nil, fmt.Errorf("invalid operation '%s'. Use 'add', 'subtract', 'multiply', or 'divide'", args.Operation)
	}
	// --- End Calculation ---

	// Return single content and nil error on success
	resultContent := protocol.TextContent{Type: "text", Text: fmt.Sprintf("%f", result)}
	return resultContent, nil
}
