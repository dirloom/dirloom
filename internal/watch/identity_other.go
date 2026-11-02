//go:build !linux && !darwin && !windows

package watch

import "os"

// identityOf only tells a directory from anything else on platforms without
// a native backend: every directory has the zero identity there.
func identityOf(path string) (Identity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Identity{}, err
	}
	if !info.IsDir() {
		return Identity{}, ErrNotDirectory
	}
	return Identity{}, nil
}
