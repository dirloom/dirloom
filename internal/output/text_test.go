package output

import "testing"

func TestValidateTextAcceptsUTF8AndRejectsInvalid(t *testing.T) {
	if err := ValidateText([]byte("tree/\n└── file\n")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateText([]byte{0xff, 0xfe, '\n'}); err == nil {
		t.Fatal("invalid UTF-8 must be rejected")
	}
}
