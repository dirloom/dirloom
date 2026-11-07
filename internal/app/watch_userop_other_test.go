//go:build !windows

package app

// heldByAScan is false outside Windows: Unix renames and removes an entry
// whatever handles are open in the tree.
func heldByAScan(error) bool { return false }
