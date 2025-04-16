// Package validator provides validation utilities for MCP tool arguments and other structures.
package validator

import (
	"fmt"
	"reflect"
	"strings"
)

// Validator helps validate tool arguments using a fluent interface.
type Validator struct {
	errors []string
}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Required checks if a field is present and not empty.
func (v *Validator) Required(field string, value interface{}) *Validator {
	if value == nil {
		v.errors = append(v.errors, fmt.Sprintf("%s is required", field))
		return v
	}

	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			v.errors = append(v.errors, fmt.Sprintf("%s is required", field))
			return v
		}
		val = val.Elem()
	}

	if val.Kind() == reflect.String && val.String() == "" {
		v.errors = append(v.errors, fmt.Sprintf("%s cannot be empty", field))
	}
	return v
}

// MinLength checks if a string field meets minimum length requirement.
func (v *Validator) MinLength(field string, value string, min int) *Validator {
	if len(value) < min {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at least %d characters", field, min))
	}
	return v
}

// MaxLength checks if a string field meets maximum length requirement.
func (v *Validator) MaxLength(field string, value string, max int) *Validator {
	if len(value) > max {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at most %d characters", field, max))
	}
	return v
}

// Min checks if a numeric field meets minimum value requirement.
func (v *Validator) Min(field string, value, min int) *Validator {
	if value < min {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at least %d", field, min))
	}
	return v
}

// Max checks if a numeric field meets maximum value requirement.
func (v *Validator) Max(field string, value, max int) *Validator {
	if value > max {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at most %d", field, max))
	}
	return v
}

// Errors returns any validation errors.
func (v *Validator) Errors() []string {
	return v.errors
}

// HasErrors checks if there are any validation errors.
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Error returns a combined error message if there are any validation errors.
func (v *Validator) Error() error {
	if !v.HasErrors() {
		return nil
	}
	return fmt.Errorf("validation failed: %v", v.errors)
}

// Arguments enforces `required` and `enum` struct tags for validation.
// Usage: if err := validator.Arguments(args); err != nil { ... }
func Arguments(s interface{}) error {
	v := reflect.ValueOf(s)
	t := reflect.TypeOf(s)
	if t.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		// Required check
		if field.Tag.Get("required") == "true" {
			empty := false
			switch value.Kind() {
			case reflect.String:
				empty = value.String() == ""
			case reflect.Slice, reflect.Array:
				empty = value.Len() == 0
			case reflect.Ptr, reflect.Interface:
				empty = value.IsNil()
			}
			if empty {
				return fmt.Errorf("%s is required", field.Name)
			}
		}
		// Enum check
		enumTag := field.Tag.Get("enum")
		if enumTag != "" && value.Kind() == reflect.String {
			allowed := strings.Split(enumTag, ",")
			found := false
			for _, a := range allowed {
				if value.String() == a {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%s must be one of [%s]", field.Name, enumTag)
			}
		}
	}
	return nil
}
