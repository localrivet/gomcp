package test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestToolRegistrationAndList tests registering tools and verifying they're available via tools/list
func TestToolRegistrationAndList(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Define and register tools
	s.Tool("calculator", "Perform calculations", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Simple implementation
		return "calculator result", nil
	})

	s.Tool("greeter", "Greet someone", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Simple implementation
		return "greeter result", nil
	})

	// Add annotations to the calculator tool
	s.WithAnnotations("calculator", map[string]interface{}{
		"category": "math",
		"priority": 1,
	})

	// Create a tools/list request
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list"
	}`)

	// Process the request
	responseBytes, err := server.HandleMessage(s.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process tools/list request: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Check that the response has a result
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result object in response, got: %T", response["result"])
	}

	// Check that the result has a tools array
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("Expected tools array in result, got: %T", result["tools"])
	}

	// Check that we have at least the two tools we registered
	if len(tools) < 2 {
		t.Errorf("Expected at least 2 tools, got %d", len(tools))
	}

	// Find the calculator tool and check its annotations
	var calcFound bool
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := toolMap["name"].(string)
		if !ok || name != "calculator" {
			continue
		}

		calcFound = true

		// Check for annotations
		annotations, ok := toolMap["annotations"].(map[string]interface{})
		if !ok {
			t.Errorf("Calculator tool is missing annotations")
			break
		}

		// Check specific annotation values
		if cat, ok := annotations["category"].(string); !ok || cat != "math" {
			t.Errorf("Expected category annotation to be 'math', got: %v", annotations["category"])
		}
	}

	if !calcFound {
		t.Errorf("Calculator tool not found in tools list")
	}
}

// TestSimpleCalculator tests simple calculator functionality with tools/call
func TestSimpleCalculator(t *testing.T) {
	// Create a server
	s := server.NewServer("test-server")

	// Add a calculator tool
	s.Tool("calculator", "Perform calculations", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Extract arguments from the map
		argsMap, isMap := args.(map[string]interface{})
		if !isMap {
			return nil, errors.New("invalid args type")
		}

		// Extract values
		xVal, xOK := argsMap["x"].(float64)
		yVal, yOK := argsMap["y"].(float64)
		if !xOK || !yOK {
			return nil, errors.New("missing or invalid x/y values")
		}

		// Default operation is add
		operation := "add"
		if op, hasOp := argsMap["operation"].(string); hasOp {
			operation = op
		}

		// Perform the calculation
		switch operation {
		case "add":
			return xVal + yVal, nil
		case "subtract":
			return xVal - yVal, nil
		case "multiply":
			return xVal * yVal, nil
		case "divide":
			if yVal == 0 {
				return nil, errors.New("division by zero")
			}
			return xVal / yVal, nil
		default:
			return nil, errors.New("unsupported operation")
		}
	})

	// Create a tools/call request
	calculatorRequest := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "calculator",
			"arguments": {
				"x": 5,
				"y": 3,
				"operation": "add"
			}
		}
	}`)

	// Process the request
	response, err := server.HandleMessage(s.GetServer(), calculatorRequest)
	if err != nil {
		t.Fatalf("Failed to process calculator request: %v", err)
	}

	// Parse the response
	var responseObj map[string]interface{}
	if err := json.Unmarshal(response, &responseObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check if the response contains a result (no error)
	result, hasResult := responseObj["result"]
	if !hasResult {
		t.Fatalf("Expected result in response, but got: %v", responseObj)
	}

	// Result should be a map or number (depending on server implementation)
	_, isMap := result.(map[string]interface{})
	_, isNumber := result.(float64)
	if !isMap && !isNumber {
		t.Errorf("Expected result to be a map or number, got: %T", result)
	}

	// If it's a map, check for content or value
	if resultMap, ok := result.(map[string]interface{}); ok {
		if content, hasContent := resultMap["content"]; hasContent {
			// Content should be an array
			if contentArray, isArray := content.([]interface{}); isArray {
				if len(contentArray) == 0 {
					t.Errorf("Expected non-empty content array")
				}
			}
		}
	}
}
