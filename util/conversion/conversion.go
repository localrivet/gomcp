// Package conversion provides utilities for converting between types.
package conversion

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"
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
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case time.Time:
		return v.Format(time.RFC3339), nil
	case []byte:
		return string(v), nil
	default:
		// For complex types, try JSON
		if reflect.TypeOf(value).Kind() == reflect.Struct ||
			reflect.TypeOf(value).Kind() == reflect.Map ||
			reflect.TypeOf(value).Kind() == reflect.Slice {
			b, err := json.Marshal(value)
			if err == nil {
				return string(b), nil
			}
		}
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
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case float32:
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
	case time.Time:
		return int(v.Unix()), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// ToInt64 converts a value to int64.
func ToInt64(value interface{}) (int64, error) {
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case time.Time:
		return v.Unix(), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", value)
	}
}

// ToUint64 converts a value to uint64.
func ToUint64(value interface{}) (uint64, error) {
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case int:
		if v < 0 {
			return 0, fmt.Errorf("cannot convert negative int to uint64")
		}
		return uint64(v), nil
	case int8:
		if v < 0 {
			return 0, fmt.Errorf("cannot convert negative int8 to uint64")
		}
		return uint64(v), nil
	case int16:
		if v < 0 {
			return 0, fmt.Errorf("cannot convert negative int16 to uint64")
		}
		return uint64(v), nil
	case int32:
		if v < 0 {
			return 0, fmt.Errorf("cannot convert negative int32 to uint64")
		}
		return uint64(v), nil
	case int64:
		if v < 0 {
			return 0, fmt.Errorf("cannot convert negative int64 to uint64")
		}
		return uint64(v), nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case float32:
		if v < 0 {
			return 0, fmt.Errorf("cannot convert negative float32 to uint64")
		}
		return uint64(v), nil
	case float64:
		if v < 0 {
			return 0, fmt.Errorf("cannot convert negative float64 to uint64")
		}
		return uint64(v), nil
	case string:
		return strconv.ParseUint(v, 10, 64)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", value)
	}
}

// ToFloat32 converts a value to float32.
func ToFloat32(value interface{}) (float32, error) {
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case float32:
		return v, nil
	case float64:
		return float32(v), nil
	case int:
		return float32(v), nil
	case int8:
		return float32(v), nil
	case int16:
		return float32(v), nil
	case int32:
		return float32(v), nil
	case int64:
		return float32(v), nil
	case uint:
		return float32(v), nil
	case uint8:
		return float32(v), nil
	case uint16:
		return float32(v), nil
	case uint32:
		return float32(v), nil
	case uint64:
		return float32(v), nil
	case string:
		f, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return 0, err
		}
		return float32(f), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float32", value)
	}
}

// ToFloat64 converts a value to float64.
func ToFloat64(value interface{}) (float64, error) {
	if value == nil {
		return 0, nil
	}

	switch v := value.(type) {
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case time.Time:
		return float64(v.Unix()), nil
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
	case int8:
		return v != 0, nil
	case int16:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case uint:
		return v != 0, nil
	case uint8:
		return v != 0, nil
	case uint16:
		return v != 0, nil
	case uint32:
		return v != 0, nil
	case uint64:
		return v != 0, nil
	case float32:
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

	// Direct map type
	if m, ok := value.(map[string]interface{}); ok {
		return m, nil
	}

	// Try to unmarshal JSON string
	if str, ok := value.(string); ok {
		var result map[string]interface{}
		err := json.Unmarshal([]byte(str), &result)
		if err == nil {
			return result, nil
		}
	}

	// Try reflection for struct or map
	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, nil
		}
		val = val.Elem()
	}

	// Handle struct by converting to map
	if val.Kind() == reflect.Struct {
		// Special case for time.Time
		if _, ok := value.(time.Time); ok {
			return nil, fmt.Errorf("cannot convert time.Time to map[string]interface{}")
		}

		// Convert struct to JSON and then to map
		data, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal struct: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
		}
		return result, nil
	}

	// Handle map
	if val.Kind() == reflect.Map {
		result := make(map[string]interface{})
		iter := val.MapRange()
		for iter.Next() {
			key := iter.Key()
			// Try to convert key to string
			keyStr, err := ToString(key.Interface())
			if err != nil {
				return nil, fmt.Errorf("map key must be convertible to string, got %v", key.Kind())
			}
			result[keyStr] = iter.Value().Interface()
		}
		return result, nil
	}

	return nil, fmt.Errorf("cannot convert %T to map[string]interface{}", value)
}

// ToSlice converts a value to []interface{}.
func ToSlice(value interface{}) ([]interface{}, error) {
	if value == nil {
		return nil, nil
	}

	// Direct slice type
	if s, ok := value.([]interface{}); ok {
		return s, nil
	}

	// Try to unmarshal JSON string
	if str, ok := value.(string); ok {
		var result []interface{}
		err := json.Unmarshal([]byte(str), &result)
		if err == nil {
			return result, nil
		}
	}

	// Try reflection
	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, nil
		}
		val = val.Elem()
	}

	if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
		result := make([]interface{}, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = val.Index(i).Interface()
		}
		return result, nil
	}

	return nil, fmt.Errorf("cannot convert %T to []interface{}", value)
}

// ToTime converts a value to time.Time.
func ToTime(value interface{}) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}

	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		// Try RFC3339 format first
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			return t, nil
		}

		// Try RFC3339Nano
		t, err = time.Parse(time.RFC3339Nano, v)
		if err == nil {
			return t, nil
		}

		// Try common formats
		formats := []string{
			"2006-01-02",
			"2006-01-02 15:04:05",
			"01/02/2006",
			"01/02/2006 15:04:05",
			"2006/01/02",
			"2006/01/02 15:04:05",
			time.RFC1123,
			time.RFC1123Z,
			time.RFC822,
			time.RFC822Z,
			time.ANSIC,
		}

		for _, format := range formats {
			t, err = time.Parse(format, v)
			if err == nil {
				return t, nil
			}
		}

		// Try to parse Unix timestamp
		i, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return time.Unix(i, 0), nil
		}

		return time.Time{}, fmt.Errorf("unable to parse time from string: %s", v)
	case int:
		return time.Unix(int64(v), 0), nil
	case int64:
		return time.Unix(v, 0), nil
	case float64:
		sec, nsec := math.Modf(v)
		return time.Unix(int64(sec), int64(nsec*1e9)), nil
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to time.Time", value)
	}
}

// FromJSON converts a JSON string to a Go value.
func FromJSON(jsonStr string, target interface{}) error {
	return json.Unmarshal([]byte(jsonStr), target)
}

// ToJSON converts a Go value to a JSON string.
func ToJSON(value interface{}) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToPrettyJSON converts a Go value to a formatted JSON string with indentation.
func ToPrettyJSON(value interface{}) (string, error) {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
