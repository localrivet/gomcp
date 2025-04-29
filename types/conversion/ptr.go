package conversion

// StrPtr returns a pointer to the given string value.
func StrPtr(s string) *string {
	return &s
}
