package snapshot

import (
	"errors"
	"testing"
)

// TestInternalFailuresKeepTheirOwnCode checks that a snapshot error built
// without a cause, a broken Dirloom invariant, carries
// snapshot_internal_failure, never a code documented for invalid input or
// I/O.
func TestInternalFailuresKeepTheirOwnCode(t *testing.T) {
	for name, err := range map[string]error{
		"invalid artifact": invalidArtifact(nil),
		"read failure":     readFailure(nil),
		"write failure":    writeFailure(nil),
	} {
		if CodeOf(err) != CodeInternalFailure || !IsInternal(err) {
			t.Fatalf("%s without a cause = %#v, want an internal %s", name, err, CodeInternalFailure)
		}
	}
	cause := errors.New("cause")
	for code, err := range map[string]error{
		CodeInvalidArtifact: invalidArtifact(cause),
		CodeReadFailure:     readFailure(cause),
		CodeWriteFailure:    writeFailure(cause),
	} {
		if CodeOf(err) != code || IsInternal(err) || !errors.Is(err, cause) {
			t.Fatalf("%s with a cause = %#v", code, err)
		}
	}
}
