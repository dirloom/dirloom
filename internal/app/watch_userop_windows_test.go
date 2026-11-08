package app

import (
	"errors"

	"golang.org/x/sys/windows"
)

// heldByAScan reports the two errors with which Windows refuses a user's
// operation for an instant while another process holds a handle in the
// tree: ERROR_ACCESS_DENIED when a directory is renamed while a handle is
// open below it, whatever its share mode, and ERROR_SHARING_VIOLATION when a
// process holds the entry without sharing delete access. Dirloom's scans
// share delete access (A10), so only the first can come from them; the CTO
// accepted it as a Windows limitation on 2026-11-07 (Q5, option A).
func heldByAScan(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
