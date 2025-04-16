// Package conversion provides utilities for converting between types.
package conversion

import (
	"fmt"
	"reflect"
	"strconv"
)

// ToString converts a value to string.
func ToString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}

	switch v := value.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// ToInt converts a value to int.
func ToInt(value interface{}) (int, error) {
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// ToFloat64 converts a value to float64.
func ToFloat64(value interface{}) (float64, error) {
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// ToBool converts a value to bool.
func ToBool(value interface{}) (bool, error) {
	if value == nil {
		return false, nil
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

// ToMap converts a value to map[string]interface{}.
func ToMap(value interface{}) (map[string]interface{}, error) {
	if value == nil {
		return nil, nil
	}

	if m, ok := value.(map[string]interface{}); ok {
		return m, nil
	}

	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Map {
		return nil, fmt.Errorf("cannot convert %T to map[string]interface{}", value)
	}

	result := make(map[string]interface{})
	iter := val.MapRange()
	for iter.Next() {
		key := iter.Key()
		if key.Kind() != reflect.String {
			return nil, fmt.Errorf("map key must be string, got %v", key.Kind())
		}
		result[key.String()] = iter.Value().Interface()
	}

	return result, nil
}

// ToSlice converts a value to []interface{}.
func ToSlice(value interface{}) ([]interface{}, error) {
	if value == nil {
		return nil, nil
	}

	if s, ok := value.([]interface{}); ok {
		return s, nil
	}

	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return nil, fmt.Errorf("cannot convert %T to []interface{}", value)
	}

	result := make([]interface{}, val.Len())
	for i := 0; i < val.Len(); i++ {
		result[i] = val.Index(i).Interface()
	}

	return result, nil
}
