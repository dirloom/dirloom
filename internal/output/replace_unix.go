//go:build !windows

package output

import "os"

func replaceAtomic(source, destination string) error {
	return os.Rename(source, destination)
}
