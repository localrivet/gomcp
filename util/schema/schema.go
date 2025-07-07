// Package schema provides utilities for generating JSON Schema from Go structs.
package schema

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/localrivet/gomcp/util/conversion"
	"github.com/mitchellh/mapstructure"
)

// PropertyDetail represents a JSON Schema property definition.
type PropertyDetail struct {
	Type        string          `json:"type"`
	Description string          `json:"description,omitempty"`
	Enum        []interface{}   `json:"enum,omitempty"`
	Format      string          `json:"format,omitempty"`
	Minimum     *float64        `json:"minimum,omitempty"`
	Maximum     *float64        `json:"maximum,omitempty"`
	MinLength   *int            `json:"minLength,omitempty"`
	MaxLength   *int            `json:"maxLength,omitempty"`
	Pattern     string          `json:"pattern,omitempty"`
	Default     interface{}     `json:"default,omitempty"`
	Items       *PropertyDetail `json:"items,omitempty"`
}

// ToolInputSchema represents a JSON Schema for tool input.
type ToolInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertyDetail `json:"properties"`
	Required   []string                  `json:"required"`
}

// Generator generates JSON Schema from Go types.
type Generator struct {
	// Configuration options
	IncludeFieldsWithoutTags bool
}

// NewGenerator creates a new schema generator with default configuration.
func NewGenerator() *Generator {
	return &Generator{
		IncludeFieldsWithoutTags: true,
	}
}

// WithIncludeFieldsWithoutTags configures whether to include fields without JSON tags.
func (g *Generator) WithIncludeFieldsWithoutTags(include bool) *Generator {
	g.IncludeFieldsWithoutTags = include
	return g
}

// GenerateSchema generates a JSON Schema from a Go struct or any value.
func (g *Generator) GenerateSchema(v interface{}) (map[string]interface{}, error) {
	schema := FromStruct(v)
	return map[string]interface{}{
		"type":       schema.Type,
		"properties": schema.Properties,
		"required":   schema.Required,
	}, nil
}

// goTypeToJSONType maps Go kinds to JSON Schema types.
func goTypeToJSONType(kind reflect.Kind) string {
	switch kind {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}

// parseNumericTag parses a numeric tag value into a float64 pointer
func parseNumericTag(tagValue string) *float64 {
	if tagValue == "" {
		return nil
	}
	val, err := strconv.ParseFloat(tagValue, 64)
	if err != nil {
		return nil
	}
	return &val
}

// parseIntTag parses an integer tag value into an int pointer
func parseIntTag(tagValue string) *int {
	if tagValue == "" {
		return nil
	}
	val, err := strconv.Atoi(tagValue)
	if err != nil {
		return nil
	}
	return &val
}

// FromStruct generates a ToolInputSchema from struct tags.
// It examines the struct fields and their tags to create a schema that describes
// the expected input format for an MCP tool.
func FromStruct(v interface{}) ToolInputSchema {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	props := map[string]PropertyDetail{}
	requiredFields := []string{}
	trackFields := make(map[string]bool)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		descTag := field.Tag.Get("description")
		jsonTag := field.Tag.Get("json")
		var name string

		if jsonTag == "-" {
			// Skip fields explicitly marked as ignored
			continue
		} else if jsonTag != "" {
			// Use JSON tag if present
			name = strings.Split(jsonTag, ",")[0]

			// Determine if field is required (convention: non-pointer types are required)
			// Only include fields with JSON tags in required fields list
			isPtr := field.Type.Kind() == reflect.Ptr
			if !isPtr && !trackFields[name] {
				requiredFields = append(requiredFields, name)
				trackFields[name] = true
			}
		} else {
			// For exported fields without JSON tags, use lowercase field name for schema
			name = strings.ToLower(field.Name)
		}

		// Check for required tag
		if field.Tag.Get("required") == "true" && !trackFields[name] {
			requiredFields = append(requiredFields, name)
			trackFields[name] = true
		}

		// Determine the schema type
		fieldType := field.Type
		if field.Type.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		schemaType := goTypeToJSONType(fieldType.Kind())

		// Create property definition
		propDetail := PropertyDetail{
			Type:        schemaType,
			Description: descTag,
		}

		// Handle array/slice types - generate items schema
		if fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array {
			elemType := fieldType.Elem()
			itemsDetail := &PropertyDetail{
				Type: goTypeToJSONType(elemType.Kind()),
			}

			// Handle nested arrays
			if elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Array {
				nestedElemType := elemType.Elem()
				itemsDetail.Items = &PropertyDetail{
					Type: goTypeToJSONType(nestedElemType.Kind()),
				}
			}

			propDetail.Items = itemsDetail
		}

		// Process enum tag
		enumTag := field.Tag.Get("enum")
		if enumTag != "" {
			enumValuesStr := strings.Split(enumTag, ",")
			enumValues := make([]interface{}, len(enumValuesStr))
			for i, v := range enumValuesStr {
				// Trim whitespace and store as interface{}
				enumValues[i] = strings.TrimSpace(v)
			}
			propDetail.Enum = enumValues
		}

		// Process format tag for string
		formatTag := field.Tag.Get("format")
		if formatTag != "" {
			propDetail.Format = formatTag
		}

		// Process pattern tag for string
		patternTag := field.Tag.Get("pattern")
		if patternTag != "" {
			propDetail.Pattern = patternTag
		}

		// Process min/max for numeric types
		if schemaType == "integer" || schemaType == "number" {
			minTag := field.Tag.Get("min")
			if minTag != "" {
				propDetail.Minimum = parseNumericTag(minTag)
			}

			maxTag := field.Tag.Get("max")
			if maxTag != "" {
				propDetail.Maximum = parseNumericTag(maxTag)
			}
		}

		// Process minLength/maxLength for string
		if schemaType == "string" {
			minLengthTag := field.Tag.Get("minLength")
			if minLengthTag != "" {
				propDetail.MinLength = parseIntTag(minLengthTag)
			}

			maxLengthTag := field.Tag.Get("maxLength")
			if maxLengthTag != "" {
				propDetail.MaxLength = parseIntTag(maxLengthTag)
			}
		}

		// Process default value
		defaultTag := field.Tag.Get("default")
		if defaultTag != "" {
			// Convert default to the appropriate type based on schema type
			switch schemaType {
			case "integer":
				if val, err := strconv.Atoi(defaultTag); err == nil {
					propDetail.Default = val
				}
			case "number":
				if val, err := strconv.ParseFloat(defaultTag, 64); err == nil {
					propDetail.Default = val
				}
			case "boolean":
				if val, err := strconv.ParseBool(defaultTag); err == nil {
					propDetail.Default = val
				}
			default: // string or other types
				propDetail.Default = defaultTag
			}
		}

		// Add to properties map
		props[name] = propDetail
	}

	schema := ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   requiredFields,
	}

	return schema
}

// Validator provides validation for struct fields.
type Validator struct {
	errors []string
}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{
		errors: []string{},
	}
}

// Required validates that a field is not nil or empty.
func (v *Validator) Required(fieldName string, value interface{}) *Validator {
	if value == nil {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' is required but was not provided", fieldName))
		return v
	}

	// Check for empty string
	if strVal, ok := value.(string); ok && strVal == "" {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' is required but was empty", fieldName))
	}

	return v
}

// Min validates a numeric value has a minimum value.
func (v *Validator) Min(fieldName string, value interface{}, min float64) *Validator {
	var numValue float64
	switch val := value.(type) {
	case int:
		numValue = float64(val)
	case int8:
		numValue = float64(val)
	case int16:
		numValue = float64(val)
	case int32:
		numValue = float64(val)
	case int64:
		numValue = float64(val)
	case uint:
		numValue = float64(val)
	case uint8:
		numValue = float64(val)
	case uint16:
		numValue = float64(val)
	case uint32:
		numValue = float64(val)
	case uint64:
		numValue = float64(val)
	case float32:
		numValue = float64(val)
	case float64:
		numValue = val
	default:
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be a number for min validation", fieldName))
		return v
	}

	if numValue < min {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be at least %v", fieldName, min))
	}

	return v
}

// Max validates a numeric value has a maximum value.
func (v *Validator) Max(fieldName string, value interface{}, max float64) *Validator {
	var numValue float64
	switch val := value.(type) {
	case int:
		numValue = float64(val)
	case int8:
		numValue = float64(val)
	case int16:
		numValue = float64(val)
	case int32:
		numValue = float64(val)
	case int64:
		numValue = float64(val)
	case uint:
		numValue = float64(val)
	case uint8:
		numValue = float64(val)
	case uint16:
		numValue = float64(val)
	case uint32:
		numValue = float64(val)
	case uint64:
		numValue = float64(val)
	case float32:
		numValue = float64(val)
	case float64:
		numValue = val
	default:
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be a number for max validation", fieldName))
		return v
	}

	if numValue > max {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be at most %v", fieldName, max))
	}

	return v
}

// MinLength validates a string has a minimum length.
func (v *Validator) MinLength(fieldName string, value string, minLength int) *Validator {
	if len(value) < minLength {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be at least %d characters long", fieldName, minLength))
	}
	return v
}

// MaxLength validates a string has a maximum length.
func (v *Validator) MaxLength(fieldName string, value string, maxLength int) *Validator {
	if len(value) > maxLength {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be at most %d characters long", fieldName, maxLength))
	}
	return v
}

// Enum validates a value is in a set of allowed values.
func (v *Validator) Enum(fieldName string, value string, allowedValues []string) *Validator {
	found := false
	for _, allowed := range allowedValues {
		if value == allowed {
			found = true
			break
		}
	}

	if !found {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be one of: %s", fieldName, strings.Join(allowedValues, ", ")))
	}
	return v
}

// Format validates a string matches a format.
func (v *Validator) Format(fieldName string, value string, format string) *Validator {
	switch format {
	case "email":
		if !strings.Contains(value, "@") {
			v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be a valid email address", fieldName))
		}
	case "uri":
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be a valid URI", fieldName))
		}
	default:
		v.errors = append(v.errors, fmt.Sprintf("Unsupported format '%s' for field '%s'", format, fieldName))
	}
	return v
}

// Error returns validation errors or nil if none.
func (v *Validator) Error() error {
	if len(v.errors) == 0 {
		return nil
	}
	return fmt.Errorf("validation failed: %s", strings.Join(v.errors, "; "))
}

// HasErrors returns true if there are validation errors.
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Errors returns all validation error messages.
func (v *Validator) Errors() []string {
	return v.errors
}

// ValidateStruct validates a struct against its schema definition tags.
func ValidateStruct(data interface{}) error {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("input must be a struct or pointer to struct")
	}

	v := NewValidator()
	t := val.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := val.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			fieldName = strings.Split(jsonTag, ",")[0]
		}

		// Required validation
		if field.Tag.Get("required") == "true" {
			if isZeroValue(value) {
				v.errors = append(v.errors, fmt.Sprintf("Field '%s' is required but was not provided", fieldName))
				continue
			}
		}

		// Skip validation for nil pointers or zero values
		if isZeroValue(value) {
			continue
		}

		// Get actual value for pointer types
		fieldValue := value
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			fieldValue = value.Elem()
		}

		// Numeric validations for int or float types
		if fieldValue.Kind() >= reflect.Int && fieldValue.Kind() <= reflect.Float64 {
			// Min validation
			if minStr := field.Tag.Get("min"); minStr != "" {
				if min, err := strconv.ParseFloat(minStr, 64); err == nil {
					numVal := getNumericValue(fieldValue)
					if numVal < min {
						v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be at least %v", fieldName, min))
					}
				}
			}

			// Max validation
			if maxStr := field.Tag.Get("max"); maxStr != "" {
				if max, err := strconv.ParseFloat(maxStr, 64); err == nil {
					numVal := getNumericValue(fieldValue)
					if numVal > max {
						v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be at most %v", fieldName, max))
					}
				}
			}
		}

		// String validations
		if fieldValue.Kind() == reflect.String {
			strVal := fieldValue.String()

			// MinLength validation
			if minLenStr := field.Tag.Get("minLength"); minLenStr != "" {
				if minLen, err := strconv.Atoi(minLenStr); err == nil {
					if len(strVal) < minLen {
						v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be at least %d characters long", fieldName, minLen))
					}
				}
			}

			// MaxLength validation
			if maxLenStr := field.Tag.Get("maxLength"); maxLenStr != "" {
				if maxLen, err := strconv.Atoi(maxLenStr); err == nil {
					if len(strVal) > maxLen {
						v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be at most %d characters long", fieldName, maxLen))
					}
				}
			}

			// Enum validation
			if enumStr := field.Tag.Get("enum"); enumStr != "" {
				allowedValues := strings.Split(enumStr, ",")
				for i, val := range allowedValues {
					allowedValues[i] = strings.TrimSpace(val)
				}

				found := false
				for _, val := range allowedValues {
					if strVal == val {
						found = true
						break
					}
				}

				if !found {
					v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be one of: %s", fieldName, strings.Join(allowedValues, ", ")))
				}
			}

			// Format validation
			if format := field.Tag.Get("format"); format != "" {
				v.Format(fieldName, strVal, format)
			}
		}
	}

	return v.Error()
}

// Helper function to check if a value is the zero value
func isZeroValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == complex(0, 0)
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !isZeroValue(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	case reflect.String:
		return v.Len() == 0
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !isZeroValue(v.Field(i)) {
				return false
			}
		}
		return true
	case reflect.UnsafePointer:
		return v.IsNil()
	}

	return false
}

// Helper function to get numeric value as float64
func getNumericValue(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	default:
		return 0
	}
}

// HandleArgs is a helper function that handles parsing and validating
// tool arguments into a strongly-typed struct.
func HandleArgs[T any](arguments any) (*T, error) {
	var args T

	// Ensure arguments is a map[string]interface{}
	argsMap, ok := arguments.(map[string]interface{})
	if !ok {
		if arguments == nil {
			argsMap = make(map[string]interface{})
		} else {
			return nil, fmt.Errorf("invalid arguments format: expected an object/map, got %T", arguments)
		}
	}

	// Use mapstructure to decode the map into the struct
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &args,
		TagName:  "json",
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("internal error creating argument decoder: %w", err)
	}

	if err := decoder.Decode(argsMap); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %w", err)
	}

	// Validate the struct against its schema definition tags
	if err := ValidateStruct(args); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &args, nil
}

// MapValidator validates a map's values against a property schema
func (v *Validator) Map(fieldName string, value map[string]interface{}, propsSchema map[string]interface{}) *Validator {
	// Validate against additionalProperties schema if available
	additionalPropsSchema, _ := propsSchema["additionalProperties"].(map[string]interface{})
	if additionalPropsSchema == nil {
		return v // Nothing to validate
	}

	// Expected type for values
	expectedType, _ := additionalPropsSchema["type"].(string)

	// Validate each map entry
	for key, val := range value {
		// Skip special keys like "_type"
		if strings.HasPrefix(key, "_") {
			continue
		}

		// Validate entry type if specified
		if expectedType != "" && !ValidateType(val, expectedType) {
			v.errors = append(v.errors, fmt.Sprintf("Entry with key '%s' in map '%s' must be of type '%s'",
				key, fieldName, expectedType))
		}

		// Recursive validation for nested objects
		if expectedType == "object" && additionalPropsSchema["properties"] != nil {
			if objVal, ok := val.(map[string]interface{}); ok {
				// Create nested validator
				nestedValidator := NewValidator()
				// Validate the object value against its schema
				if nestedProps, ok := additionalPropsSchema["properties"].(map[string]interface{}); ok {
					for propName, propSchema := range nestedProps {
						if propMap, ok := propSchema.(map[string]interface{}); ok {
							// Check if property exists
							propVal, exists := objVal[propName]
							if !exists {
								// Check if required
								if requiredProps, ok := additionalPropsSchema["required"].([]string); ok {
									for _, req := range requiredProps {
										if req == propName {
											nestedValidator.Required(propName, nil)
										}
									}
								}
								continue
							}
							// Validate property value
							ValidateValueAgainstSchema(nestedValidator, propName, propVal, propMap)
						}
					}
				}

				// Add nested validation errors to parent validator
				if nestedValidator.HasErrors() {
					for _, err := range nestedValidator.Errors() {
						v.errors = append(v.errors, fmt.Sprintf("In map entry '%s' of field '%s': %s",
							key, fieldName, err))
					}
				}
			}
		}

		// Validate basic constraints for primitive types
		ValidateConstraints(v, fmt.Sprintf("%s[%s]", fieldName, key), val, additionalPropsSchema)
	}

	return v
}

// ArrayValidator validates array elements against a schema
func (v *Validator) Array(fieldName string, array []interface{}, arraySchema map[string]interface{}) *Validator {
	// Validate array length
	if minItems, ok := arraySchema["minItems"].(float64); ok && float64(len(array)) < minItems {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must have at least %.0f items", fieldName, minItems))
	}

	if maxItems, ok := arraySchema["maxItems"].(float64); ok && float64(len(array)) > maxItems {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must have at most %.0f items", fieldName, maxItems))
	}

	// Validate items against items schema
	itemsSchema, ok := arraySchema["items"].(map[string]interface{})
	if !ok || len(array) == 0 {
		return v // Nothing to validate
	}

	// Expected type for items
	expectedType, _ := itemsSchema["type"].(string)

	// Validate each item
	for i, item := range array {
		// Validate item type if specified
		if expectedType != "" && !ValidateType(item, expectedType) {
			v.errors = append(v.errors, fmt.Sprintf("Item at index %d in array '%s' must be of type '%s'",
				i, fieldName, expectedType))
		}

		// Recursive validation for nested objects
		if expectedType == "object" && itemsSchema["properties"] != nil {
			if objItem, ok := item.(map[string]interface{}); ok {
				// Create nested validator
				nestedValidator := NewValidator()
				// Validate the object against its schema
				if nestedProps, ok := itemsSchema["properties"].(map[string]interface{}); ok {
					for propName, propSchema := range nestedProps {
						if propMap, ok := propSchema.(map[string]interface{}); ok {
							// Check if property exists
							propVal, exists := objItem[propName]
							if !exists {
								// Check if required
								if requiredProps, ok := itemsSchema["required"].([]string); ok {
									for _, req := range requiredProps {
										if req == propName {
											nestedValidator.Required(propName, nil)
										}
									}
								}
								continue
							}
							// Validate property value
							ValidateValueAgainstSchema(nestedValidator, propName, propVal, propMap)
						}
					}
				}

				// Add nested validation errors to parent validator
				if nestedValidator.HasErrors() {
					for _, err := range nestedValidator.Errors() {
						v.errors = append(v.errors, fmt.Sprintf("In item %d of array '%s': %s",
							i, fieldName, err))
					}
				}
			}
		}

		// Validate basic constraints for primitive types
		ValidateConstraints(v, fmt.Sprintf("%s[%d]", fieldName, i), item, itemsSchema)
	}

	return v
}

// ValidateType checks if a value matches the expected JSON Schema type.
// This is a non-method version of validateType for shared usage.
func ValidateType(value interface{}, expectedType string) bool {
	if value == nil {
		return expectedType == "null"
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	case "integer":
		switch value := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		case float32:
			return float32(int32(value)) == value
		case float64:
			return float64(int64(value)) == value
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		// Check for slice or array type
		switch value.(type) {
		case []interface{}:
			return true
		case []string, []int, []float64, []bool:
			// Other array types are also valid
			return true
		default:
			return false
		}
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return true // Unknown type, assume valid
	}
}

// ValidateValueAgainstSchema validates a single value against its schema
func ValidateValueAgainstSchema(v *Validator, fieldName string, value interface{}, schema map[string]interface{}) {
	// Validate type
	if typeName, ok := schema["type"].(string); ok && !ValidateType(value, typeName) {
		v.errors = append(v.errors, fmt.Sprintf("Field '%s' must be of type '%s'", fieldName, typeName))
		return
	}

	// Validate constraints
	ValidateConstraints(v, fieldName, value, schema)

	// Validate nested structures
	if typeName, ok := schema["type"].(string); ok {
		switch typeName {
		case "object":
			if objValue, ok := value.(map[string]interface{}); ok {
				v.Map(fieldName, objValue, schema)
			}
		case "array":
			if arrayValue, ok := value.([]interface{}); ok {
				v.Array(fieldName, arrayValue, schema)
			}
		}
	}
}

// ValidateConstraints validates a value against constraint properties in a schema
func ValidateConstraints(v *Validator, fieldName string, value interface{}, schema map[string]interface{}) {
	// Get type from schema
	typeName, _ := schema["type"].(string)

	// Validate based on type
	switch typeName {
	case "number", "integer":
		// Convert to float64 for numeric validation
		var numValue float64
		switch value := value.(type) {
		case float64:
			numValue = value
		case float32:
			numValue = float64(value)
		case int:
			numValue = float64(value)
		case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			numValue = reflect.ValueOf(value).Float()
		default:
			return // Skip non-numeric values
		}

		// Validate minimum
		if min, ok := schema["minimum"].(float64); ok {
			v.Min(fieldName, numValue, min)
		}

		// Validate maximum
		if max, ok := schema["maximum"].(float64); ok {
			v.Max(fieldName, numValue, max)
		}

	case "string":
		strValue, ok := value.(string)
		if !ok {
			return // Skip non-string values
		}

		// Validate minLength
		if minLength, ok := schema["minLength"].(float64); ok {
			v.MinLength(fieldName, strValue, int(minLength))
		}

		// Validate maxLength
		if maxLength, ok := schema["maxLength"].(float64); ok {
			v.MaxLength(fieldName, strValue, int(maxLength))
		}

		// Validate pattern
		if pattern, ok := schema["pattern"].(string); ok && pattern != "" {
			matched, _ := regexp.MatchString(pattern, strValue)
			if !matched {
				v.errors = append(v.errors, fmt.Sprintf("Field '%s' does not match pattern '%s'", fieldName, pattern))
			}
		}

		// Validate format
		if format, ok := schema["format"].(string); ok && format != "" {
			v.Format(fieldName, strValue, format)
		}

		// Validate enum
		if enum, ok := schema["enum"].([]interface{}); ok && len(enum) > 0 {
			// Convert enum values to strings
			enumStrs := make([]string, len(enum))
			for i, e := range enum {
				enumStrs[i] = fmt.Sprintf("%v", e)
			}
			v.Enum(fieldName, strValue, enumStrs)
		}
	}
}

// Enhanced HandleArgs to support more complex validations
func HandleArgsWithSchema[T any](arguments any, schemaMap map[string]interface{}) (*T, error) {
	var args T

	// Ensure arguments is a map[string]interface{}
	argsMap, ok := arguments.(map[string]interface{})
	if !ok {
		if arguments == nil {
			argsMap = make(map[string]interface{})
		} else {
			return nil, fmt.Errorf("invalid arguments format: expected an object/map, got %T", arguments)
		}
	}

	// Validate against schema before decoding
	validator := NewValidator()

	// Get properties map from schema
	properties, hasProps := schemaMap["properties"].(map[string]interface{})

	// Get required fields list
	var requiredFields []string
	if required, ok := schemaMap["required"].([]string); ok {
		requiredFields = required
	}

	// Validate required fields
	for _, field := range requiredFields {
		fieldValue, exists := argsMap[field]
		validator.Required(field, fieldValue)
		if !exists {
			validator.Required(fmt.Sprintf("missing_required_%s", field), nil)
		}
	}

	// Validate each field against schema
	if hasProps {
		for fieldName, propSchema := range properties {
			if fieldValue, exists := argsMap[fieldName]; exists {
				if propMap, ok := propSchema.(map[string]interface{}); ok {
					ValidateValueAgainstSchema(validator, fieldName, fieldValue, propMap)
				}
			}
		}
	}

	// Check for validation errors
	if validator.HasErrors() {
		return nil, fmt.Errorf("validation failed: %v", validator.Errors())
	}

	// Use mapstructure to decode the map into the struct
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &args,
		TagName:  "json",
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("internal error creating argument decoder: %w", err)
	}

	if err := decoder.Decode(argsMap); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %w", err)
	}

	// Validate the struct against its schema definition tags
	if err := ValidateStruct(args); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &args, nil
}

// ValidateAndConvertArgs validates arguments against a schema and converts
// them to the appropriate type based on reflection target type.
// This is a more general version than HandleArgsWithSchema that works with any target Go type.
func ValidateAndConvertArgs(schemaMap map[string]interface{}, args map[string]interface{}, paramType reflect.Type) (interface{}, error) {
	// If parameter type is already map[string]interface{}, just return the args
	if paramType.Kind() == reflect.Map &&
		paramType.Key().Kind() == reflect.String &&
		paramType.Elem().Kind() == reflect.Interface {
		return args, nil
	}

	// If parameter type is interface{}, return the args as is
	if paramType.Kind() == reflect.Interface {
		return args, nil
	}

	// For struct types, create an instance and populate it
	if paramType.Kind() == reflect.Struct ||
		(paramType.Kind() == reflect.Ptr && paramType.Elem().Kind() == reflect.Struct) {

		// Determine the actual struct type and whether we need a pointer
		var structType reflect.Type
		needsPointer := paramType.Kind() == reflect.Ptr

		if needsPointer {
			structType = paramType.Elem()
		} else {
			structType = paramType
		}

		// Create a new instance of the target struct (always start with a pointer)
		target := reflect.New(structType)

		// Validate against schema before decoding
		validator := NewValidator()

		// Get properties map from schema
		properties, hasProps := schemaMap["properties"].(map[string]interface{})

		// Get required fields list
		var requiredFields []string
		if required, ok := schemaMap["required"].([]string); ok {
			requiredFields = required
		}

		// Validate required fields
		for _, field := range requiredFields {
			fieldValue, exists := args[field]
			validator.Required(field, fieldValue)
			if !exists {
				validator.Required(fmt.Sprintf("missing_required_%s", field), nil)
			}
		}

		// Validate each field against schema
		if hasProps {
			for fieldName, propSchema := range properties {
				if fieldValue, exists := args[fieldName]; exists {
					if propMap, ok := propSchema.(map[string]interface{}); ok {
						ValidateValueAgainstSchema(validator, fieldName, fieldValue, propMap)
					}
				}
			}
		}

		// Check for validation errors
		if validator.HasErrors() {
			return nil, fmt.Errorf("validation failed: %v", validator.Errors())
		}

		// Use mapstructure to decode the map into the struct
		decoderConfig := &mapstructure.DecoderConfig{
			Metadata:         nil,
			Result:           target.Interface(),
			TagName:          "json",
			WeaklyTypedInput: true, // Allow type conversion during decoding
			DecodeHook:       mapstructure.StringToTimeHookFunc(time.RFC3339),
		}
		decoder, err := mapstructure.NewDecoder(decoderConfig)
		if err != nil {
			return nil, fmt.Errorf("internal error creating argument decoder: %w", err)
		}

		if err := decoder.Decode(args); err != nil {
			return nil, fmt.Errorf("error parsing arguments: %w", err)
		}

		// Return the appropriate type based on what the parameter expects
		if needsPointer {
			// Parameter expects a pointer to struct
			return target.Interface(), nil
		} else {
			// Parameter expects a struct by value
			return target.Elem().Interface(), nil
		}
	}

	// For slice/array types, create a slice and populate it
	if paramType.Kind() == reflect.Slice || paramType.Kind() == reflect.Array {
		// Get the element type of the slice
		elemType := paramType.Elem()

		// Create a new slice of the appropriate type
		sliceValue := reflect.MakeSlice(paramType, 0, 0)

		// Check if the args is an array
		argsArray, ok := args["_array"].([]interface{})
		if !ok {
			// Cannot type assert map[string]interface{} to []interface{}
			// Instead, if a special key "_array" doesn't exist, look for direct array values
			// or create a wrapper around the map itself
			return nil, fmt.Errorf("expected array arguments with _array key, got map[string]interface{}")
		}

		// Validate the array elements against schema
		validator := NewValidator()
		validator.Array("root", argsArray, schemaMap)
		if validator.HasErrors() {
			return nil, fmt.Errorf("array validation failed: %v", validator.Errors())
		}

		// Process each element in the array
		for i, item := range argsArray {
			// Convert item to map if it's an object
			var itemMap map[string]interface{}
			if m, ok := item.(map[string]interface{}); ok {
				itemMap = m
			} else {
				// For non-object items, create a simple wrapper
				itemMap = map[string]interface{}{"value": item}
			}

			// Convert and validate the item
			itemSchema, ok := schemaMap["items"].(map[string]interface{})
			if !ok {
				itemSchema = map[string]interface{}{"type": "object"}
			}

			convertedItem, err := ValidateAndConvertArgs(itemSchema, itemMap, elemType)
			if err != nil {
				return nil, fmt.Errorf("invalid item at index %d: %w", i, err)
			}

			// Append to the slice
			sliceValue = reflect.Append(sliceValue, reflect.ValueOf(convertedItem))
		}

		return sliceValue.Interface(), nil
	}

	// For map types with string keys
	if paramType.Kind() == reflect.Map && paramType.Key().Kind() == reflect.String {
		// Create a new map of the appropriate type
		mapValue := reflect.MakeMap(paramType)

		// Get element type of the map
		elemType := paramType.Elem()

		// Validate the map values against schema
		validator := NewValidator()
		validator.Map("root", args, schemaMap)
		if validator.HasErrors() {
			return nil, fmt.Errorf("map validation failed: %v", validator.Errors())
		}

		// Get the conversion utility
		converter := NewConverter()

		// Convert and add each entry to the map
		for key, value := range args {
			// Skip special keys like "_type"
			if strings.HasPrefix(key, "_") {
				continue
			}

			// For map[string]interface{}, we can just set directly
			if elemType.Kind() == reflect.Interface {
				mapValue.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(value))
				continue
			}

			// For other types, we need to convert
			var convertedValue interface{}
			var convErr error

			// If we have a schema for additional properties, validate against it
			additionalPropsSchema, hasAddProps := schemaMap["additionalProperties"].(map[string]interface{})
			if hasAddProps {
				// Wrap value in map if it's not already a map
				var valueMap map[string]interface{}
				if m, ok := value.(map[string]interface{}); ok {
					valueMap = m
				} else {
					valueMap = map[string]interface{}{"value": value}
				}

				convertedValue, convErr = ValidateAndConvertArgs(additionalPropsSchema, valueMap, elemType)
			} else {
				// Basic conversion for primitive types
				convertedValue, convErr = converter.Convert(value, elemType)
			}

			if convErr != nil {
				return nil, fmt.Errorf("invalid value for key '%s': %w", key, convErr)
			}

			mapValue.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(convertedValue))
		}

		return mapValue.Interface(), nil
	}

	// For primitive types, convert directly
	if isPrimitiveType(paramType.Kind()) {
		// Since args is already map[string]interface{}, extract "value" or any relevant field
		var valueToConvert interface{}
		if val, exists := args["value"]; exists {
			valueToConvert = val
		} else if len(args) > 0 {
			// Just use the first value found
			for _, v := range args {
				valueToConvert = v
				break
			}
		} else {
			// If no fields, set to nil
			valueToConvert = nil
		}

		// Create a converter
		converter := NewConverter()
		convertedValue, err := converter.Convert(valueToConvert, paramType)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %w", err)
		}
		return convertedValue, nil
	}

	return nil, fmt.Errorf("unsupported parameter type: %s", paramType.String())
}

// isPrimitiveType checks if a kind represents a primitive type.
func isPrimitiveType(kind reflect.Kind) bool {
	switch kind {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return true
	default:
		return false
	}
}

// Converter provides utilities for converting values between different types.
type Converter struct{}

// NewConverter creates a new Converter.
func NewConverter() *Converter {
	return &Converter{}
}

// Convert converts a value to a specified type using various conversion strategies.
func (c *Converter) Convert(value interface{}, targetType reflect.Type) (interface{}, error) {
	// Handle nil value
	if value == nil {
		if targetType.Kind() == reflect.Ptr ||
			targetType.Kind() == reflect.Interface ||
			targetType.Kind() == reflect.Map ||
			targetType.Kind() == reflect.Slice {
			return reflect.Zero(targetType).Interface(), nil
		}
		return nil, fmt.Errorf("cannot set nil value to non-pointer type %s", targetType.String())
	}

	// If value is already a map, extract the actual value if it's a single-value map
	if valueMap, isMap := value.(map[string]interface{}); isMap && len(valueMap) == 1 {
		if v, exists := valueMap["value"]; exists {
			value = v
		}
	}

	// Choose conversion based on target type
	switch targetType.Kind() {
	case reflect.String:
		return conversion.ToString(value)

	case reflect.Bool:
		return conversion.ToBool(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := conversion.ToInt64(value)
		if err != nil {
			return nil, err
		}
		// Convert to the specific int type
		return reflect.ValueOf(val).Convert(targetType).Interface(), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := conversion.ToUint64(value)
		if err != nil {
			return nil, err
		}
		// Convert to the specific uint type
		return reflect.ValueOf(val).Convert(targetType).Interface(), nil

	case reflect.Float32, reflect.Float64:
		val, err := conversion.ToFloat64(value)
		if err != nil {
			return nil, err
		}
		// Convert to the specific float type
		return reflect.ValueOf(val).Convert(targetType).Interface(), nil

	case reflect.Struct:
		// Special handling for time.Time
		if targetType == reflect.TypeOf(time.Time{}) {
			return conversion.ToTime(value)
		}

		// For other structs, convert from map
		valueMap, mapErr := conversion.ToMap(value)
		if mapErr != nil || valueMap == nil {
			return nil, fmt.Errorf("cannot convert %T to struct %s", value, targetType.String())
		}

		// Create a new instance of the target struct
		newStruct := reflect.New(targetType).Interface()

		// Use mapstructure to decode the map into the struct
		decoderConfig := &mapstructure.DecoderConfig{
			Metadata: nil,
			Result:   newStruct,
			TagName:  "json",
		}
		decoder, err := mapstructure.NewDecoder(decoderConfig)
		if err != nil {
			return nil, fmt.Errorf("internal error creating decoder: %w", err)
		}

		if err := decoder.Decode(valueMap); err != nil {
			return nil, fmt.Errorf("failed to convert map to struct %s: %w", targetType.String(), err)
		}

		// Return the struct value (not the pointer)
		return reflect.ValueOf(newStruct).Elem().Interface(), nil

	case reflect.Map:
		// Convert to map and ensure it has string keys
		valueMap, err := conversion.ToMap(value)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %T to map: %w", value, err)
		}

		// Create a new map of the target type
		mapValue := reflect.MakeMap(targetType)
		keyType := targetType.Key()
		elemType := targetType.Elem()

		// Convert and add each entry
		for k, v := range valueMap {
			// Convert the key to the target key type
			keyValue, err := conversion.ToString(k)
			if err != nil {
				return nil, fmt.Errorf("invalid map key: %w", err)
			}

			// Skip if key is empty or starts with underscore
			if keyValue == "" || strings.HasPrefix(keyValue, "_") {
				continue
			}

			// Convert the key to the appropriate type
			var convertedKey reflect.Value
			if keyType.Kind() == reflect.String {
				convertedKey = reflect.ValueOf(keyValue)
			} else {
				// For non-string keys, attempt conversion
				key, err := c.Convert(keyValue, keyType)
				if err != nil {
					return nil, fmt.Errorf("failed to convert map key '%s': %w", keyValue, err)
				}
				convertedKey = reflect.ValueOf(key)
			}

			// Convert the value
			convertedValue, err := c.Convert(v, elemType)
			if err != nil {
				return nil, fmt.Errorf("failed to convert map value for key '%s': %w", keyValue, err)
			}

			// Add to map
			mapValue.SetMapIndex(convertedKey, reflect.ValueOf(convertedValue))
		}

		return mapValue.Interface(), nil

	case reflect.Slice, reflect.Array:
		// Convert to slice
		valueSlice, err := conversion.ToSlice(value)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %T to slice: %w", value, err)
		}

		// Create a new slice of the target type
		elemType := targetType.Elem()
		sliceValue := reflect.MakeSlice(targetType, 0, len(valueSlice))

		// Convert and add each element
		for i, elem := range valueSlice {
			convertedElem, err := c.Convert(elem, elemType)
			if err != nil {
				return nil, fmt.Errorf("failed to convert slice element at index %d: %w", i, err)
			}
			sliceValue = reflect.Append(sliceValue, reflect.ValueOf(convertedElem))
		}

		return sliceValue.Interface(), nil

	case reflect.Ptr:
		// Create a new instance of the target type
		elemType := targetType.Elem()
		elemValue := reflect.New(elemType)

		// Convert the value to the element type
		convertedValue, err := c.Convert(value, elemType)
		if err != nil {
			return nil, err
		}

		// Set the element value
		elemValue.Elem().Set(reflect.ValueOf(convertedValue))
		return elemValue.Interface(), nil

	case reflect.Interface:
		// For interface{}, just return the value as is
		return value, nil

	default:
		return nil, fmt.Errorf("unsupported target type: %s", targetType.String())
	}
}
