package common

import "fmt"

// EnumString returns the display name of v, falling back to "TypeName(n)" for unknown values.
func EnumString[T ~uint8](v T, names map[T]string, typeName string) string {
	if s, ok := names[v]; ok {
		return s
	}
	return fmt.Sprintf("%s(%d)", typeName, uint8(v))
}
