package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ctrlCConsole carries the process identifier whose console the Ctrl+C
// helper process interrupts.
const ctrlCConsole = "DIRLOOM_CTRL_C_CONSOLE"

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
	procAttachConsole         = kernel32.NewProc("AttachConsole")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procSetConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
)

// ownProcessGroup starts the child in its own process group, so that a
// console control event reaches it alone.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

// interruptProcess sends CTRL_BREAK_EVENT, which Go delivers as
// os.Interrupt.
func interruptProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	pid := uint32(cmd.Process.Pid) //nolint:gosec // G115: process identifiers are non-negative 32-bit values.
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, pid); err != nil {
		t.Fatal(err)
	}
}

// endedByClosedOutput reports exit status 5 with the diagnostic of a
// failed write: Windows has no SIGPIPE.
func endedByClosedOutput(err error, stderr string) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 5 && strings.HasPrefix(stderr, "Error: write watch events: ")
}

// TestWatchProcessEndsOnSignal checks the controlled interruption of a real
// process through CTRL_BREAK_EVENT, which Go delivers as os.Interrupt.
func TestWatchProcessEndsOnSignal(t *testing.T) {
	p := startWatchProcess(t, t.TempDir())
	interruptProcess(t, p.cmd)
	p.endsInterrupted(t)
}

// TestCtrlCHelperProcess is not a test: it sends CTRL_C_EVENT to every
// process of another console, as Ctrl+C typed in that console does. A
// process can only send CTRL_C_EVENT to its own console, so the test binary
// runs this as a separate process that leaves its console for the target's.
// It refuses when that console also holds the test process.
func TestCtrlCHelperProcess(t *testing.T) {
	value := os.Getenv(ctrlCConsole)
	if value == "" {
		t.Skip("helper process for the PowerShell interruption test")
	}
	pid, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		os.Exit(10)
	}
	_, _, _ = procFreeConsole.Call()
	if attached, _, _ := procAttachConsole.Call(uintptr(pid)); attached == 0 {
		os.Exit(11)
	}
	list := make([]uint32, 64)
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&list[0])), uintptr(len(list)))
	if n == 0 || n > uintptr(len(list)) {
		os.Exit(12)
	}
	for _, member := range list[:n] {
		if int(member) == os.Getppid() {
			os.Exit(13)
		}
	}
	// Ignore the event here: it reaches this process too.
	_, _, _ = procSetConsoleCtrlHandler.Call(0, 1)
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, 0); err != nil {
		os.Exit(14)
	}
	os.Exit(0)
}

// childProcesses returns the executable names of the running children of
// parent, by process identifier.
func childProcesses(t *testing.T, parent uint32) map[uint32]string {
	t.Helper()
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()
	children := make(map[uint32]string)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ParentProcessID == parent {
			children[entry.ProcessID] = windows.UTF16ToString(entry.ExeFile[:])
		}
	}
	return children
}

// TestWatchPowerShellExampleAfterCtrlC runs the PowerShell example of the
// watch reference, read from docs/reference/watch.md, in a console of its own
// and stops it with CTRL_C_EVENT, as Ctrl+C in a terminal does. Ctrl+C also
// stops the script, so the finally block must still read exit status 0, and
// the stream must end with watch.stopped.
func TestWatchPowerShellExampleAfterCtrlC(t *testing.T) {
	shell, err := exec.LookPath("pwsh")
	if err != nil {
		t.Fatal("pwsh is required to prove the PowerShell example of the watch reference")
	}
	bin := buildVerifyBinary(t)
	// Run the fresh binary once: its first execution can wait seconds on the
	// system (an antivirus scan), which must not count against the watch.
	if out, err := exec.Command(bin, "--version").CombinedOutput(); err != nil { // #nosec G204 -- the test binary.
		t.Fatalf("dirloom --version: %v\n%s", err, out)
	}
	work := t.TempDir()
	if err := os.Mkdir(filepath.Join(work, "tree"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The example runs on the test binary, on the tree directory and without
	// the user configuration. A process started in a new process group
	// ignores Ctrl+C, and its descendants inherit that, so the script first
	// handles Ctrl+C again, as in a terminal, for itself and the watch. The
	// outer finally block reports the exit code that the example reads.
	script := `
param(
  [Parameter(Mandatory = $true)][string]$Work,
  [Parameter(Mandatory = $true)][string]$Bin
)
if ($PSVersionTable.PSVersion -lt [version]'7.4') { Write-Host "skip $($PSVersionTable.PSVersion)"; exit 0 }
$console = Add-Type -Name ConsoleCtrl -Namespace DirloomTest -PassThru -MemberDefinition '[DllImport("kernel32.dll")] public static extern bool SetConsoleCtrlHandler(System.IntPtr handler, bool add);'
[void]$console::SetConsoleCtrlHandler([System.IntPtr]::Zero, $false)
Set-Location $Work
Set-Content -LiteralPath ready.txt -Value ready
try {
` + powerShellWatchExample(t) + `} finally {
    Write-Host "exit code $($watch.ExitCode)"
}
`
	scriptPath := filepath.Join(t.TempDir(), "watch-ctrl-c.ps1")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	// A file, not a pipe: cmd.Wait must not wait for a dirloom process that
	// still holds the script's output.
	logPath := filepath.Join(t.TempDir(), "script.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = logFile.Close() }()
	readLog := func() string {
		data, _ := os.ReadFile(logPath)
		return strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
	}
	cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-File", scriptPath, work, bin)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	var watcher uint32
	t.Cleanup(func() {
		if watcher != 0 {
			if p, err := os.FindProcess(int(watcher)); err == nil {
				_ = p.Kill()
			}
		}
		_ = cmd.Process.Kill()
		<-exited
	})

	// A cold PowerShell start, with the Add-Type compilation, can take most of
	// the usual wait on a CI runner.
	started := time.Now()
	deadline := started.Add(4 * watchWait)
	for watcher == 0 {
		select {
		case <-exited:
			if version, ok := strings.CutPrefix(readLog(), "skip "); ok {
				t.Skipf("PowerShell %s: the example needs 7.4 or later", version)
			}
			t.Fatalf("the script ended before the watch started:\n%s", readLog())
		default:
		}
		time.Sleep(50 * time.Millisecond)
		children := childProcesses(t, uint32(cmd.Process.Pid)) //nolint:gosec // G115: process identifiers are non-negative 32-bit values.
		for pid, name := range children {
			if strings.EqualFold(name, "dirloom.exe") {
				watcher = pid
			}
		}
		if watcher == 0 && time.Now().After(deadline) {
			_, readyErr := os.Stat(filepath.Join(work, "ready.txt"))
			t.Fatalf("the watch did not start in %s; the script reached it: %t; children of pwsh: %v; script output:\n%s",
				time.Since(started).Round(time.Second), readyErr == nil, children, readLog())
		}
	}
	t.Logf("the watch process appeared %s after the script", time.Since(started).Round(100*time.Millisecond))
	// A process that exists may not run yet. The watch writes the file
	// itself: it runs, its Ctrl+C handler installed, once watch.started is
	// there.
	events := filepath.Join(work, "events.ndjson")
	for {
		if info, err := os.Stat(events); err == nil && info.Size() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the watch published nothing in %s; script output:\n%s", time.Since(started).Round(time.Second), readLog())
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("the watch was running %s after the script", time.Since(started).Round(100*time.Millisecond))

	helper := exec.Command(os.Args[0], "-test.run=^TestCtrlCHelperProcess$") // #nosec G204 -- the test binary itself.
	helper.Env = append(os.Environ(), ctrlCConsole+"="+strconv.Itoa(cmd.Process.Pid))
	if out, err := helper.CombinedOutput(); err != nil {
		t.Fatalf("Ctrl+C helper: %v\n%s", err, out)
	}
	select {
	case <-exited:
	case <-time.After(watchWait):
		t.Fatal("the script did not end after Ctrl+C")
	}

	if got, want := readLog(), "interrupted\nexit code 0"; got != want {
		t.Fatalf("finally blocks wrote %q, want %q", got, want)
	}
	data, err := os.ReadFile(filepath.Join(work, "events.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.ContainsRune(data, '\r') || !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("the redirected stream is not the watch's bytes: %q", data)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, `{"schemaVersion":1,"event":"watch.stopped",`) || !strings.Contains(last, `"reason":"interrupted"`) {
		t.Fatalf("last line %q", last)
	}
}

// powerShellWatchExample returns the PowerShell example of the watch
// reference, starting the test binary on the tree directory without the user
// configuration.
func powerShellWatchExample(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "watch.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.ReplaceAll(string(data), "\r\n", "\n")
	_, rest, found := strings.Cut(doc, "PowerShell 7.4 or later:\n\n```powershell\n")
	example, _, closed := strings.Cut(rest, "```\n")
	const call = "Start-Process dirloom -ArgumentList 'watch', '--format', 'ndjson'"
	if !found || !closed || strings.Count(example, call) != 1 {
		t.Fatalf("docs/reference/watch.md has no PowerShell example starting the watch with %q", call)
	}
	return strings.Replace(example, call, "Start-Process $Bin -ArgumentList 'watch', '--no-config', 'tree', '--format', 'ndjson'", 1)
}
