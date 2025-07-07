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

	// Tags field (array with items)
	tags, ok := schema.Properties["tags"]
	if !ok {
		t.Fatal("Expected 'tags' property to exist")
	}
	if tags.Type != "array" {
		t.Errorf("Expected 'tags' type to be 'array', got '%s'", tags.Type)
	}
	if tags.Items == nil {
		t.Error("Expected 'tags' to have items property")
	} else if tags.Items.Type != "string" {
		t.Errorf("Expected 'tags' items type to be 'string', got '%s'", tags.Items.Type)
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

func TestToolInputSchemaRequiredFieldNeverNull(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name: "struct with required fields",
			input: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{},
			expected: []string{"name", "age"},
		},
		{
			name: "struct with optional fields only",
			input: struct {
				Format *string `json:"format,omitempty"`
			}{},
			expected: []string{}, // Should be empty array, not null
		},
		{
			name: "struct with mixed fields",
			input: struct {
				Required string  `json:"required"`
				Optional *string `json:"optional,omitempty"`
			}{},
			expected: []string{"required"},
		},
		{
			name:     "empty struct",
			input:    struct{}{},
			expected: []string{}, // Should be empty array, not null
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := FromStruct(tt.input)

			// Verify schema.Required is never nil
			if schema.Required == nil {
				t.Errorf("Required field should never be nil, got nil")
			}

			// Verify the correct required fields
			if len(schema.Required) != len(tt.expected) {
				t.Errorf("Expected %d required fields, got %d", len(tt.expected), len(schema.Required))
			}

			for _, expected := range tt.expected {
				found := false
				for _, actual := range schema.Required {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected required field '%s' not found in %v", expected, schema.Required)
				}
			}

			// Most importantly: JSON marshal and verify it's an array, not null
			jsonBytes, err := json.Marshal(schema)
			if err != nil {
				t.Fatalf("Failed to marshal schema: %v", err)
			}

			var unmarshaled map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal schema: %v", err)
			}

			// Check that required field exists and is an array
			requiredField, exists := unmarshaled["required"]
			if !exists {
				t.Errorf("Required field missing from JSON output")
			}

			// Verify it's an array, not null
			requiredArray, isArray := requiredField.([]interface{})
			if !isArray {
				t.Errorf("Required field should be an array, got %T: %v", requiredField, requiredField)
			}

			// Verify array length matches expected
			if len(requiredArray) != len(tt.expected) {
				t.Errorf("JSON required array has %d elements, expected %d", len(requiredArray), len(tt.expected))
			}

			t.Logf("Schema JSON: %s", string(jsonBytes))
		})
	}
}

func TestGenerateSchemaRequiredFieldNeverNull(t *testing.T) {
	generator := NewGenerator()

	// Test with struct that has no required fields
	schema, err := generator.GenerateSchema(struct {
		Optional *string `json:"optional,omitempty"`
	}{})
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	// Marshal to JSON and verify
	jsonBytes, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	var unmarshaled map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	// Check that required field is an empty array, not null
	requiredField, exists := unmarshaled["required"]
	if !exists {
		t.Errorf("Required field missing from JSON output")
	}

	requiredArray, isArray := requiredField.([]interface{})
	if !isArray {
		t.Errorf("Required field should be an array, got %T: %v", requiredField, requiredField)
	}

	if len(requiredArray) != 0 {
		t.Errorf("Expected empty required array, got %v", requiredArray)
	}

	t.Logf("Generated schema JSON: %s", string(jsonBytes))
}

func TestArraySchemaGeneratesItems(t *testing.T) {
	tests := []struct {
		name           string
		input          interface{}
		expectedFields map[string]struct {
			hasItems    bool
			itemsType   string
			nestedItems bool
		}
	}{
		{
			name: "simple string array",
			input: struct {
				Tags []string `json:"tags" description:"List of tags"`
			}{},
			expectedFields: map[string]struct {
				hasItems    bool
				itemsType   string
				nestedItems bool
			}{
				"tags": {hasItems: true, itemsType: "string", nestedItems: false},
			},
		},
		{
			name: "integer array",
			input: struct {
				Numbers []int `json:"numbers" description:"List of numbers"`
			}{},
			expectedFields: map[string]struct {
				hasItems    bool
				itemsType   string
				nestedItems bool
			}{
				"numbers": {hasItems: true, itemsType: "integer", nestedItems: false},
			},
		},
		{
			name: "array of arrays",
			input: struct {
				Matrix [][]float64 `json:"matrix" description:"2D matrix"`
			}{},
			expectedFields: map[string]struct {
				hasItems    bool
				itemsType   string
				nestedItems bool
			}{
				"matrix": {hasItems: true, itemsType: "array", nestedItems: true},
			},
		},
		{
			name: "mixed types",
			input: struct {
				Name    string   `json:"name"`
				Tags    []string `json:"tags"`
				Numbers []int    `json:"numbers"`
			}{},
			expectedFields: map[string]struct {
				hasItems    bool
				itemsType   string
				nestedItems bool
			}{
				"tags":    {hasItems: true, itemsType: "string", nestedItems: false},
				"numbers": {hasItems: true, itemsType: "integer", nestedItems: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := FromStruct(tt.input)

			// Convert to JSON and back to verify JSON structure
			jsonBytes, err := json.Marshal(schema)
			if err != nil {
				t.Fatalf("Failed to marshal schema: %v", err)
			}

			var schemaMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &schemaMap); err != nil {
				t.Fatalf("Failed to unmarshal schema: %v", err)
			}

			properties, ok := schemaMap["properties"].(map[string]interface{})
			if !ok {
				t.Fatal("Properties not found in schema")
			}

			// Check each expected field
			for fieldName, expected := range tt.expectedFields {
				prop, exists := properties[fieldName].(map[string]interface{})
				if !exists {
					continue // Non-array fields might not be in expectedFields
				}

				// Verify type is array
				propType, _ := prop["type"].(string)
				if propType != "array" {
					continue
				}

				// Check for items property
				items, hasItems := prop["items"].(map[string]interface{})
				if expected.hasItems && !hasItems {
					t.Errorf("Field '%s' is type 'array' but missing 'items' property", fieldName)
					t.Logf("Property content: %+v", prop)
				}

				if hasItems {
					// Check items type
					itemsType, _ := items["type"].(string)
					if itemsType != expected.itemsType {
						t.Errorf("Field '%s' items type: got '%s', expected '%s'", fieldName, itemsType, expected.itemsType)
					}

					// Check for nested items (array of arrays)
					if expected.nestedItems {
						nestedItems, hasNestedItems := items["items"].(map[string]interface{})
						if !hasNestedItems {
							t.Errorf("Field '%s' is array of arrays but missing nested 'items' property", fieldName)
						} else {
							nestedType, _ := nestedItems["type"].(string)
							t.Logf("Nested items type for '%s': %s", fieldName, nestedType)
						}
					}
				}
			}

			t.Logf("Generated schema JSON: %s", string(jsonBytes))
		})
	}
}

func TestArraySchemaValidation(t *testing.T) {
	// Test that the generated schema can be used for validation
	type TestArrayStruct struct {
		Tags    []string `json:"tags" description:"List of tags"`
		Numbers []int    `json:"numbers" description:"List of numbers"`
	}

	schema := FromStruct(TestArrayStruct{})

	// The schema should properly define array types with items
	tagsSchema, ok := schema.Properties["tags"]
	if !ok {
		t.Fatal("tags property not found")
	}

	if tagsSchema.Type != "array" {
		t.Errorf("Expected tags type to be 'array', got '%s'", tagsSchema.Type)
	}

	if tagsSchema.Items == nil {
		t.Error("Expected tags to have items property")
	} else if tagsSchema.Items.Type != "string" {
		t.Errorf("Expected tags items type to be 'string', got '%s'", tagsSchema.Items.Type)
	}

	numbersSchema, ok := schema.Properties["numbers"]
	if !ok {
		t.Fatal("numbers property not found")
	}

	if numbersSchema.Type != "array" {
		t.Errorf("Expected numbers type to be 'array', got '%s'", numbersSchema.Type)
	}

	if numbersSchema.Items == nil {
		t.Error("Expected numbers to have items property")
	} else if numbersSchema.Items.Type != "integer" {
		t.Errorf("Expected numbers items type to be 'integer', got '%s'", numbersSchema.Items.Type)
	}
}
