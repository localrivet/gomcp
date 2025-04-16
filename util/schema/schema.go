// Package schema provides utilities for generating MCP tool input schemas from Go structs.
package schema

import (
	"reflect"
	"strings"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/util/conversion"
	"github.com/localrivet/gomcp/util/validator"
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
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		descTag := field.Tag.Get("description")
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := strings.Split(jsonTag, ",")[0]
		props[name] = protocol.PropertyDetail{
			Type:        goTypeToMCPType(field.Type.Kind()),
			Description: descTag,
		}
	}
	return protocol.ToolInputSchema{
		Type:       "object",
		Properties: props,
	}
}

// HandleArgs is a helper function that handles the common pattern of parsing and validating
// tool arguments into a strongly-typed struct.
func HandleArgs[T any](arguments any) (*T, []protocol.Content, bool) {
	var args T

	// Convert arguments to map[string]interface{}
	argsMap, err := conversion.ToMap(arguments)
	if err != nil {
		return nil, []protocol.Content{protocol.TextContent{
			Type: "text",
			Text: "Invalid arguments: " + err.Error(),
		}}, true
	}

	// Convert map to struct using reflection
	val := reflect.ValueOf(&args).Elem()
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := strings.Split(jsonTag, ",")[0]
		if value, ok := argsMap[name]; ok {
			fieldVal := val.Field(i)
			switch field.Type.Kind() {
			case reflect.String:
				if str, err := conversion.ToString(value); err == nil {
					fieldVal.SetString(str)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if num, err := conversion.ToInt(value); err == nil {
					fieldVal.SetInt(int64(num))
				}
			case reflect.Float32, reflect.Float64:
				if num, err := conversion.ToFloat64(value); err == nil {
					fieldVal.SetFloat(num)
				}
			case reflect.Bool:
				if b, err := conversion.ToBool(value); err == nil {
					fieldVal.SetBool(b)
				}
			}
		}
	}

	if err := validator.Arguments(args); err != nil {
		return nil, []protocol.Content{protocol.TextContent{
			Type: "text",
			Text: "Invalid arguments: " + err.Error(),
		}}, true
	}

	return &args, nil, false
}
