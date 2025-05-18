//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// This is a simple test client for the MCP server
// Run with: go run client.go | go run main.go
func main() {
	// Send resource request to server
	sendRequest("resources/read", map[string]interface{}{
		"uri": "/hello",
	})

	// Send resource request to server with parameters
	sendRequest("resources/read", map[string]interface{}{
		"uri":  "/hello",
		"name": "Test User",
	})

	// Send resource request to text resource
	sendRequest("resources/read", map[string]interface{}{
		"uri": "/text",
	})

	// Send resource request to mixed-content resource
	sendRequest("resources/read", map[string]interface{}{
		"uri": "/mixed-content",
	})
}

// sendRequest sends a JSON-RPC request to the server
func sendRequest(method string, params interface{}) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling request: %v\n", err)
		return
	}

	// Send to stdout
	fmt.Println(string(jsonBytes))
}
