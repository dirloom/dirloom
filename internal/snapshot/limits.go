package snapshot

import "fmt"

// Resource limits of a snapshot reader (decision D6 of the v0.4.0-rc1
// plan). A snapshot is untrusted input: a document beyond any limit is
// refused as snapshot_limit_exceeded, an invalid snapshot, before its bytes
// are fully read or its structures built. Loading a valid document at the
// node limit peaks between 1.5 and 2.1 GiB of heap, garbage awaiting
// collection included (BenchmarkLoadPeakMemory).
const (
	// MaxBytes bounds the bytes read; the reader never reads past it.
	MaxBytes = 256 << 20
	// MaxNodes bounds artifact.nodes and any other JSON array.
	MaxNodes = 2_000_000
	// MaxDepth bounds JSON nesting; Snapshot Schema v1 uses 4 levels.
	MaxDepth = 64
	// MaxIgnorePatterns bounds capture.ignore.
	MaxIgnorePatterns = 4096
	// MaxRequiredFeatures bounds requiredFeatures.
	MaxRequiredFeatures = 64
)

// limits are the limits in force; tests lower them to stay small.
var limits = struct {
	bytes, nodes, depth, ignore, features int
}{MaxBytes, MaxNodes, MaxDepth, MaxIgnorePatterns, MaxRequiredFeatures}

func limitExceeded(format string, args ...any) error {
	return &Error{Code: CodeLimitExceeded, Message: fmt.Sprintf(format, args...)}
}
