package app

import (
	"errors"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/watch"
)

// sharingViolation reports whether err is ERROR_SHARING_VIOLATION: a scan
// lists a directory through a handle opened without FILE_SHARE_DELETE, so the
// directory cannot be renamed or removed for that instant (anomaly A10 of the
// v0.4.0-rc1 plan, to be fixed in LOT 5).
func sharingViolation(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// sharingViolationMessage reports whether a diagnostic message carries the
// system text of ERROR_SHARING_VIOLATION, in the language of this machine.
func sharingViolationMessage(message string) bool {
	return strings.Contains(message, windows.ERROR_SHARING_VIOLATION.Error())
}

// TestA10ToleranceIsNarrow pins the test-only tolerance of A10 to the
// transient sharing violation: nothing else is excused.
func TestA10ToleranceIsNarrow(t *testing.T) {
	shared := `read directory "src/lib": open .\src\lib: ` + windows.ERROR_SHARING_VIOLATION.Error()
	denied := `read directory "src/lib": open .\src\lib: ` + windows.ERROR_ACCESS_DENIED.Error()
	event := func(code, message string, fatal bool) watch.Event {
		return watch.Event{Kind: watch.KindError, Fatal: fatal, Diagnostic: watch.Diagnostic{Code: code, Message: message}}
	}
	if !transientSharingError(event(artifact.CodeSourceRead, shared, false)) {
		t.Fatal("the transient sharing violation is not tolerated")
	}
	for name, ev := range map[string]watch.Event{
		"fatal":           event(artifact.CodeSourceRead, shared, true),
		"another code":    event(artifact.CodeUnsupportedKind, shared, false),
		"access denied":   event(artifact.CodeSourceRead, denied, false),
		"not an error":    {Kind: watch.KindStateChanged, Diagnostic: watch.Diagnostic{Code: artifact.CodeSourceRead, Message: shared}},
		"no system error": event(artifact.CodeSourceRead, "read directory: entry no longer exists", false),
	} {
		if transientSharingError(ev) {
			t.Fatalf("%s is tolerated", name)
		}
	}
	if !sharingViolation(&os.PathError{Op: "unlinkat", Path: "d", Err: windows.ERROR_SHARING_VIOLATION}) {
		t.Fatal("a sharing violation of an operation is not retried")
	}
	if sharingViolation(&os.PathError{Op: "unlinkat", Path: "d", Err: windows.ERROR_ACCESS_DENIED}) || sharingViolation(errors.New("other")) {
		t.Fatal("another error is retried")
	}
}
