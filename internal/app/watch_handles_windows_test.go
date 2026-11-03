package app

import (
	"testing"

	"github.com/dirloom/dirloom/internal/watch/backend"
)

// handleSlack is what a watch may leave open once it has ended: nothing.
const handleSlack = 0

// openHandles counts the handles the Windows backends hold. The process
// handle count is no measure: Windows creates handles of its own in the
// process (thread pool workers) at any time.
func openHandles(*testing.T) int { return int(backend.HeldHandles()) }
