//go:build !windows

package tree

import "io/fs"

// IsJunction reports whether the filesystem entry at path is a Windows
// junction. Other systems have none.
func IsJunction(string, fs.FileInfo) (bool, error) { return false, nil }
