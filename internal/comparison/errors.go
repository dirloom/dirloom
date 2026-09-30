package comparison

import "errors"

// Side identifies one compared source in user order.
type Side string

const (
	// SideA is the left source expression.
	SideA Side = "a"
	// SideB is the right source expression.
	SideB Side = "b"
)

// SideError preserves which side of a comparison produced err. Callers must
// never recover the side from message text.
type SideError struct {
	Side Side
	Err  error
}

func (e *SideError) Error() string { return e.Err.Error() }

func (e *SideError) Unwrap() error { return e.Err }

// SideOf returns the responsible side when err carries one.
func SideOf(err error) (Side, bool) {
	var typed *SideError
	if err != nil && errors.As(err, &typed) {
		return typed.Side, true
	}
	return "", false
}
