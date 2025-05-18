package test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client/test/jsonrpc"
)

// VersionAssertions provides version-specific assertion utilities for testing
// protocol compatibility

// AssertVersionSpecificResource verifies that a resource response follows the
// correct structure for the specified protocol version
func AssertVersionSpecificResource(t *testing.T, version string, response []byte) {
	var responseObj map[string]interface{}
	if err := json.Unmarshal(response, &responseObj); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Check for the jsonrpc field
	if responseObj["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc field to be 2.0, got %v", responseObj["jsonrpc"])
	}

	// Check for the result field
	result, ok := responseObj["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result field to be an object, got %T", responseObj["result"])
	}

	// Verify the structure based on version
	switch version {
	case "2025-03-26":
		// Check for contents array
		_, ok := result["contents"]
		if !ok {
			t.Errorf("Expected contents field in 2025-03-26 response, got %v", result)
		}
	case "2024-11-05", "draft":
		// Check for content array
		_, ok := result["content"]
		if !ok {
			t.Errorf("Expected content field in %s response, got %v", version, result)
		}
	}
}

// AssertVersionNegotiation verifies that the client correctly negotiates
// the protocol version with the server
func AssertVersionNegotiation(t *testing.T, clientVersion, serverVersion string, response []byte) {
	t.Helper()

	var resp map[string]interface{}
	if err := json.Unmarshal(response, &resp); err != nil {
		t.Fatalf("Failed to parse initialize response: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", resp["result"])
	}

	// Check for protocolVersion field
	protocolVersion, ok := result["protocolVersion"].(string)
	if !ok {
		t.Errorf("Missing or invalid protocolVersion in response")
	} else if protocolVersion != serverVersion {
		t.Errorf("Expected server protocolVersion %s, got %s", serverVersion, protocolVersion)
	}

	// If client version and server version differ, additional checks may be needed
	if clientVersion != serverVersion {
		t.Logf("Client version %s negotiated with server version %s", clientVersion, serverVersion)
	}
}

// AssertVersionCapabilities verifies that the capabilities in the response
// are appropriate for the protocol version
func AssertVersionCapabilities(t *testing.T, version string, response []byte) {
	t.Helper()

	var resp map[string]interface{}
	if err := json.Unmarshal(response, &resp); err != nil {
		t.Fatalf("Failed to parse initialize response: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", resp["result"])
	}

	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected capabilities to be a map, got %T", result["capabilities"])
	}

	// Check version-specific capabilities
	switch version {
	case "2025-03-26":
		// Check for enhanced resources capability
		_, hasEnhancedResources := capabilities["enhancedResources"]
		if !hasEnhancedResources {
			t.Errorf("Version %s should support enhancedResources capability", version)
		}

	case "draft":
		// Check for experimental features in draft
		_, hasExperimental := capabilities["experimental"]
		if !hasExperimental {
			t.Errorf("Version %s should have experimental capabilities", version)
		}
	}

	// All versions should have roots capability
	roots, hasRoots := capabilities["roots"]
	if !hasRoots {
		t.Errorf("Version %s missing roots capability", version)
	} else {
		rootsMap, ok := roots.(map[string]interface{})
		if !ok {
			t.Errorf("Roots capability should be a map, got %T", roots)
		} else {
			_, hasListChanged := rootsMap["listChanged"]
			if !hasListChanged {
				t.Errorf("Roots capability missing listChanged property")
			}
		}
	}
}

// AssertVersionCompatibility verifies a message is compatible with the specified version
func AssertVersionCompatibility(t *testing.T, version string, jsonData []byte) {
	t.Helper()

	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check JSON-RPC version
	if data["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc version 2.0, got %v", data["jsonrpc"])
	}

	// Check method and perform version-specific validations
	if method, hasMethod := data["method"].(string); hasMethod {
		switch method {
		case "resource/get":
			params, ok := data["params"].(map[string]interface{})
			if !ok {
				t.Errorf("resource/get should have params object")
				return
			}

			// All versions need a path parameter
			_, hasPath := params["path"]
			if !hasPath {
				t.Errorf("resource/get missing path parameter")
			}

			// Version-specific checks
			if version == "2025-03-26" {
				// 2025-03-26 supports additional options
				_, hasOptions := params["options"]
				if !hasOptions {
					t.Logf("2025-03-26 supports options parameter in resource/get")
				}
			}

		case "initialize":
			params, ok := data["params"].(map[string]interface{})
			if !ok {
				t.Errorf("initialize should have params object")
				return
			}

			clientInfo, hasClientInfo := params["clientInfo"]
			if !hasClientInfo {
				t.Errorf("initialize missing clientInfo parameter")
			} else {
				clientInfoMap, ok := clientInfo.(map[string]interface{})
				if !ok {
					t.Errorf("clientInfo should be an object")
				} else {
					_, hasName := clientInfoMap["name"]
					_, hasVersion := clientInfoMap["version"]
					if !hasName || !hasVersion {
						t.Errorf("clientInfo missing name or version fields")
					}
				}
			}

			versions, hasVersions := params["versions"]
			if !hasVersions {
				t.Errorf("initialize missing versions parameter")
			} else {
				// Verify versions is an array
				versionsArr, ok := versions.([]interface{})
				if !ok {
					t.Errorf("versions should be an array, got %T", versions)
				} else if len(versionsArr) == 0 {
					t.Errorf("versions array should not be empty")
				}
			}
		}
	}
}

// AssertVersionSpecificError checks if an error response is properly formatted
// for the specified protocol version and has the expected code
func AssertVersionSpecificError(t *testing.T, version string, response []byte, expectedCode int) {
	t.Helper()

	var resp map[string]interface{}
	if err := json.Unmarshal(response, &resp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	// Check for error object
	errorObj, hasError := resp["error"]
	if !hasError {
		t.Fatalf("Expected error object, got none")
	}

	errorMap, ok := errorObj.(map[string]interface{})
	if !ok {
		t.Fatalf("Error should be an object, got %T", errorObj)
	}

	// Check error code
	code, hasCode := errorMap["code"]
	if !hasCode {
		t.Errorf("Error missing code field")
	} else {
		codeFloat, ok := code.(float64)
		if !ok {
			t.Errorf("Error code should be a number, got %T", code)
		} else if int(codeFloat) != expectedCode {
			t.Errorf("Expected error code %d, got %d", expectedCode, int(codeFloat))
		}
	}

	// Check error message
	message, hasMessage := errorMap["message"]
	if !hasMessage {
		t.Errorf("Error missing message field")
	} else if message == "" {
		t.Errorf("Error message should not be empty")
	}

	// Version-specific error checks
	if version == "2025-03-26" {
		// 2025-03-26 may include additional error data
		_, hasData := errorMap["data"]
		if hasData {
			t.Logf("2025-03-26 error includes additional data")
		}
	}
}

// AssertContentEquivalence compares resource content across different protocol versions
func AssertContentEquivalence(t *testing.T, version1 string, content1 interface{}, version2 string, content2 interface{}) {
	t.Helper()

	// Convert both contents to a standardized format for comparison
	normalizedContent1 := normalizeContent(version1, content1)
	normalizedContent2 := normalizeContent(version2, content2)

	// Compare the normalized content
	if !reflect.DeepEqual(normalizedContent1, normalizedContent2) {
		t.Errorf("Content from %s and %s should be equivalent, but differ:\n%v\n%v",
			version1, version2, normalizedContent1, normalizedContent2)
	}
}

// normalizeContent converts version-specific content formats to a common format for comparison
func normalizeContent(version string, content interface{}) interface{} {
	switch c := content.(type) {
	case map[string]interface{}:
		// Handle different version structures
		if version == "2025-03-26" {
			if contents, ok := c["contents"].([]interface{}); ok && len(contents) > 0 {
				// Extract content from contents array
				if contentItem, ok := contents[0].(map[string]interface{}); ok {
					return contentItem["content"]
				}
			}
		} else if version == "2024-11-05" || version == "draft" {
			if content, ok := c["content"].([]interface{}); ok {
				return content
			}
		}
	}

	// If we can't normalize, return as is
	return content
}

// CreateVersionError creates an error response with version-specific format
func CreateVersionError(version string, id interface{}, code int, message string, data interface{}) []byte {
	// Create error object
	errorObj := map[string]interface{}{
		"code":    code,
		"message": message,
	}

	// Add data if provided
	if data != nil {
		errorObj["data"] = data
	}

	// For 2025-03-26, add additional version-specific error details
	if version == "2025-03-26" {
		errorObj["timestamp"] = time.Now().UTC().Format(time.RFC3339)
		errorObj["version"] = version
	}

	response := &jsonrpc.JSONRPC{
		Version: "2.0",
		ID:      id,
		Error: &jsonrpc.RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateVersionSpecificInitializeResponse creates an initialize response for the given version
// with appropriate capabilities for that version
func CreateVersionSpecificInitializeResponse(version string, id interface{}) []byte {
	capabilities := map[string]interface{}{
		"roots": map[string]interface{}{
			"listChanged": true,
		},
	}

	// Add version-specific capabilities
	switch version {
	case "2025-03-26":
		capabilities["enhancedResources"] = true
		capabilities["multipleRoots"] = true
	case "draft":
		capabilities["experimental"] = map[string]interface{}{
			"featureX": true,
		}
	}

	response := &jsonrpc.JSONRPC{
		Version: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"protocolVersion": version,
			"serverInfo": map[string]interface{}{
				"name":    "Test Server",
				"version": "1.0.0",
			},
			"capabilities": capabilities,
		},
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// AssertCorrectFallback verifies that the client correctly falls back to a supported version
func AssertCorrectFallback(t *testing.T, clientVersions []string, serverVersions []string, expectedVersion string) {
	t.Helper()

	// Check that the expected version is actually in both lists
	foundInClient := false
	for _, v := range clientVersions {
		if v == expectedVersion {
			foundInClient = true
			break
		}
	}

	foundInServer := false
	for _, v := range serverVersions {
		if v == expectedVersion {
			foundInServer = true
			break
		}
	}

	if !foundInClient {
		t.Errorf("Expected version %s not in client supported versions", expectedVersion)
	}

	if !foundInServer {
		t.Errorf("Expected version %s not in server supported versions", expectedVersion)
	}

	// Create a mock initialize response with server versions
	serverVersionsJSON, _ := json.Marshal(serverVersions)
	t.Logf("Server supports versions: %s", string(serverVersionsJSON))

	// The expected version should be the newest supported by both
	if !isExpectedVersionNewest(t, expectedVersion, clientVersions, serverVersions) {
		t.Errorf("Expected version %s is not the newest version supported by both client and server", expectedVersion)
	}
}

// isExpectedVersionNewest checks if the expected version is the newest version supported by both
func isExpectedVersionNewest(t *testing.T, expected string, clientVersions, serverVersions []string) bool {
	t.Helper()

	// Find common versions
	var commonVersions []string
	for _, cv := range clientVersions {
		for _, sv := range serverVersions {
			if cv == sv {
				commonVersions = append(commonVersions, cv)
			}
		}
	}

	if len(commonVersions) == 0 {
		t.Fatalf("No common versions between client and server")
	}

	// Sort versions by precedence (assuming newer versions come later in the version matrices)
	// This is a simplified version comparison that works with our specific version format
	newestVersion := commonVersions[0]
	for _, v := range commonVersions[1:] {
		if compareVersions(v, newestVersion) > 0 {
			newestVersion = v
		}
	}

	return newestVersion == expected
}

// compareVersions compares two version strings
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Special handling for "draft" which is considered the oldest
	if v1 == "draft" && v2 != "draft" {
		return -1
	}
	if v1 != "draft" && v2 == "draft" {
		return 1
	}
	if v1 == "draft" && v2 == "draft" {
		return 0
	}

	// For numeric versions like "2024-11-05", compare them lexicographically
	// This works because the format is consistent (YYYY-MM-DD)
	return strings.Compare(v1, v2)
}

// AssertFeatureSupport verifies that a specific feature is supported in a
// given protocol version based on the capabilities response
func AssertFeatureSupport(t *testing.T, version string, response []byte, featurePath ...string) {
	t.Helper()

	var resp map[string]interface{}
	if err := json.Unmarshal(response, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", resp["result"])
	}

	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected capabilities to be a map, got %T", result["capabilities"])
	}

	// Walk the feature path to find the specific capability
	current := capabilities
	for i, segment := range featurePath[:len(featurePath)-1] {
		next, ok := current[segment].(map[string]interface{})
		if !ok {
			t.Errorf("Feature path segment [%d] %s not found or not an object in version %s",
				i, segment, version)
			return
		}
		current = next
	}

	// Check the final segment
	lastSegment := featurePath[len(featurePath)-1]
	feature, exists := current[lastSegment]
	if !exists {
		t.Errorf("Feature %s not found in version %s", lastSegment, version)
		return
	}

	// Check if feature is enabled (could be a boolean or an object)
	_, disabled := feature.(bool)
	if disabled && feature == false {
		t.Errorf("Feature %s is disabled in version %s", lastSegment, version)
	}
}

// AssertVersionMatrixCompatibility verifies client behavior across a matrix of
// client and server versions
func AssertVersionMatrixCompatibility(t *testing.T, fn func(t *testing.T, clientVersion, serverVersion string)) {
	t.Helper()

	// Define all supported versions
	versions := []string{"draft", "2024-11-05", "2025-03-26"}

	// Run tests for all combinations
	for _, clientVersion := range versions {
		for _, serverVersion := range versions {
			t.Run(clientVersion+"-"+serverVersion, func(t *testing.T) {
				fn(t, clientVersion, serverVersion)
			})
		}
	}
}

// AssertVersionFallbackMatrix tests that a client correctly falls back to
// compatible versions across different combinations
func AssertVersionFallbackMatrix(t *testing.T, fallbackFn func(t *testing.T, clientVersions, serverVersions []string) string) {
	t.Helper()

	testCases := []struct {
		name           string
		clientVersions []string
		serverVersions []string
		expected       string
	}{
		{
			name:           "Full compatibility",
			clientVersions: []string{"draft", "2024-11-05", "2025-03-26"},
			serverVersions: []string{"draft", "2024-11-05", "2025-03-26"},
			expected:       "2025-03-26", // Should pick newest
		},
		{
			name:           "Client missing newest",
			clientVersions: []string{"draft", "2024-11-05"},
			serverVersions: []string{"draft", "2024-11-05", "2025-03-26"},
			expected:       "2024-11-05", // Should pick newest common
		},
		{
			name:           "Server missing newest",
			clientVersions: []string{"draft", "2024-11-05", "2025-03-26"},
			serverVersions: []string{"draft", "2024-11-05"},
			expected:       "2024-11-05", // Should pick newest common
		},
		{
			name:           "Only draft common",
			clientVersions: []string{"draft", "2025-03-26"},
			serverVersions: []string{"draft", "2024-11-05"},
			expected:       "draft", // Only common version
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := fallbackFn(t, tc.clientVersions, tc.serverVersions)
			if actual != tc.expected {
				t.Errorf("Expected version %s for client versions %v and server versions %v, got %s",
					tc.expected, tc.clientVersions, tc.serverVersions, actual)
			}
		})
	}
}

// AssertVersionSpecificOperation tests that an operation behaves correctly
// based on the specific protocol version
func AssertVersionSpecificOperation(t *testing.T, version string,
	operation func() ([]byte, error),
	validator func(t *testing.T, version string, response []byte)) {
	t.Helper()

	response, err := operation()
	if err != nil {
		t.Fatalf("Operation failed for version %s: %v", version, err)
	}

	// Validate the response is appropriate for the version
	validator(t, version, response)
}

// AssertRootOperations verifies that the root operations work correctly for
// the given protocol version
func AssertRootOperations(t *testing.T, version string, transport *MockTransport) {
	t.Helper()

	// Test root list operation
	rootListReq := jsonrpc.NewRootListRequest(1)
	rootListReqJSON, _ := json.Marshal(rootListReq)

	// Create response for the root list operation
	var rootsResult interface{}
	switch version {
	case "2025-03-26":
		// Enhanced roots in newer version
		rootsResult = map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{
					"uri":  "/test/root",
					"name": "Test Root",
					"metadata": map[string]interface{}{
						"description": "Test root directory",
					},
				},
			},
		}
	default:
		// Basic roots in older versions
		rootsResult = map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{
					"uri":  "/test/root",
					"name": "Test Root",
				},
			},
		}
	}

	rootListResp := &jsonrpc.JSONRPC{
		Version: "2.0",
		ID:      1,
		Result:  rootsResult,
	}
	rootListRespJSON, _ := json.Marshal(rootListResp)

	transport.QueueResponse(rootListRespJSON, nil)

	response, err := transport.Send(rootListReqJSON)
	if err != nil {
		t.Fatalf("Root list operation failed for version %s: %v", version, err)
	}

	// Verify response has roots array
	var respObj map[string]interface{}
	if err := json.Unmarshal(response, &respObj); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	result, ok := respObj["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", respObj["result"])
	}

	roots, ok := result["roots"].([]interface{})
	if !ok {
		t.Errorf("Expected roots to be an array, got %T", result["roots"])
	} else if len(roots) == 0 {
		t.Errorf("Roots array should not be empty")
	}

	// Test other root operations if needed for the version
	if version == "2025-03-26" {
		// Test operations specific to 2025-03-26 like root/add
		rootAddReq := jsonrpc.NewRootAddRequest(2, "/new/root", "New Root")
		rootAddReqJSON, _ := json.Marshal(rootAddReq)

		emptyResult := map[string]interface{}{}
		rootAddResp := &jsonrpc.JSONRPC{
			Version: "2.0",
			ID:      2,
			Result:  emptyResult,
		}
		rootAddRespJSON, _ := json.Marshal(rootAddResp)

		transport.QueueResponse(rootAddRespJSON, nil)

		response, err = transport.Send(rootAddReqJSON)
		if err != nil {
			t.Fatalf("Root add operation failed for version %s: %v", version, err)
		}

		// Verify successful response
		var addRespObj map[string]interface{}
		if err := json.Unmarshal(response, &addRespObj); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		_, hasResult := addRespObj["result"]
		if !hasResult {
			t.Errorf("Expected result in response, got none")
		}
	}
}

// ParseResponse parses a JSON-RPC response using the JSONRPC struct
func ParseResponse(t *testing.T, response []byte) *jsonrpc.JSONRPC {
	var responseObj jsonrpc.JSONRPC
	if err := json.Unmarshal(response, &responseObj); err != nil {
		t.Fatalf("Failed to parse response as JSONRPC: %v", err)
	}
	return &responseObj
}

// ParseVersionSpecificResource parses a resource response and checks if it matches
// the expected format for the specified version
func ParseVersionSpecificResource(t *testing.T, version string, response []byte) *jsonrpc.JSONRPC {
	responseObj := ParseResponse(t, response)

	// Verify the structure based on version
	result, ok := responseObj.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", responseObj.Result)
	}

	switch version {
	case "2025-03-26":
		// Check for contents array
		_, ok := result["contents"]
		if !ok {
			t.Errorf("Expected contents field in 2025-03-26 response, got %v", result)
		}
	case "2024-11-05", "draft":
		// Check for content array
		_, ok := result["content"]
		if !ok {
			t.Errorf("Expected content field in %s response, got %v", version, result)
		}
	}

	return responseObj
}

// CreateMockResourceResponse creates a structured resource response for testing
func CreateMockResourceResponse(version string, id interface{}) []byte {
	var responseContent interface{}

	switch version {
	case "2025-03-26":
		responseContent = map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"uri":  "/test/resource",
					"text": "Test Resource",
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Hello, World!",
						},
					},
				},
			},
		}
	default: // draft and 2024-11-05
		responseContent = map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Hello, World!",
				},
			},
		}
	}

	response := &jsonrpc.JSONRPC{
		Version: "2.0",
		ID:      id,
		Result:  responseContent,
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// CreateRootListResponse creates a response for roots/list request
func CreateRootListResponse(version string, id interface{}) []byte {
	// Define roots based on the version
	roots := []interface{}{
		map[string]interface{}{
			"uri":  "/root1",
			"name": "Root 1",
		},
		map[string]interface{}{
			"uri":  "/root2",
			"name": "Root 2",
		},
	}

	// Enhanced metadata for 2025-03-26
	if version == "2025-03-26" {
		roots = []interface{}{
			map[string]interface{}{
				"uri":  "/root1",
				"name": "Root 1",
				"metadata": map[string]interface{}{
					"type":        "directory",
					"description": "First root directory",
				},
			},
			map[string]interface{}{
				"uri":  "/root2",
				"name": "Root 2",
				"metadata": map[string]interface{}{
					"type":        "directory",
					"description": "Second root directory",
				},
			},
		}
	}

	rootListResp := &jsonrpc.JSONRPC{
		Version: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"roots": roots,
		},
	}

	responseJSON, _ := json.Marshal(rootListResp)
	return responseJSON
}

// CreateRootAddResponse creates a response for roots/add request
func CreateRootAddResponse(version string, id interface{}) []byte {
	// Base success response
	result := map[string]interface{}{}

	// Add version-specific fields
	if version == "2025-03-26" {
		result["success"] = true
		result["metadata"] = map[string]interface{}{
			"addedAt": time.Now().UTC().Format(time.RFC3339),
		}
	}

	rootAddResp := &jsonrpc.JSONRPC{
		Version: "2.0",
		ID:      id,
		Result:  result,
	}

	responseJSON, _ := json.Marshal(rootAddResp)
	return responseJSON
}
