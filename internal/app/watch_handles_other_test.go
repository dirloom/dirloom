//go:build !linux && !darwin && !windows

package app

import "testing"

const handleSlack = 0

// openHandles is not measured on this platform.
func openHandles(testing.TB) int { return 0 }
