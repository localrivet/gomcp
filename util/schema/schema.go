// Package schema provides utilities for generating MCP tool input schemas from Go structs.
package schema

import (
	"fmt" // Added for error formatting
	"reflect"
	"strings"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/util/conversion"
	"github.com/localrivet/gomcp/util/validator"
	"github.com/mitchellh/mapstructure" // Added mapstructure import
)

// goTypeToMCPType maps Go kinds to MCP schema types.
func goTypeToMCPType(kind reflect.Kind) string {
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

// FromStruct generates a protocol.ToolInputSchema from struct tags.
// It examines the struct fields and their tags to create a schema that describes
// the expected input format for an MCP tool.
func FromStruct(v interface{}) protocol.ToolInputSchema {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	props := map[string]protocol.PropertyDetail{}
	requiredFields := []string{}         // Initialize slice for required fields
	trackFields := make(map[string]bool) // Track fields to prevent duplicates in requiredFields

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
		} else {
			// For exported fields without JSON tags, use lowercase field name for schema
			// This creates more natural looking JSON while still allowing case-insensitive matching
			name = strings.ToLower(field.Name)
		}

		// Determine if field is required (convention: non-pointer types are required)
		isPtr := field.Type.Kind() == reflect.Ptr
		if !isPtr && !trackFields[name] {
			requiredFields = append(requiredFields, name)
			trackFields[name] = true
		}

		// Determine the schema type (handle pointers correctly for type mapping)
		fieldType := field.Type
		if isPtr {
			fieldType = fieldType.Elem() // Get the underlying type for pointers
		}
		schemaType := goTypeToMCPType(fieldType.Kind())

		// Read enum tag
		enumTag := field.Tag.Get("enum")
		var enumValues []interface{}
		if enumTag != "" {
			enumValuesStr := strings.Split(enumTag, ",")
			enumValues = make([]interface{}, len(enumValuesStr))
			for i, v := range enumValuesStr {
				// Trim whitespace and store as interface{}
				enumValues[i] = strings.TrimSpace(v)
			}
		}

		// Add description from original field name if not explicitly set
		if descTag == "" && jsonTag == "" {
			descTag = fmt.Sprintf("Field for %s", field.Name)
		}

		// Create property definition
		propDetail := protocol.PropertyDetail{
			Type:        schemaType,
			Description: descTag,
			Enum:        enumValues, // Add enum values if tag was present
		}

		// Add to properties map
		props[name] = propDetail
	}

	schema := protocol.ToolInputSchema{
		Type:       "object",
		Properties: props,
	}

	// Only add Required field if there are any required fields
	if len(requiredFields) > 0 {
		schema.Required = requiredFields
	}

	return schema
}

// HandleArgs is a helper function that handles the common pattern of parsing and validating
// tool arguments into a strongly-typed struct.
// HandleArgs is a helper function that handles the common pattern of parsing and validating
// tool arguments (typically map[string]interface{}) into a strongly-typed struct T.
// It now uses mapstructure for more robust decoding.
func HandleArgs[T any](arguments any) (*T, []protocol.Content, bool) {
	var args T

	// Ensure arguments is a map[string]interface{}
	argsMap, ok := arguments.(map[string]interface{})
	if !ok {
		// Handle cases where arguments might be nil or not a map
		if arguments == nil {
			argsMap = make(map[string]interface{}) // Treat nil as empty map
		} else {
			// Attempt conversion if it's some other type that might represent an object
			convertedMap, err := conversion.ToMap(arguments)
			if err != nil {
				return nil, []protocol.Content{protocol.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Invalid arguments format: expected an object/map, got %T", arguments),
				}}, true
			}
			argsMap = convertedMap
		}
	}

	// Use mapstructure to decode the map into the struct
	// Configure mapstructure to handle potential type mismatches gracefully if needed
	// and use json tags by default. Add ErrorUnused if strict matching is desired.
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &args,
		TagName:  "json", // Use json tags like the old code
		// Consider adding WeaklyTypedInput: true for more flexible number/bool parsing
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		// This error is unlikely during decoder creation itself
		return nil, []protocol.Content{protocol.TextContent{
			Type: "text",
			Text: "Internal error creating argument decoder: " + err.Error(),
		}}, true
	}

	if err := decoder.Decode(argsMap); err != nil {
		// mapstructure provides detailed errors
		return nil, []protocol.Content{protocol.TextContent{
			Type: "text",
			Text: "Error parsing arguments: " + err.Error(),
		}}, true
	}

	// Perform validation using the existing validator utility
	if err := validator.Arguments(args); err != nil { // Validate the populated struct
		return nil, []protocol.Content{protocol.TextContent{
			Type: "text",
			Text: "Invalid arguments: " + err.Error(),
		}}, true
	}

	return &args, nil, false
}
