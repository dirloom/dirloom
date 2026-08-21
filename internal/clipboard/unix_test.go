//go:build !windows

package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func TestUnixDarwinUsesPbcopy(t *testing.T) {
	var name string
	var stdin []byte
	writer := &unixWriter{host: unixHost{
		goos: "darwin",
		run: func(_ context.Context, command string, args []string, data []byte) error {
			name = command
			if len(args) != 0 {
				t.Fatalf("args = %#v", args)
			}
			stdin = append([]byte(nil), data...)
			return nil
		},
	}}
	payload := []byte("tree/\n")
	if err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if name != "/usr/bin/pbcopy" || !bytes.Equal(stdin, payload) {
		t.Fatalf("pbcopy name=%q stdin=%q", name, stdin)
	}
}

func TestLinuxPrefersWaylandThenX11ThenWSL(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		tools   []string
		wsl     bool
		wantCmd string
		wantArg []string
		utf16   bool
	}{
		{
			name:    "wayland",
			env:     map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"},
			tools:   []string{"wl-copy", "xclip"},
			wantCmd: "/bin/wl-copy",
		},
		{
			name:    "wayland without wl-copy uses xclip",
			env:     map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"},
			tools:   []string{"xclip"},
			wantCmd: "/bin/xclip",
			wantArg: []string{"-selection", "clipboard", "-in"},
		},
		{
			name:    "xclip",
			env:     map[string]string{"DISPLAY": ":0"},
			tools:   []string{"xclip", "xsel"},
			wantCmd: "/bin/xclip",
			wantArg: []string{"-selection", "clipboard", "-in"},
		},
		{
			name:    "xsel",
			env:     map[string]string{"DISPLAY": ":0"},
			tools:   []string{"xsel"},
			wantCmd: "/bin/xsel",
			wantArg: []string{"--clipboard", "--input"},
		},
		{
			name:    "wsl clip",
			env:     map[string]string{},
			tools:   []string{"clip.exe"},
			wsl:     true,
			wantCmd: "/mnt/c/Windows/System32/clip.exe",
			utf16:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotName string
			var gotArgs []string
			var gotStdin []byte
			writer := &unixWriter{host: unixHost{
				goos: "linux",
				lookupEnv: func(key string) (string, bool) {
					value, ok := test.env[key]
					return value, ok
				},
				lookPath: func(tool string) (string, error) {
					for _, available := range test.tools {
						if available == tool {
							switch tool {
							case "wl-copy":
								return "/bin/wl-copy", nil
							case "xclip":
								return "/bin/xclip", nil
							case "xsel":
								return "/bin/xsel", nil
							case "clip.exe":
								return "/mnt/c/Windows/System32/clip.exe", nil
							}
						}
					}
					return "", errors.New("not found")
				},
				wsl: func() bool { return test.wsl },
				run: func(_ context.Context, command string, args []string, data []byte) error {
					if command == "/bin/sh" || command == "bash" || command == "cmd" {
						t.Fatal("must not invoke a shell")
					}
					gotName = command
					gotArgs = append([]string(nil), args...)
					gotStdin = append([]byte(nil), data...)
					return nil
				},
			}}
			payload := []byte("ok\n")
			if err := writer.Write(payload); err != nil {
				t.Fatal(err)
			}
			if gotName != test.wantCmd {
				t.Fatalf("command = %q, want %q", gotName, test.wantCmd)
			}
			if fmt.Sprintf("%v", gotArgs) != fmt.Sprintf("%v", test.wantArg) && len(test.wantArg)+len(gotArgs) != 0 {
				if len(test.wantArg) != len(gotArgs) {
					t.Fatalf("args = %#v, want %#v", gotArgs, test.wantArg)
				}
				for i := range test.wantArg {
					if gotArgs[i] != test.wantArg[i] {
						t.Fatalf("args = %#v, want %#v", gotArgs, test.wantArg)
					}
				}
			}
			if test.utf16 {
				if bytes.Equal(gotStdin, payload) || !bytes.HasPrefix(gotStdin, []byte{0xff, 0xfe}) {
					t.Fatalf("clip.exe stdin = %v", gotStdin)
				}
			} else if !bytes.Equal(gotStdin, payload) {
				t.Fatalf("stdin = %q", gotStdin)
			}
		})
	}
}

func TestLinuxMissingBackendIsActionable(t *testing.T) {
	writer := &unixWriter{host: unixHost{
		goos:      "linux",
		lookupEnv: func(string) (string, bool) { return "", false },
		lookPath:  func(string) (string, error) { return "", errors.New("missing") },
		wsl:       func() bool { return false },
		run: func(context.Context, string, []string, []byte) error {
			t.Fatal("run must not be called")
			return nil
		},
	}}
	err := writer.Write([]byte("secret-tree\n"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("wl-copy")) {
		t.Fatalf("error = %v", err)
	}
	if bytes.Contains([]byte(err.Error()), []byte("secret-tree")) {
		t.Fatal("error leaked copied content")
	}
}

func TestRunCommandTimesOut(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep is not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := runCommand(ctx, "sleep", []string{"2"}, nil); err == nil {
		t.Fatal("expected timeout")
	}
}

func TestUnsupportedUnix(t *testing.T) {
	writer := &unixWriter{host: unixHost{goos: "plan9"}}
	if err := writer.Write([]byte("x")); err == nil {
		t.Fatal("expected unsupported OS error")
	}
}
