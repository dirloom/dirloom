//go:build !windows

package clipboard

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type unixHost struct {
	goos      string
	lookupEnv func(string) (string, bool)
	lookPath  func(string) (string, error)
	run       commandRunner
	wsl       func() bool
}

func newNativeWriter() Writer {
	return &unixWriter{host: nativeUnixHost()}
}

func nativeUnixHost() unixHost {
	return unixHost{
		goos:      runtime.GOOS,
		lookupEnv: os.LookupEnv,
		lookPath:  exec.LookPath,
		run:       runCommand,
		wsl:       detectWSL,
	}
}

type unixWriter struct {
	host unixHost
}

func (writer *unixWriter) Write(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), copyTimeout)
	defer cancel()
	switch writer.host.goos {
	case "darwin":
		return writer.host.run(ctx, "/usr/bin/pbcopy", nil, data)
	case "linux":
		return writer.writeLinux(ctx, data)
	default:
		return fmt.Errorf("clipboard copy is not supported on %s", writer.host.goos)
	}
}

func (writer *unixWriter) writeLinux(ctx context.Context, data []byte) error {
	host := writer.host
	if wayland, _ := host.lookupEnv("WAYLAND_DISPLAY"); wayland != "" {
		if path, err := host.lookPath("wl-copy"); err == nil {
			return host.run(ctx, path, nil, data)
		}
	}
	if display, _ := host.lookupEnv("DISPLAY"); display != "" {
		if path, err := host.lookPath("xclip"); err == nil {
			return host.run(ctx, path, []string{"-selection", "clipboard", "-in"}, data)
		}
		if path, err := host.lookPath("xsel"); err == nil {
			return host.run(ctx, path, []string{"--clipboard", "--input"}, data)
		}
	}
	if host.wsl() {
		if path, err := host.lookPath("clip.exe"); err == nil {
			return host.run(ctx, path, nil, utf16LEWithBOM(data))
		}
	}
	return fmt.Errorf("clipboard copy is unavailable: install wl-copy (Wayland), xclip, or xsel (X11)")
}

func detectWSL() bool {
	if value, ok := os.LookupEnv("WSL_INTEROP"); ok && strings.TrimSpace(value) != "" {
		return true
	}
	if _, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
		return true
	}
	return false
}
