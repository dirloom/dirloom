//go:build !windows

package app

// sharingViolation is false outside Windows: anomaly A10 is Windows-only.
func sharingViolation(error) bool { return false }

// sharingViolationMessage is false outside Windows: anomaly A10 is
// Windows-only.
func sharingViolationMessage(string) bool { return false }
