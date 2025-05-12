package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestResourceSerialization tests JSON marshalling of the Resource struct.
func TestResourceSerialization(t *testing.T) {
	// Case 1: Resource without optional Size field
	resWithoutSize := Resource{
		URI:  "test://resource/1",
		Name: "Test Resource 1",
	}
	bytesWithoutSize, err := json.Marshal(resWithoutSize)
	if err != nil {
		t.Fatalf("Failed to marshal Resource without size: %v", err)
	}
	// Check if "size" key is absent
	var mapWithoutSize map[string]interface{}
	if err := json.Unmarshal(bytesWithoutSize, &mapWithoutSize); err != nil {
		t.Fatalf("Failed to unmarshal JSON for size check: %v", err)
	}
	if _, exists := mapWithoutSize["size"]; exists {
		t.Errorf("Expected 'size' field to be omitted when nil, but it was present in JSON: %s", string(bytesWithoutSize))
	}

	// Case 2: Resource with optional Size field
	sizeVal := int(512)
	resWithSize := Resource{
		URI:  "test://resource/2",
		Name: "Test Resource 2",
		Size: &sizeVal,
	}
	bytesWithSize, err := json.Marshal(resWithSize)
	if err != nil {
		t.Fatalf("Failed to marshal Resource with size: %v", err)
	}
	// Check if "size" key is present with correct value
	var mapWithSize map[string]interface{}
	if err := json.Unmarshal(bytesWithSize, &mapWithSize); err != nil {
		t.Fatalf("Failed to unmarshal JSON for size check: %v", err)
	}
	if sizeJSON, exists := mapWithSize["size"]; !exists {
		t.Errorf("Expected 'size' field to be present when set, but it was omitted in JSON: %s", string(bytesWithSize))
	} else if sizeFloat, ok := sizeJSON.(float64); !ok || int(sizeFloat) != sizeVal { // JSON numbers are float64
		t.Errorf("Expected 'size' field to be %d, but got %v in JSON: %s", sizeVal, sizeJSON, string(bytesWithSize))
	}
}

// TestResourceDeserialization tests JSON unmarshalling into the Resource struct.
func TestResourceDeserialization(t *testing.T) {
	// Case 1: JSON without size field (Old format)
	jsonWithoutSize := `{"uri":"test://res/old","title":"Old Resource"}`
	var resOld Resource
	if err := json.Unmarshal([]byte(jsonWithoutSize), &resOld); err != nil {
		t.Fatalf("Failed to unmarshal old format JSON: %v", err)
	}
	if resOld.URI != "test://res/old" {
		t.Errorf("URI mismatch for old format: expected %s, got %s", "test://res/old", resOld.URI)
	}
	if resOld.Size != nil {
		t.Errorf("Expected Size to be nil for old format JSON, but got %v", *resOld.Size)
	}

	// Case 2: JSON with size field (New format)
	jsonWithSize := `{"uri":"test://res/new","title":"New Resource","size":1024}`
	var resNew Resource
	if err := json.Unmarshal([]byte(jsonWithSize), &resNew); err != nil {
		t.Fatalf("Failed to unmarshal new format JSON: %v", err)
	}
	if resNew.URI != "test://res/new" {
		t.Errorf("URI mismatch for new format: expected %s, got %s", "test://res/new", resNew.URI)
	}
	if resNew.Size == nil {
		t.Errorf("Expected Size to be non-nil for new format JSON, but it was nil")
	} else if *resNew.Size != 1024 {
		t.Errorf("Expected Size to be 1024 for new format JSON, but got %d", *resNew.Size)
	}
}

// TestProgressParamsSerialization tests JSON marshalling of ProgressParams.
func TestProgressParamsSerialization(t *testing.T) {
	// Case 1: Without optional message
	paramsWithoutMsg := ProgressParams{Token: "abc", Value: 50}
	bytesWithoutMsg, err := json.Marshal(paramsWithoutMsg)
	if err != nil {
		t.Fatalf("Failed to marshal ProgressParams without message: %v", err)
	}
	var mapWithoutMsg map[string]interface{}
	if err := json.Unmarshal(bytesWithoutMsg, &mapWithoutMsg); err != nil {
		t.Fatalf("Failed to unmarshal JSON for message check: %v", err)
	}
	if _, exists := mapWithoutMsg["message"]; exists {
		t.Errorf("Expected 'message' field to be omitted when nil, but it was present: %s", string(bytesWithoutMsg))
	}

	// Case 2: With optional message
	msgVal := "Processing..."
	paramsWithMsg := ProgressParams{Token: "def", Value: 75, Message: &msgVal}
	bytesWithMsg, err := json.Marshal(paramsWithMsg)
	if err != nil {
		t.Fatalf("Failed to marshal ProgressParams with message: %v", err)
	}
	var mapWithMsg map[string]interface{}
	if err := json.Unmarshal(bytesWithMsg, &mapWithMsg); err != nil {
		t.Fatalf("Failed to unmarshal JSON for message check: %v", err)
	}
	if msgJSON, exists := mapWithMsg["message"]; !exists {
		t.Errorf("Expected 'message' field to be present when set, but it was omitted: %s", string(bytesWithMsg))
	} else if msgStr, ok := msgJSON.(string); !ok || msgStr != msgVal {
		t.Errorf("Expected 'message' field to be '%s', but got %v: %s", msgVal, msgJSON, string(bytesWithMsg))
	}
}

// TestProgressParamsDeserialization tests JSON unmarshalling into ProgressParams.
func TestProgressParamsDeserialization(t *testing.T) {
	// Case 1: JSON without message (Old format)
	jsonWithoutMsg := `{"token":"xyz","value":10}`
	var paramsOld ProgressParams
	if err := json.Unmarshal([]byte(jsonWithoutMsg), &paramsOld); err != nil {
		t.Fatalf("Failed to unmarshal old format JSON: %v", err)
	}
	if paramsOld.Token != "xyz" {
		t.Errorf("Token mismatch for old format: expected %s, got %s", "xyz", paramsOld.Token)
	}
	if paramsOld.Message != nil {
		t.Errorf("Expected Message to be nil for old format JSON, but got %v", *paramsOld.Message)
	}

	// Case 2: JSON with message (New format)
	jsonWithMsg := `{"token":"123","value":99,"message":"Almost done"}`
	var paramsNew ProgressParams
	if err := json.Unmarshal([]byte(jsonWithMsg), &paramsNew); err != nil {
		t.Fatalf("Failed to unmarshal new format JSON: %v", err)
	}
	if paramsNew.Token != "123" {
		t.Errorf("Token mismatch for new format: expected %s, got %s", "123", paramsNew.Token)
	}
	if paramsNew.Message == nil {
		t.Errorf("Expected Message to be non-nil for new format JSON, but it was nil")
	} else if *paramsNew.Message != "Almost done" {
		t.Errorf("Expected Message to be 'Almost done', but got '%s'", *paramsNew.Message)
	}
}

// Helper function to compare capabilities (handles nil pointers)
func capabilitiesEqual(a, b ServerCapabilities) bool {
	// Basic comparison (add more fields as needed)
	if !reflect.DeepEqual(a.Logging, b.Logging) {
		return false
	}
	if !reflect.DeepEqual(a.Prompts, b.Prompts) {
		return false
	}
	if !reflect.DeepEqual(a.Resources, b.Resources) {
		return false
	}
	if !reflect.DeepEqual(a.Tools, b.Tools) {
		return false
	}
	// Compare pointers carefully
	if (a.Authorization == nil) != (b.Authorization == nil) {
		return false
	}
	if (a.Completions == nil) != (b.Completions == nil) {
		return false
	}
	// Compare experimental if needed
	if !reflect.DeepEqual(a.Experimental, b.Experimental) {
		return false
	}
	return true
}

// TestServerCapabilitiesSerialization tests JSON marshalling of ServerCapabilities.
func TestServerCapabilitiesSerialization(t *testing.T) {
	// Case 1: Without optional fields (Authorization, Completions)
	capsOld := ServerCapabilities{
		Logging: &struct{}{},
		// Initialize with a composite literal matching the anonymous struct definition
		Resources: &struct {
			Subscribe   bool `json:"subscribe,omitempty"`
			ListChanged bool `json:"listChanged,omitempty"`
		}{Subscribe: true},
	}
	bytesOld, err := json.Marshal(capsOld)
	if err != nil {
		t.Fatalf("Failed to marshal old caps: %v", err)
	}
	var mapOld map[string]interface{}
	json.Unmarshal(bytesOld, &mapOld)
	if _, exists := mapOld["authorization"]; exists {
		t.Errorf("Expected 'authorization' to be omitted, but was present: %s", string(bytesOld))
	}
	if _, exists := mapOld["completions"]; exists {
		t.Errorf("Expected 'completions' to be omitted, but was present: %s", string(bytesOld))
	}
	if _, exists := mapOld["logging"]; !exists {
		t.Errorf("Expected 'logging' to be present, but was omitted: %s", string(bytesOld))
	}

	// Case 2: With optional fields
	capsNew := ServerCapabilities{
		Logging: &struct{}{},
		// Initialize with a composite literal matching the anonymous struct definition
		Resources: &struct {
			Subscribe   bool `json:"subscribe,omitempty"`
			ListChanged bool `json:"listChanged,omitempty"`
		}{Subscribe: true},
		Authorization: &struct{}{},
		Completions:   &struct{}{},
	}
	bytesNew, err := json.Marshal(capsNew)
	if err != nil {
		t.Fatalf("Failed to marshal new caps: %v", err)
	}
	var mapNew map[string]interface{}
	json.Unmarshal(bytesNew, &mapNew)
	if _, exists := mapNew["authorization"]; !exists {
		t.Errorf("Expected 'authorization' to be present, but was omitted: %s", string(bytesNew))
	}
	if _, exists := mapNew["completions"]; !exists {
		t.Errorf("Expected 'completions' to be present, but was omitted: %s", string(bytesNew))
	}
}

// TestServerCapabilitiesDeserialization tests JSON unmarshalling into ServerCapabilities.
func TestServerCapabilitiesDeserialization(t *testing.T) {
	// Case 1: JSON without optional fields
	jsonOld := `{"logging":{},"resources":{"subscribe":true}}`
	var capsOld ServerCapabilities
	if err := json.Unmarshal([]byte(jsonOld), &capsOld); err != nil {
		t.Fatalf("Failed to unmarshal old caps JSON: %v", err)
	}
	if capsOld.Authorization != nil {
		t.Errorf("Expected Authorization to be nil for old JSON, got non-nil")
	}
	if capsOld.Completions != nil {
		t.Errorf("Expected Completions to be nil for old JSON, got non-nil")
	}
	if capsOld.Logging == nil {
		t.Errorf("Expected Logging to be non-nil for old JSON, got nil")
	}

	// Case 2: JSON with optional fields
	jsonNew := `{"logging":{},"resources":{},"authorization":{},"completions":{}}`
	var capsNew ServerCapabilities
	if err := json.Unmarshal([]byte(jsonNew), &capsNew); err != nil {
		t.Fatalf("Failed to unmarshal new caps JSON: %v", err)
	}
	if capsNew.Authorization == nil {
		t.Errorf("Expected Authorization to be non-nil for new JSON, got nil")
	}
	if capsNew.Completions == nil {
		t.Errorf("Expected Completions to be non-nil for new JSON, got nil")
	}
}

// TODO: Add tests for ClientCapabilities serialization/deserialization
// TODO: Add tests for custom unmarshallers (CallToolResult, SamplingMessage) if needed
