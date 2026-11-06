package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dirloom/dirloom/internal/app"
)

// TestVerifyUnknownStatusIsInternal pins F4 of the v0.4.0-rc1 plan: an
// unknown verify status is an internal error (exit 6) found before any byte
// is written, never an output failure (exit 5).
func TestVerifyUnknownStatusIsInternal(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		var out bytes.Buffer
		err := renderVerifySuccess(&out, format, app.VerifyResult{Status: "BOGUS"})
		var exit *exitError
		if !errors.As(err, &exit) || exit.Code != 6 || exit.Silent {
			t.Fatalf("%s: error = %#v, want a visible exit 6", format, err)
		}
		if out.Len() != 0 {
			t.Fatalf("%s: wrote %q before failing", format, out.String())
		}
	}
}
