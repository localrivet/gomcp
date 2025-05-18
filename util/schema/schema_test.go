package schema

import (
	"encoding/json"
	"testing"
)

type TestStruct struct {
	Name            string   `json:"name" required:"true" description:"The name field" minLength:"3" maxLength:"50"`
	Age             int      `json:"age" min:"0" max:"120" description:"Age in years"`
	Email           string   `json:"email" format:"email" description:"Contact email address"`
	Role            string   `json:"role" enum:"admin,user,guest" description:"User role"`
	Score           float64  `json:"score" min:"0" max:"100" description:"User score" default:"50"`
	Tags            []string `json:"tags,omitempty" description:"Optional tags"`
	UnexportedField string   `json:"-"`
}

func TestFromStruct(t *testing.T) {
	schema := FromStruct(TestStruct{})

	// Check type
	if schema.Type != "object" {
		t.Errorf("Expected schema type to be 'object', got '%s'", schema.Type)
	}

	// Check required fields
	requiredFound := false
	for _, req := range schema.Required {
		if req == "name" {
			requiredFound = true
			break
		}
	}
	if !requiredFound {
		t.Error("Expected 'name' to be in required fields list")
	}

	// Check properties
	// Name field
	name, ok := schema.Properties["name"]
	if !ok {
		t.Fatal("Expected 'name' property to exist")
	}
	if name.Type != "string" {
		t.Errorf("Expected 'name' type to be 'string', got '%s'", name.Type)
	}
	if name.Description != "The name field" {
		t.Errorf("Expected correct description for 'name', got '%s'", name.Description)
	}
	if *name.MinLength != 3 {
		t.Errorf("Expected 'name' minLength to be 3, got %d", *name.MinLength)
	}
	if *name.MaxLength != 50 {
		t.Errorf("Expected 'name' maxLength to be 50, got %d", *name.MaxLength)
	}

	// Age field
	age, ok := schema.Properties["age"]
	if !ok {
		t.Fatal("Expected 'age' property to exist")
	}
	if age.Type != "integer" {
		t.Errorf("Expected 'age' type to be 'integer', got '%s'", age.Type)
	}
	if *age.Minimum != 0 {
		t.Errorf("Expected 'age' minimum to be 0, got %f", *age.Minimum)
	}
	if *age.Maximum != 120 {
		t.Errorf("Expected 'age' maximum to be 120, got %f", *age.Maximum)
	}

	// Role field (enum)
	role, ok := schema.Properties["role"]
	if !ok {
		t.Fatal("Expected 'role' property to exist")
	}
	if role.Type != "string" {
		t.Errorf("Expected 'role' type to be 'string', got '%s'", role.Type)
	}
	if len(role.Enum) != 3 {
		t.Errorf("Expected 'role' to have 3 enum values, got %d", len(role.Enum))
	}
	enumValues := make([]string, len(role.Enum))
	for i, v := range role.Enum {
		enumValues[i] = v.(string)
	}
	expectedEnums := []string{"admin", "user", "guest"}
	for _, expected := range expectedEnums {
		found := false
		for _, actual := range enumValues {
			if expected == actual {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected enum value '%s' not found", expected)
		}
	}

	// Email field (format)
	email, ok := schema.Properties["email"]
	if !ok {
		t.Fatal("Expected 'email' property to exist")
	}
	if email.Format != "email" {
		t.Errorf("Expected 'email' format to be 'email', got '%s'", email.Format)
	}

	// Score field (default)
	score, ok := schema.Properties["score"]
	if !ok {
		t.Fatal("Expected 'score' property to exist")
	}
	if score.Default != float64(50) {
		t.Errorf("Expected 'score' default to be 50, got %v", score.Default)
	}

	// Check for unexported field
	if _, ok := schema.Properties["unexportedField"]; ok {
		t.Error("Unexported fields should not be included in schema")
	}
}

func TestValidateStruct(t *testing.T) {
	// Valid struct
	valid := TestStruct{
		Name:  "John Doe",
		Age:   30,
		Email: "john@example.com",
		Role:  "admin",
		Score: 85.5,
	}

	if err := ValidateStruct(valid); err != nil {
		t.Errorf("Expected no validation errors for valid struct, got: %v", err)
	}

	// Invalid struct - required field missing
	invalid1 := TestStruct{
		Age:   30,
		Email: "john@example.com",
		Role:  "admin",
	}

	if err := ValidateStruct(invalid1); err == nil {
		t.Error("Expected validation error for missing required field 'name'")
	}

	// Invalid struct - min value violation
	invalid2 := TestStruct{
		Name:  "John Doe",
		Age:   -5, // Negative age should fail min validation
		Email: "john@example.com",
		Role:  "admin",
	}

	if err := ValidateStruct(invalid2); err == nil {
		t.Error("Expected validation error for age < 0")
	}

	// Invalid struct - enum violation
	invalid3 := TestStruct{
		Name:  "John Doe",
		Age:   30,
		Email: "john@example.com",
		Role:  "manager", // Invalid role
	}

	if err := ValidateStruct(invalid3); err == nil {
		t.Error("Expected validation error for invalid role")
	}

	// Invalid struct - format violation
	invalid4 := TestStruct{
		Name:  "John Doe",
		Age:   30,
		Email: "invalid-email", // Invalid email
		Role:  "admin",
	}

	if err := ValidateStruct(invalid4); err == nil {
		t.Error("Expected validation error for invalid email format")
	}

	// Invalid struct - minLength violation
	invalid5 := TestStruct{
		Name:  "Jo", // Too short
		Age:   30,
		Email: "john@example.com",
		Role:  "admin",
	}

	if err := ValidateStruct(invalid5); err == nil {
		t.Error("Expected validation error for name too short")
	}
}

func TestGenerateSchema(t *testing.T) {
	g := NewGenerator()
	schema, err := g.GenerateSchema(TestStruct{})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check the top-level schema structure
	expected := map[string]interface{}{
		"type": "object",
	}

	for k, v := range expected {
		if schema[k] != v {
			t.Errorf("Expected schema[%s] to be %v, got %v", k, v, schema[k])
		}
	}

	// Verify properties exists
	if _, ok := schema["properties"]; !ok {
		t.Fatal("Expected 'properties' key in schema")
	}

	// Verify required exists
	if _, ok := schema["required"]; !ok {
		t.Fatal("Expected 'required' key in schema")
	}
}

func TestHandleArgs(t *testing.T) {
	// Valid arguments
	validArgs := map[string]interface{}{
		"name":  "John Doe",
		"age":   30,
		"email": "john@example.com",
		"role":  "admin",
		"score": 85.5,
	}

	result, err := HandleArgs[TestStruct](validArgs)
	if err != nil {
		t.Errorf("Unexpected error for valid args: %v", err)
	}

	if result.Name != "John Doe" {
		t.Errorf("Expected Name to be 'John Doe', got '%s'", result.Name)
	}

	// Invalid arguments - missing required field
	invalidArgs := map[string]interface{}{
		"age":   30,
		"email": "john@example.com",
		"role":  "admin",
	}

	_, err = HandleArgs[TestStruct](invalidArgs)
	if err == nil {
		t.Error("Expected validation error for missing required field")
	}
}

func TestValidator(t *testing.T) {
	v := NewValidator()

	// Test Required
	v.Required("name", "")
	if !v.HasErrors() {
		t.Error("Expected error for empty required field")
	}

	// Reset validator
	v = NewValidator()

	// Test Min
	v.Min("age", 5, 10)
	if !v.HasErrors() {
		t.Error("Expected error for value below minimum")
	}

	// Reset validator
	v = NewValidator()

	// Test Max
	v.Max("score", 150, 100)
	if !v.HasErrors() {
		t.Error("Expected error for value above maximum")
	}

	// Reset validator
	v = NewValidator()

	// Test MinLength
	v.MinLength("name", "Jo", 3)
	if !v.HasErrors() {
		t.Error("Expected error for string below minimum length")
	}

	// Reset validator
	v = NewValidator()

	// Test MaxLength
	v.MaxLength("description", "This is a very long description", 10)
	if !v.HasErrors() {
		t.Error("Expected error for string above maximum length")
	}

	// Reset validator
	v = NewValidator()

	// Test Enum
	v.Enum("role", "supervisor", []string{"admin", "user", "guest"})
	if !v.HasErrors() {
		t.Error("Expected error for value not in enum")
	}

	// Reset validator
	v = NewValidator()

	// Test Format
	v.Format("email", "invalid-email", "email")
	if !v.HasErrors() {
		t.Error("Expected error for invalid email format")
	}
}

func TestPropertyDetailJson(t *testing.T) {
	// Test that PropertyDetail correctly serializes to JSON
	prop := PropertyDetail{
		Type:        "string",
		Description: "Test field",
		Format:      "email",
		Enum:        []interface{}{"a", "b", "c"},
		Minimum:     float64Ptr(5),
		Maximum:     float64Ptr(10),
		MinLength:   intPtr(3),
		MaxLength:   intPtr(50),
		Pattern:     "^test",
		Default:     "default",
	}

	bytes, err := json.Marshal(prop)
	if err != nil {
		t.Fatalf("Failed to marshal PropertyDetail: %v", err)
	}

	// Unmarshal to map to check fields
	var m map[string]interface{}
	if err := json.Unmarshal(bytes, &m); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check that all fields are serialized correctly
	if m["type"] != "string" {
		t.Errorf("Expected type to be 'string', got %v", m["type"])
	}

	if m["description"] != "Test field" {
		t.Errorf("Expected description to be 'Test field', got %v", m["description"])
	}

	if m["format"] != "email" {
		t.Errorf("Expected format to be 'email', got %v", m["format"])
	}

	if m["pattern"] != "^test" {
		t.Errorf("Expected pattern to be '^test', got %v", m["pattern"])
	}

	if m["default"] != "default" {
		t.Errorf("Expected default to be 'default', got %v", m["default"])
	}
}

// Helper functions for creating pointers
func float64Ptr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}
