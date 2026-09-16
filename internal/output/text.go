package output

import (
	"fmt"
	"unicode/utf8"
)

// ValidateText enforces the public UTF-8 invariant of every textual render.
func ValidateText(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("rendered output is not valid UTF-8")
	}
	return nil
}
