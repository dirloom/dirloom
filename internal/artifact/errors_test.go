package artifact

import (
	"errors"
	"testing"
)

func TestAsInternalMarksOnlyInvariants(t *testing.T) {
	marked := AsInternal(invariant("duplicate node %q", "a"))
	var typed *Error
	if !errors.As(marked, &typed) || typed.Code != CodeInvariant || !typed.Internal || typed.Message != `duplicate node "a"` {
		t.Fatalf("AsInternal(invariant) = %#v", marked)
	}
	for _, err := range []error{
		collision("é", "é", "é"),
		invalidPath("bad path"),
		SourceReadFailure(errors.New("denied")),
		errors.New("plain"),
		nil,
	} {
		if got := AsInternal(err); !errors.Is(got, err) || IsInternal(got) {
			t.Fatalf("AsInternal(%v) changed a non-invariant error: %#v", err, got)
		}
	}
	original := invariant("x")
	_ = AsInternal(original)
	if IsInternal(original) {
		t.Fatal("AsInternal modified the error it was given")
	}
}

func TestRecoveredIsInternal(t *testing.T) {
	for _, tc := range []struct {
		value   any
		message string
	}{
		{"bug", "unexpected panic: bug"},
		{errors.New("broken"), "unexpected panic: broken"},
		{nil, "unexpected panic: <nil>"},
	} {
		var typed *Error
		if err := Recovered(tc.value); !errors.As(err, &typed) || typed.Code != CodeInternal || !typed.Internal || typed.Message != tc.message {
			t.Fatalf("Recovered(%v) = %#v, want an internal %s %q", tc.value, err, CodeInternal, tc.message)
		}
	}
}

func TestEncodingFailureIsInternal(t *testing.T) {
	err := EncodingFailure(errors.New("hash refused a write"))
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeEncoding || !typed.Internal {
		t.Fatalf("EncodingFailure = %#v, want an internal %s", err, CodeEncoding)
	}
}
