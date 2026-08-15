//go:build !windows

package presentation

import "os"

func prepareANSI(_ *os.File) (func() error, error) {
	return func() error { return nil }, nil
}
