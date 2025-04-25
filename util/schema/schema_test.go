package schema

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/protocol"
	// Assuming validator might be needed if we test validation failures
	// "github.com/localrivet/gomcp/util/validator"
)

// Sample struct for testing HandleArgs
type testArgsStruct struct {
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	Optional *string `json:"optional,omitempty"`
	Nested   struct {
		Value float64 `json:"value"`
	} `json:"nested"`
	// Add a field for validation testing if validator package is used
	// ValidatedField string `json:"validatedField" validate:"required"`
}

func TestHandleArgs(t *testing.T) {
	testCases := []struct {
		name          string
		inputArgs     any
		expectedArgs  *testArgsStruct
		expectError   bool
		expectedError string // Substring to check in error message
	}{
		{
			name: "Valid Full Input",
			inputArgs: map[string]interface{}{
				"name":  "Test Name",
				"count": float64(10), // JSON numbers are float64
				"nested": map[string]interface{}{
					"value": float64(1.23),
				},
				"optional": "present",
			},
			expectedArgs: &testArgsStruct{
				Name:  "Test Name",
				Count: 10,
				Nested: struct {
					Value float64 `json:"value"`
				}{Value: 1.23},
				Optional: func() *string { s := "present"; return &s }(),
			},
			expectError: false,
		},
		{
			name: "Valid Partial Input (Missing Optional)",
			inputArgs: map[string]interface{}{
				"name":  "Partial Test",
				"count": float64(5),
				"nested": map[string]interface{}{
					"value": float64(4.56),
				},
				// "optional" field is missing
			},
			expectedArgs: &testArgsStruct{
				Name:  "Partial Test",
				Count: 5,
				Nested: struct {
					Value float64 `json:"value"`
				}{Value: 4.56},
				Optional: nil, // Expect nil
			},
			expectError: false,
		},
		{
			name: "Input with Extra Field",
			inputArgs: map[string]interface{}{
				"name":       "Extra Field Test",
				"count":      float64(1),
				"nested":     map[string]interface{}{"value": 0.0},
				"extraField": "should be ignored",
			},
			expectedArgs: &testArgsStruct{
				Name:  "Extra Field Test",
				Count: 1,
				Nested: struct {
					Value float64 `json:"value"`
				}{Value: 0.0},
			},
			expectError: false, // Default mapstructure ignores extra fields
		},
		{
			name: "Input with Type Mismatch (String for Int)",
			inputArgs: map[string]interface{}{
				"name":  "Type Mismatch",
				"count": "not-an-int", // String instead of number
				"nested": map[string]interface{}{
					"value": float64(1.0),
				},
			},
			expectError:   true,
			expectedError: "Error parsing arguments", // mapstructure should error
		},
		{
			name: "Invalid Input Type (Slice)",
			inputArgs: []interface{}{
				"not", "a", "map",
			},
			expectError:   true,
			expectedError: "Invalid arguments format",
		},
		{
			name:         "Nil Input",
			inputArgs:    nil,
			expectedArgs: &testArgsStruct{}, // Expect zero-value struct
			expectError:  false,
		},
		// Add validation test case if validator is set up
		// {
		// 	name: "Validation Failure",
		// 	inputArgs: map[string]interface{}{
		// 		"name":  "Validation Test",
		// 		"count": 1,
		// 		"nested": map[string]interface{}{"value": 1.0},
		// 		// Missing "validatedField" which is required
		// 	},
		// 	expectError:   true,
		// 	expectedError: "Invalid arguments: Key: 'testArgsStruct.ValidatedField' Error:Field validation for 'ValidatedField' failed on the 'required' tag",
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resultArgs, errContent, isErr := HandleArgs[testArgsStruct](tc.inputArgs)

			if isErr != tc.expectError {
				t.Errorf("Expected error: %v, but got: %v. Error content: %+v", tc.expectError, isErr, errContent)
			}

			if tc.expectError {
				if !isErr {
					t.Errorf("Expected an error, but got none.")
				} else if len(errContent) > 0 {
					// Check if the actual error message contains the expected substring
					actualError := errContent[0].(protocol.TextContent).Text
					if tc.expectedError != "" && !strings.Contains(actualError, tc.expectedError) {
						t.Errorf("Expected error message to contain '%s', but got '%s'", tc.expectedError, actualError)
					}
				} else {
					t.Errorf("Expected error content, but got nil")
				}
			} else {
				if isErr {
					t.Errorf("Did not expect an error, but got one: %+v", errContent)
				}
				if !reflect.DeepEqual(resultArgs, tc.expectedArgs) {
					t.Errorf("Argument mismatch:\nExpected: %+v\nGot:      %+v", tc.expectedArgs, resultArgs)
				}
			}
		})
	}
}

// TestFromStructRequiredFields verifies that FromStruct correctly identifies required fields.
func TestFromStructRequiredFields(t *testing.T) {
	type requiredTestStruct struct {
		RequiredField1 string `json:"req1"`
		OptionalField1 *int   `json:"opt1,omitempty"`
		RequiredField2 bool   `json:"req2"`
		OptionalField2 *struct {
			Inner string `json:"inner"`
		} `json:"opt2,omitempty"`
		IgnoredField   string `json:"-"`
		NoJsonTag      string
		RequiredField3 float64 `json:"req3"`
	}

	schema := FromStruct(requiredTestStruct{})

	expectedRequired := []string{"req1", "req2", "req3"}
	// Sort both slices for consistent comparison
	sort.Strings(schema.Required)
	sort.Strings(expectedRequired)

	if !reflect.DeepEqual(schema.Required, expectedRequired) {
		t.Errorf("Required fields mismatch.\nExpected: %v\nGot:      %v", expectedRequired, schema.Required)
	}

	// Also check a couple of properties for basic correctness
	if prop, ok := schema.Properties["req1"]; !ok || prop.Type != "string" {
		t.Errorf("Property 'req1' missing or has wrong type: %+v", prop)
	}
	if prop, ok := schema.Properties["opt1"]; !ok || prop.Type != "integer" { // Type should be underlying type (int)
		t.Errorf("Property 'opt1' missing or has wrong type: %+v", prop)
	}
	if _, ok := schema.Properties["IgnoredField"]; ok {
		t.Errorf("Property 'IgnoredField' should not be present")
	}
	if _, ok := schema.Properties["NoJsonTag"]; ok {
		t.Errorf("Property 'NoJsonTag' should not be present")
	}
}

// TestFromStructEnumField verifies that FromStruct correctly parses the enum tag.
func TestFromStructEnumField(t *testing.T) {
	type enumTestStruct struct {
		Color string  `json:"color" description:"A color" enum:"red,green, blue"` // Note space around blue
		Shape *string `json:"shape" enum:"circle,square"`                         // Optional enum
		Mode  int     `json:"mode" enum:"1,2,3"`                                  // Enum on non-string (values treated as strings)
	}

	schema := FromStruct(enumTestStruct{})

	// Check Color enum
	colorProp, ok := schema.Properties["color"]
	if !ok {
		t.Fatalf("Property 'color' not found in schema")
	}
	expectedColorEnum := []interface{}{"red", "green", "blue"} // Expect trimmed strings
	if !reflect.DeepEqual(colorProp.Enum, expectedColorEnum) {
		t.Errorf("Enum mismatch for 'color'.\nExpected: %v\nGot:      %v", expectedColorEnum, colorProp.Enum)
	}

	// Check Shape enum
	shapeProp, ok := schema.Properties["shape"]
	if !ok {
		t.Fatalf("Property 'shape' not found in schema")
	}
	expectedShapeEnum := []interface{}{"circle", "square"}
	if !reflect.DeepEqual(shapeProp.Enum, expectedShapeEnum) {
		t.Errorf("Enum mismatch for 'shape'.\nExpected: %v\nGot:      %v", expectedShapeEnum, shapeProp.Enum)
	}
	if shapeProp.Type != "string" { // Ensure type is still correct (underlying string)
		t.Errorf("Expected type 'string' for 'shape', got '%s'", shapeProp.Type)
	}

	// Check Mode enum (values are strings even though Go type is int)
	modeProp, ok := schema.Properties["mode"]
	if !ok {
		t.Fatalf("Property 'mode' not found in schema")
	}
	expectedModeEnum := []interface{}{"1", "2", "3"}
	if !reflect.DeepEqual(modeProp.Enum, expectedModeEnum) {
		t.Errorf("Enum mismatch for 'mode'.\nExpected: %v\nGot:      %v", expectedModeEnum, modeProp.Enum)
	}
	if modeProp.Type != "integer" { // Ensure type is still correct (integer)
		t.Errorf("Expected type 'integer' for 'mode', got '%s'", modeProp.Type)
	}
}
