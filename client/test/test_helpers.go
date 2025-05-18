package test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/mcp"
)

// SetupClientWithMockTransport creates a client with a mock transport for the given version and returns the client and transport
func SetupClientWithMockTransport(t *testing.T, version string) (client.Client, *MockTransport) {
	mockTransport := SetupMockTransport(version)

	// Ensure the mock transport is connected to prevent "connection closed" errors
	EnsureConnected(mockTransport)

	// Create a new client with the mock transport
	c, err := client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Force initialization by calling GetResource, which triggers Connect() internally
	_, err = c.GetResource("/")
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Verify the correct protocol version was negotiated
	if c.Version() != version {
		t.Fatalf("Expected protocol version %s, got %s", version, c.Version())
	}

	// Reset the mock transport's response queue and history for the actual test
	mockTransport.ClearResponses()
	mockTransport.ClearHistory()

	return c, mockTransport
}

// SetupClientWithOptions creates a client with a mock transport and additional client options
func SetupClientWithOptions(t *testing.T, version string, options ...client.Option) (client.Client, *MockTransport) {
	mockTransport := SetupMockTransport(version)

	// Ensure the mock transport is connected to prevent "connection closed" errors
	EnsureConnected(mockTransport)

	// Combine options with required ones
	allOptions := append([]client.Option{
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	}, options...)

	// Create a new client with the mock transport and additional options
	c, err := client.NewClient("test://server", allOptions...)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Force initialization by calling GetResource, which triggers Connect() internally
	_, err = c.GetResource("/")
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Verify the correct protocol version was negotiated
	if c.Version() != version {
		t.Fatalf("Expected protocol version %s, got %s", version, c.Version())
	}

	// Reset the mock transport's response queue and history for the actual test
	mockTransport.ClearResponses()
	mockTransport.ClearHistory()

	return c, mockTransport
}

// CreateToolResponse creates a response for a tool execution with the given output
func CreateToolResponse(output interface{}) []byte {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"output": output,
		},
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateToolResponseWithID creates a response for a tool execution with a specific ID
func CreateToolResponseWithID(id interface{}, output interface{}, metadata map[string]interface{}) []byte {
	result := map[string]interface{}{
		"output": output,
	}

	// Add metadata if provided
	if metadata != nil {
		for k, v := range metadata {
			result[k] = v
		}
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateToolErrorResponse creates an error response for a tool execution
func CreateToolErrorResponse(id interface{}, code int, message string, data interface{}) []byte {
	errorObj := map[string]interface{}{
		"code":    code,
		"message": message,
	}

	if data != nil {
		errorObj["data"] = data
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   errorObj,
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateResourceResponse creates a resource response for the given version and content
func CreateResourceResponse(version string, content interface{}) []byte {
	var result map[string]interface{}

	switch version {
	case "2025-03-26":
		// 2025-03-26 response format
		if contentMap, ok := content.(map[string]interface{}); ok {
			// If content is a map, use it as is for structured data
			result = map[string]interface{}{
				"contents": []interface{}{
					map[string]interface{}{
						"uri":     "/test/resource",
						"text":    "Test Resource",
						"content": []interface{}{contentMap},
					},
				},
			}
		} else {
			// Handle string/text content
			result = map[string]interface{}{
				"contents": []interface{}{
					map[string]interface{}{
						"uri":  "/test/resource",
						"text": "Test Resource",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": content,
							},
						},
					},
				},
			}
		}
	case "2024-11-05", "draft":
		// 2024-11-05 and draft response format
		if contentMap, ok := content.(map[string]interface{}); ok {
			// If content is a map, convert to content array
			result = map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": contentMap,
					},
				},
			}
		} else {
			// Handle string content
			result = map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": content,
					},
				},
			}
		}
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result":  result,
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateResourceResponseWithPath creates a resource response for a specific resource path
func CreateResourceResponseWithPath(version string, path string, content interface{}, id interface{}) []byte {
	var result map[string]interface{}

	switch version {
	case "2025-03-26":
		// 2025-03-26 response format
		if contentMap, ok := content.(map[string]interface{}); ok {
			// If content is a map, use it as is for structured data
			result = map[string]interface{}{
				"contents": []interface{}{
					map[string]interface{}{
						"uri":     path,
						"text":    fmt.Sprintf("Resource at %s", path),
						"content": []interface{}{contentMap},
					},
				},
			}
		} else {
			// Handle string/text content
			result = map[string]interface{}{
				"contents": []interface{}{
					map[string]interface{}{
						"uri":  path,
						"text": fmt.Sprintf("Resource at %s", path),
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": content,
							},
						},
					},
				},
			}
		}
	case "2024-11-05", "draft":
		// 2024-11-05 and draft response format
		if contentMap, ok := content.(map[string]interface{}); ok {
			// If content is a map, convert to content array
			result = map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": contentMap,
					},
				},
			}
		} else {
			// Handle string content
			result = map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": content,
					},
				},
			}
		}
	}

	// Use provided ID or default to 2
	responseID := id
	if responseID == nil {
		responseID = 2
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      responseID,
		"result":  result,
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreatePromptResponse creates a prompt response with the given prompt and rendered text
func CreatePromptResponse(prompt, rendered string) []byte {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"result": map[string]interface{}{
			"prompt":   prompt,
			"rendered": rendered,
		},
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreatePromptResponseWithID creates a prompt response with a specific ID
func CreatePromptResponseWithID(id interface{}, prompt, rendered string, metadata map[string]interface{}) []byte {
	result := map[string]interface{}{
		"prompt":   prompt,
		"rendered": rendered,
	}

	// Add metadata if provided
	if metadata != nil {
		for k, v := range metadata {
			result[k] = v
		}
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateRootsListResponse creates a response for a roots/list request
func CreateRootsListResponse(id interface{}, roots []map[string]interface{}) []byte {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"roots": roots,
		},
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateEmptyResponse creates a simple success response with empty result
func CreateEmptyResponse(id interface{}) []byte {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]interface{}{},
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateNotificationResponse creates a notification (no ID) response
func CreateNotificationResponse(method string, params map[string]interface{}) []byte {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}

	if params != nil {
		notification["params"] = params
	}

	notificationJSON, _ := json.Marshal(notification)
	return notificationJSON
}

// AssertJSONContains checks if the given JSON data contains the expected key with expected value
func AssertJSONContains(t *testing.T, jsonData []byte, expectedKey string, expectedValue interface{}) {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check if top-level JSON contains the key
	if value, ok := data[expectedKey]; ok {
		// Compare the value with the expected value
		if value != expectedValue {
			t.Errorf("Expected %s to be %v, got %v", expectedKey, expectedValue, value)
		}
		return
	}

	// If not at top level, check in nested structures
	// First check in 'params' which is common for requests
	if params, ok := data["params"].(map[string]interface{}); ok {
		if value, ok := params[expectedKey]; ok {
			if value != expectedValue {
				t.Errorf("Expected params.%s to be %v, got %v", expectedKey, expectedValue, value)
			}
			return
		}
	}

	// Then check in 'result' which is common for responses
	if result, ok := data["result"].(map[string]interface{}); ok {
		if value, ok := result[expectedKey]; ok {
			if value != expectedValue {
				t.Errorf("Expected result.%s to be %v, got %v", expectedKey, expectedValue, value)
			}
			return
		}
	}

	t.Errorf("JSON does not contain key '%s'", expectedKey)
}

// AssertJSONPath checks if the given JSON data contains a value at the specified path
func AssertJSONPath(t *testing.T, jsonData []byte, path []string, expectedValue interface{}) {
	var data interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Navigate the path
	current := data
	for i, key := range path {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected map at path %v, got %T", path[:i], current)
		}

		current, ok = currentMap[key]
		if !ok {
			t.Fatalf("Path %v not found in JSON", path[:i+1])
		}
	}

	// Compare the final value
	if !reflect.DeepEqual(current, expectedValue) {
		t.Errorf("Expected %v at path %v, got %v", expectedValue, path, current)
	}
}

// AssertMethodEquals checks if the JSON-RPC method matches the expected value
func AssertMethodEquals(t *testing.T, jsonData []byte, expectedMethod string) {
	AssertJSONContains(t, jsonData, "method", expectedMethod)
}

// AssertResponseSuccess checks if the JSON-RPC response indicates success
func AssertResponseSuccess(t *testing.T, jsonData []byte) {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check for error field
	if errorObj, hasError := data["error"]; hasError {
		t.Errorf("Expected successful response, got error: %v", errorObj)
	}

	// Check for id and result fields
	if _, hasID := data["id"]; !hasID {
		t.Error("Response missing 'id' field")
	}

	if _, hasResult := data["result"]; !hasResult {
		t.Error("Response missing 'result' field")
	}
}

// AssertResponseError checks if the JSON-RPC response indicates an error with the expected code
func AssertResponseError(t *testing.T, jsonData []byte, expectedCode int) {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check for error field
	errorObj, hasError := data["error"]
	if !hasError {
		t.Fatal("Expected error response, got success")
	}

	errorMap, ok := errorObj.(map[string]interface{})
	if !ok {
		t.Fatalf("Error field is not an object: %v", errorObj)
	}

	code, hasCode := errorMap["code"]
	if !hasCode {
		t.Fatal("Error object missing 'code' field")
	}

	codeFloat, ok := code.(float64)
	if !ok {
		t.Fatalf("Error code is not a number: %v", code)
	}

	if int(codeFloat) != expectedCode {
		t.Errorf("Expected error code %d, got %d", expectedCode, int(codeFloat))
	}
}

// AssertRequestParams checks if a request has the expected parameters
func AssertRequestParams(t *testing.T, request []byte, expected map[string]interface{}) {
	var req map[string]interface{}
	err := json.Unmarshal(request, &req)
	if err != nil {
		t.Fatalf("Failed to parse request: %v", err)
	}

	params, ok := req["params"].(map[string]interface{})
	if !ok {
		t.Fatal("Request does not contain params object")
	}

	for key, expectedValue := range expected {
		actualValue, exists := params[key]
		if !exists {
			t.Errorf("Parameter '%s' not found in request", key)
			continue
		}

		if !reflect.DeepEqual(actualValue, expectedValue) {
			t.Errorf("Parameter '%s' expected to be %v, got %v", key, expectedValue, actualValue)
		}
	}
}

// GetJSONRPCID extracts the ID from a JSON-RPC message
func GetJSONRPCID(jsonData []byte) (interface{}, error) {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, err
	}

	id, exists := data["id"]
	if !exists {
		return nil, fmt.Errorf("id field not found in JSON-RPC message")
	}

	return id, nil
}

// GetJSONRPCParams extracts the params from a JSON-RPC message
func GetJSONRPCParams(jsonData []byte) (map[string]interface{}, error) {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, err
	}

	params, ok := data["params"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("params field not found or not an object in JSON-RPC message")
	}

	return params, nil
}

// EnsureConnected sets the Connected property of the MockTransport to true
// to prevent "connection closed" errors in tests
func EnsureConnected(transport *MockTransport) {
	transport.mu.Lock()
	transport.Connected = true
	transport.mu.Unlock()
}
