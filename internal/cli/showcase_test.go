package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/gittest"
)

// The showcase walkthrough, docs/showcase.md, and its README section publish
// the output of the scripts of examples/showcase. A marker names each
// published block:
//
//	<!-- showcase:<section> -->      a section of the scripts' transcript, or
//	                                 the watch demonstration (TestShowcaseWatch)
//	<!-- showcase.sh:<region> -->    the lines of showcase.sh between
//	                                 "# >>> <region>" and "# <<< <region>"
//
// The tests below run the scripts and compare every block byte for byte.
// Set DIRLOOM_UPDATE_GOLDEN=1 to rewrite the blocks from the scripts.

// showcaseRoot is the repository root, from this package.
var showcaseRoot = filepath.Join("..", "..")

// showcaseSections are the sections of the scenario transcript, in order.
var showcaseSections = []string{"project", "snapshot", "verify", "diff", "git", "history"}

// showcaseBlock matches a marker and the fenced block that follows it.
var showcaseBlock = regexp.MustCompile("(?s)<!-- (showcase(?:\\.sh)?):([a-z]+) -->\n```([a-z]*)\n(.*?)```\n")

// showcaseDiff is the change list of the refactoring, from the snapshot to
// the live tree as from main to the branch: the published diff blocks must
// say exactly this.
const showcaseDiff = `Structural Diff

Summary: 2 added, 1 removed, 0 changed, 3 moved, 6 total

ADDED
  + docs/design.md
  + internal/tax

REMOVED
  - docs/architecture.md

MOVED
  > payment -> internal/payment (subtree, 4 nodes)
  > internal/billing/tax.go -> internal/tax/tax.go (name, 1 node)
  > internal/utils -> internal/textutil (subtree, 2 nodes)
`

// TestShowcaseScenario runs the showcase scripts on the dirloom built from
// this tree, with every shell that runs them here, and checks each step:
// its command, its exit code (docs/reference/exit-codes.md) and its key
// output. Every shell must print the same transcript, the one published in
// docs/showcase.md and README.md, on every operating system. The scripts
// run in a hostile environment, so the transcript also proves that they read
// no configuration of the host.
func TestShowcaseScenario(t *testing.T) {
	gittest.Require(t)
	bin := buildVerifyBinary(t)
	var first, firstShell string
	for _, name := range []string{"bash", "pwsh", "powershell"} {
		t.Run(name, func(t *testing.T) {
			shell := findShowcaseShell(t, name)
			work := t.TempDir()
			transcript := runShowcaseScript(t, shell, bin, work)
			if first == "" {
				first, firstShell = transcript, name
			} else if transcript != first {
				t.Fatalf("the %s transcript differs from the %s one:\n%s", name, firstShell, transcript)
			}
			sections := checkShowcaseScenario(t, transcript)
			checkShowcasePages(t, sections)
			checkHostileConfigurationIsLive(t, bin, filepath.Join(work, "shop"), sections["project"])
			checkShowcaseRefusesNonEmptyDirectory(t, shell, bin)
			checkShowcaseNamesItsDirectoryOnFailure(t, shell, bin)
			if shell.script == "showcase.ps1" {
				checkShowcaseRestoresTheSession(t, shell, bin)
			}
		})
	}
}

// TestShowcaseWatch replays the watch demonstration of docs/showcase.md on
// the project that the scenario leaves: a native watch, one file created,
// then an interruption. Every wait is bounded. watch stays experimental: its
// text output is for people and is not a contract.
//
// The output is deterministic without waiting for the watch to be idle,
// which nothing outside the process can observe: after its header, a native
// watch observes again at a time of its own (a verification of its startup
// registrations), and later after each notification. The change is the
// creation of one file: its entry appears at once and nothing is ever
// removed, so every observation, before, during or after the creation, sees
// either the state before or the state after, and never meets an entry that
// disappears. A watch publishes only an observation that differs from the
// last published state, so it publishes exactly one generation. A rename
// would not do: an observation that lists a directory before the rename and
// reads it after meets an entry that no longer exists.
func TestShowcaseWatch(t *testing.T) {
	gittest.Require(t)
	bin := buildVerifyBinary(t)
	native := "bash"
	if runtime.GOOS == "windows" {
		native = "pwsh"
	}
	work := t.TempDir()
	runShowcaseScript(t, findShowcaseShell(t, native), bin, work)
	project := filepath.Join(work, "shop")

	cmd := exec.Command(bin, "watch", "--no-config") // #nosec G204 -- the dirloom built by the test.
	cmd.Dir = project
	cmd.Env = showcaseEnvironment(t, bin)
	ownProcessGroup(cmd)
	stderr := newSyncBuffer()
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// One goroutine reads stdout and one waits for the process; the cleanup
	// kills the process if the test failed and collects both.
	lines := make(chan string)
	quit := make(chan struct{})
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-quit:
				return
			}
		}
	}()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		close(quit)
		_ = cmd.Process.Kill()
		<-done
	})
	read := func(what string, deadline <-chan time.Time) string {
		t.Helper()
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("the watch ended before %s; stderr %q", what, stderr.String())
			}
			return line
		case <-deadline:
			t.Fatalf("no %s from the watch in time; stderr %q", what, stderr.String())
			return ""
		}
	}

	got := []string{read("header", time.After(2*watchWait))}
	if err := os.WriteFile(filepath.Join(project, "internal", "billing", "refund.go"), []byte("package billing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	generation := time.After(watchWait)
	for !strings.HasPrefix(got[len(got)-1], "#1  = ") {
		got = append(got, read("generation #1", generation))
	}
	interruptProcess(t, cmd)
	stopped := time.After(watchWait)
	for open := true; open; {
		select {
		case line, ok := <-lines:
			if ok {
				got = append(got, line)
			}
			open = ok
		case <-stopped:
			t.Fatalf("the watch did not end after the interruption; stdout %q", got)
		}
	}
	select {
	case err := <-done:
		done <- err // for the cleanup
		if err != nil || stderr.String() != "" {
			t.Fatalf("watch exit: %v; stderr %q; stdout %q", err, stderr.String(), got)
		}
	case <-time.After(watchWait):
		t.Fatal("the watch did not exit after the interruption")
	}

	fingerprint := `dlm:v1:sha256:[0-9a-f]{64}`
	want := []*regexp.Regexp{
		regexp.MustCompile(`^Watching structure \(experimental, native\): 20 nodes, ` + fingerprint + `$`),
		regexp.MustCompile(`^#1  \+ internal/billing/refund\.go$`),
		regexp.MustCompile(`^#1  = 21 nodes, ` + fingerprint + `  \(\+1 -0 ~0 >0\)$`),
		regexp.MustCompile(`^Stopped at #1\.$`),
	}
	if len(got) != len(want) {
		t.Fatalf("watch output %q", got)
	}
	for i, re := range want {
		if !re.MatchString(got[i]) {
			t.Fatalf("watch line %d = %q, want %s", i, got[i], re)
		}
	}
	checkShowcasePages(t, map[string]string{
		"watch": "$ dirloom watch --no-config\n" + strings.Join(got, "\n") + "\n[exit 0]\n",
	})
}

// showcaseShell runs one of the scripts: program, then args, then the
// script path and the work directory.
type showcaseShell struct {
	program string
	args    []string
	script  string
}

// findShowcaseShell returns the shell called name, or skips the test when
// it is not installed and not required. The system's own shells are always
// required: bash on Linux and macOS; PowerShell 7 and Windows PowerShell 5.1
// on Windows. On CI every shell is required: PowerShell 7 on Linux and
// macOS, and Git Bash on Windows, as well.
func findShowcaseShell(t *testing.T, name string) showcaseShell {
	t.Helper()
	windows := runtime.GOOS == "windows"
	powerShellArgs := []string{"-NoProfile", "-NonInteractive", "-File"}
	if windows {
		// Windows PowerShell 5.1 refuses scripts under its default policy.
		powerShellArgs = []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File"}
	}
	switch name {
	case "bash":
		if !windows {
			// The pages run the script as a command: it must be executable.
			info, err := os.Stat(filepath.Join(showcaseRoot, "examples", "showcase", "showcase.sh"))
			if err != nil || info.Mode()&0o111 == 0 {
				t.Fatalf("examples/showcase/showcase.sh is not executable: %v", err)
			}
			// macOS ships bash 3.2 as /bin/bash; PATH may hold a newer one.
			if _, err := os.Stat("/bin/bash"); err == nil && runtime.GOOS == "darwin" {
				return showcaseShell{program: "/bin/bash", script: "showcase.sh"}
			}
			bash, err := exec.LookPath("bash")
			if err != nil {
				t.Fatal("bash is required to run showcase.sh")
			}
			return showcaseShell{program: bash, script: "showcase.sh"}
		}
		if bash := gitBash(); bash != "" {
			return showcaseShell{program: bash, script: "showcase.sh"}
		}
		if os.Getenv("CI") != "" {
			t.Fatal("Git Bash is required to run showcase.sh on Windows CI")
		}
		t.Skip("Git Bash is not installed")
	case "pwsh":
		pwsh, err := exec.LookPath("pwsh")
		if err != nil {
			if windows || os.Getenv("CI") != "" {
				t.Fatal("pwsh is required to run showcase.ps1 on Windows and on CI")
			}
			t.Skip("PowerShell 7 is not installed")
		}
		return showcaseShell{program: pwsh, args: powerShellArgs, script: "showcase.ps1"}
	case "powershell":
		if !windows {
			t.Skip("Windows PowerShell 5.1 runs on Windows only")
		}
		powershell, err := exec.LookPath("powershell")
		if err != nil {
			t.Fatal("Windows PowerShell 5.1 is required to run showcase.ps1 on Windows")
		}
		return showcaseShell{program: powershell, args: powerShellArgs, script: "showcase.ps1"}
	}
	t.Fatalf("unknown shell %q", name)
	return showcaseShell{}
}

// gitBash returns the bash of Git for Windows, or "" when there is none.
// The bash.exe of the Windows directory and of WindowsApps starts WSL
// instead.
func gitBash() string {
	if git, err := exec.LookPath("git"); err == nil {
		// git.exe lives in <root>\cmd, <root>\bin or <root>\mingw64\bin.
		dir := filepath.Dir(git)
		for range 3 {
			dir = filepath.Dir(dir)
			bash := filepath.Join(dir, "bin", "bash.exe")
			if _, err := os.Stat(bash); err == nil {
				return bash
			}
		}
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		return ""
	}
	lower := strings.ToLower(bash)
	if system := strings.ToLower(os.Getenv("SystemRoot")); system != "" && strings.HasPrefix(lower, system+`\`) {
		return ""
	}
	if strings.Contains(lower, `\windowsapps\`) {
		return ""
	}
	return bash
}

// runShowcaseScript runs the scenario into work and returns its transcript,
// with LF line ends: PowerShell ends the lines it writes with CRLF on
// Windows. The work directory and DIRLOOM are given as relative paths, which
// the script must resolve before it changes directory. The script must
// succeed with an empty stderr.
func runShowcaseScript(t *testing.T, shell showcaseShell, bin, work string) string {
	t.Helper()
	script, err := filepath.Abs(filepath.Join(showcaseRoot, "examples", "showcase", shell.script))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(work)
	dirloom, err := filepath.Rel(dir, bin)
	if err != nil || !strings.ContainsRune(dirloom, filepath.Separator) {
		t.Fatalf("no relative path from %s to %s: %v", dir, bin, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell.program, append(append([]string{}, shell.args...), script, filepath.Base(work))...) // #nosec G204 -- a shell running a script of this repository.
	cmd.Dir = dir
	cmd.Env = showcaseEnvironment(t, filepath.ToSlash(dirloom))
	// After a timeout, a dirloom or git child may still hold stdout.
	cmd.WaitDelay = 10 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil || stderr.Len() != 0 {
		t.Fatalf("%s %s: %v\nstdout:\n%s\nstderr:\n%s", shell.program, shell.script, err, stdout.String(), stderr.String())
	}
	return strings.ReplaceAll(stdout.String(), "\r\n", "\n")
}

// showcaseEnvironment is the test's environment with DIRLOOM set to dirloom,
// plus a configuration of the host that the scripts must not read:
//   - Git settings and variables that would change the identity, the dates,
//     the branch, the repository or the line ends of the commits, or make
//     them fail by requiring a signature;
//   - the files Git reads without any configuration: the default
//     core.excludesFile, which ignores the snapshot, the default
//     core.attributesFile, which re-encodes every file, and the template of
//     git init, whose commit-msg hook changes every commit;
//   - a user configuration of Dirloom that changes the tree and the snapshot.
func showcaseEnvironment(t *testing.T, dirloom string) []string {
	t.Helper()
	dir := t.TempDir()
	write := func(path, content string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	gitConfig := filepath.Join(dir, "gitconfig")
	write(gitConfig, "[user]\n\tname = Intruder\n\temail = intruder@example.invalid\n"+
		"[commit]\n\tgpgSign = true\n[init]\n\tdefaultBranch = trunk\n[core]\n\tautocrlf = true\n", 0o600)
	xdg := filepath.Join(dir, "xdg")
	write(filepath.Join(xdg, "git", "ignore"), "*.json\n", 0o600)
	write(filepath.Join(xdg, "git", "attributes"), "* text working-tree-encoding=UTF-16LE\n", 0o600)
	templates := filepath.Join(dir, "templates")
	write(filepath.Join(templates, "hooks", "commit-msg"), "#!/bin/sh\necho 'Hostile-Trailer: yes' >>\"$1\"\n", 0o755) // #nosec G306 -- a hook must be executable.
	env := append(os.Environ(),
		"DIRLOOM="+dirloom,
		"GIT_CONFIG_GLOBAL="+gitConfig,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=commit.gpgSign",
		"GIT_CONFIG_VALUE_0=true",
		"GIT_DIR="+filepath.Join(dir, "elsewhere.git"),
		"GIT_AUTHOR_NAME=Intruder",
		"GIT_COMMITTER_DATE=2001-02-03T04:05:06Z",
		"GIT_TEMPLATE_DIR="+templates,
		"XDG_CONFIG_HOME="+xdg,
	)
	// os.UserConfigDir, per system: XDG_CONFIG_HOME on Linux.
	userConfig := xdg
	switch runtime.GOOS {
	case "windows":
		userConfig = filepath.Join(dir, "AppData")
		env = append(env, "APPDATA="+userConfig)
	case "darwin":
		home := filepath.Join(dir, "home")
		userConfig = filepath.Join(home, "Library", "Application Support")
		env = append(env, "HOME="+home)
	}
	write(filepath.Join(userConfig, "dirloom", "config.yaml"), "schemaVersion: 1\n\ndefaults:\n  depth: 1\n\nignore:\n  - payment\n", 0o600)
	return env
}

// checkShowcaseRestoresTheSession runs showcase.ps1 inside a PowerShell
// session, as `& ./showcase.ps1`, with a dirloom that does not exist: the
// script fails after it has changed the environment and the location, and
// must still give the session back as it was.
func checkShowcaseRestoresTheSession(t *testing.T, shell showcaseShell, bin string) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join(showcaseRoot, "examples", "showcase", shell.script))
	if err != nil {
		t.Fatal(err)
	}
	session := `
param([string]$Showcase, [string]$Work)
$ErrorActionPreference = 'Stop'
$env:GIT_DIR = 'sentinel'
$env:GIT_AUTHOR_DATE = 'sentinel'
Remove-Item Env:GIT_CONFIG_NOSYSTEM -ErrorAction SilentlyContinue
$env:DIRLOOM = 'dirloom-showcase-not-installed'
Set-Location -LiteralPath $Work
function Get-SessionState {
  (Get-Location).ProviderPath
  Get-ChildItem Env: | Sort-Object Name | ForEach-Object { "$($_.Name)=$($_.Value)" }
}
$before = Get-SessionState
$failed = $false
try { $null = & $Showcase project } catch { $failed = $true }
if (-not $failed) { throw 'the showcase did not fail without dirloom' }
$changes = Compare-Object $before (Get-SessionState)
if ($changes) { throw "the session changed: $($changes | Out-String)" }
'restored'
`
	sessionPath := filepath.Join(t.TempDir(), "session.ps1")
	if err := os.WriteFile(sessionPath, []byte(session), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell.program, append(append([]string{}, shell.args...), sessionPath, script, t.TempDir())...) // #nosec G204 -- a shell running a test script.
	cmd.Env = showcaseEnvironment(t, bin)
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "restored" {
		t.Fatalf("in-session run: %v\n%s", err, out)
	}
}

// checkHostileConfigurationIsLive proves that the configuration of
// showcaseEnvironment would have changed the transcript: without
// --no-config, the tree of the project differs.
func checkHostileConfigurationIsLive(t *testing.T, bin, project, published string) {
	t.Helper()
	cmd := exec.Command(bin, "--style", "ascii") // #nosec G204 -- the dirloom built by the test.
	cmd.Dir = project
	cmd.Env = showcaseEnvironment(t, bin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("dirloom with the user configuration: %v", err)
	}
	if strings.Contains(published, string(out)) || bytes.Contains(out, []byte("main.go")) {
		t.Fatalf("the hostile user configuration is not read:\n%s", out)
	}
}

// checkShowcaseRefusesNonEmptyDirectory checks that a script never writes
// into a directory that already holds something: exit 2, nothing on stdout.
func checkShowcaseRefusesNonEmptyDirectory(t *testing.T, shell showcaseShell, bin string) {
	t.Helper()
	work := t.TempDir()
	keep := filepath.Join(work, "keep.txt")
	if err := os.WriteFile(keep, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs(filepath.Join(showcaseRoot, "examples", "showcase", shell.script))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell.program, append(append([]string{}, shell.args...), script, work)...) // #nosec G204 -- a shell running a script of this repository.
	cmd.Env = showcaseEnvironment(t, bin)
	cmd.WaitDelay = 10 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "is not empty") {
		t.Fatalf("non-empty work directory: %v; stdout %q; stderr %q", err, stdout.String(), stderr.String())
	}
	entries, err := os.ReadDir(work)
	if err != nil || len(entries) != 1 {
		t.Fatalf("the work directory changed: %v %v", entries, err)
	}
}

// checkShowcaseNamesItsDirectoryOnFailure runs a script without a work
// directory and with a dirloom that does not exist: it fails at its first
// dirloom command, and must still name the temporary directory that it
// created and leaves behind.
func checkShowcaseNamesItsDirectoryOnFailure(t *testing.T, shell showcaseShell, bin string) {
	t.Helper()
	tmp := t.TempDir()
	script, err := filepath.Abs(filepath.Join(showcaseRoot, "examples", "showcase", shell.script))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell.program, append(append([]string{}, shell.args...), script)...) // #nosec G204 -- a shell running a script of this repository.
	// bash's mktemp reads TMPDIR; .NET reads TMPDIR on Unix, TMP and TEMP on
	// Windows.
	cmd.Env = append(showcaseEnvironment(t, bin), "DIRLOOM=dirloom-showcase-not-installed", "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp)
	cmd.WaitDelay = 10 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	entries, readErr := os.ReadDir(tmp)
	if err == nil || readErr != nil || len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "dirloom-showcase.") {
		t.Fatalf("failure without a work directory: %v; %s holds %v (%v); stderr %q", err, tmp, entries, readErr, stderr.String())
	}
	announce := regexp.MustCompile(`(?m)^showcase: the project is in .*` + regexp.QuoteMeta(entries[0].Name()) + `[/\\]shop\r?$`)
	if !announce.MatchString(stderr.String()) {
		t.Fatalf("the script did not name %s on a failure; stderr %q", entries[0].Name(), stderr.String())
	}
}

// showcaseStep is one dirloom command of a transcript.
type showcaseStep struct {
	command string
	output  string
	exit    string
}

// checkShowcaseScenario parses the transcript into its sections and checks
// each step. It returns the sections, as they are published.
func checkShowcaseScenario(t *testing.T, transcript string) map[string]string {
	t.Helper()
	sections := map[string]string{}
	var order []string
	header := regexp.MustCompile(`(?m)^== ([a-z]+) ==\n`)
	bounds := header.FindAllStringSubmatchIndex(transcript, -1)
	if len(bounds) == 0 || bounds[0][0] != 0 {
		t.Fatalf("the transcript does not start with a section:\n%s", transcript)
	}
	for i, b := range bounds {
		end := len(transcript)
		if i+1 < len(bounds) {
			end = bounds[i+1][0]
		}
		name := transcript[b[2]:b[3]]
		order = append(order, name)
		sections[name] = strings.TrimRight(transcript[b[1]:end], "\n") + "\n"
	}
	if strings.Join(order, " ") != strings.Join(showcaseSections, " ") {
		t.Fatalf("sections %q, want %q", order, showcaseSections)
	}

	steps := map[string][]showcaseStep{}
	stepPattern := regexp.MustCompile(`(?s)\$ (dirloom[^\n]*)\n(.*?)\[exit (\d+)\]\n`)
	for name, body := range sections {
		matches := stepPattern.FindAllStringSubmatchIndex(body, -1)
		var rebuilt strings.Builder
		for i, m := range matches {
			if i > 0 {
				rebuilt.WriteString("\n")
			}
			rebuilt.WriteString(body[m[0]:m[1]])
			steps[name] = append(steps[name], showcaseStep{command: body[m[2]:m[3]], output: body[m[4]:m[5]], exit: body[m[6]:m[7]]})
		}
		if rebuilt.String() != body {
			t.Fatalf("section %s is not a list of steps:\n%s", name, body)
		}
	}
	step := func(section string, count, i int, command, exit string) showcaseStep {
		t.Helper()
		if len(steps[section]) != count {
			t.Fatalf("section %s has %d steps, want %d:\n%s", section, len(steps[section]), count, sections[section])
		}
		s := steps[section][i]
		if s.command != command || s.exit != exit {
			t.Fatalf("section %s step %d = %q exit %s, want %q exit %s", section, i, s.command, s.exit, command, exit)
		}
		return s
	}

	// Exit codes: tree output and snapshot 0 on success; verify 0 MATCH and
	// 1 MISMATCH; diff 1 DIFFERENCES; history 0 OK.
	tree := step("project", 1, 0, "dirloom --no-config --style ascii --color never", "0").output
	for _, want := range []string{"shop/\n", "|-- payment/\n", "|   `-- utils/\n", "|       `-- strings.go\n", "`-- README.md\n"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("project tree lacks %q:\n%s", want, tree)
		}
	}
	if out := step("snapshot", 2, 0, "dirloom snapshot --no-config --output architecture.dlm.json", "0").output; out != "" {
		t.Fatalf("snapshot --output wrote %q", out)
	}
	if out := step("snapshot", 2, 1, "dirloom verify architecture.dlm.json", "0").output; out != "Structure matches snapshot.\n" {
		t.Fatalf("verify before the refactoring: %q", out)
	}
	mismatch := regexp.MustCompile(`^Structure differs from snapshot\.\nExpected: (dlm:v1:sha256:[0-9a-f]{64})\nActual:   (dlm:v1:sha256:[0-9a-f]{64})\n$`)
	out := step("verify", 1, 0, "dirloom verify architecture.dlm.json", "1").output
	if m := mismatch.FindStringSubmatch(out); m == nil || m[1] == m[2] {
		t.Fatalf("verify after the refactoring: %q", out)
	}
	if out := step("diff", 1, 0, "dirloom diff snapshot:architecture.dlm.json live:.", "1").output; out != showcaseDiff {
		t.Fatalf("diff of the live tree:\n%s", out)
	}
	if out := step("git", 1, 0, "dirloom diff git:main git:refactor/layout", "1").output; out != showcaseDiff {
		t.Fatalf("diff of the branches:\n%s", out)
	}
	// Newest first: a content-only commit, the refactoring (the counts of
	// the diff above), the snapshot, and the root commit with 17 paths.
	history := regexp.MustCompile(`^Structural History \(first parent, 4 commits examined, end: root\)\n\n` +
		`[0-9a-f]{12}  2026-01-05T12:00:00Z  no structural change\n` +
		`[0-9a-f]{12}  2026-01-05T11:00:00Z  \+2 -1 ~0 >3\n` +
		`[0-9a-f]{12}  2026-01-05T10:00:00Z  \+1 -0 ~0 >0\n` +
		`[0-9a-f]{12}  2026-01-05T09:00:00Z  \+17 -0 ~0 >0  root\n\n` +
		`Changed paths of a commit: dirloom diff git:<commit>\^ git:<commit>\n$`)
	if out := step("history", 1, 0, "dirloom history", "0").output; !history.MatchString(out) {
		t.Fatalf("history:\n%s", out)
	}
	return sections
}

// checkShowcasePages compares the published blocks named in want, and the
// showcase.sh regions, with what the scripts produce, or rewrites them under
// DIRLOOM_UPDATE_GOLDEN=1. docs/showcase.md must publish every block once;
// the README links it and may repeat some.
func checkShowcasePages(t *testing.T, want map[string]string) {
	t.Helper()
	update := os.Getenv("DIRLOOM_UPDATE_GOLDEN") == "1"
	known := map[string]bool{"watch": true}
	for _, name := range showcaseSections {
		known[name] = true
	}
	scriptPath := filepath.Join(showcaseRoot, "examples", "showcase", "showcase.sh")
	script, err := os.ReadFile(scriptPath) // #nosec G304 -- a script of this repository.
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"docs/showcase.md", "README.md"} {
		path := filepath.Join(showcaseRoot, filepath.FromSlash(page))
		data, err := os.ReadFile(path) // #nosec G304 -- a page of this repository.
		if err != nil {
			t.Fatal(err)
		}
		doc := string(data)
		seen := map[string]int{}
		var rewritten strings.Builder
		last := 0
		for _, m := range showcaseBlock.FindAllStringSubmatchIndex(doc, -1) {
			kind, name, lang, body := doc[m[2]:m[3]], doc[m[4]:m[5]], doc[m[6]:m[7]], doc[m[8]:m[9]]
			key := kind + ":" + name
			seen[key]++
			var expected, wantLang string
			var check bool
			switch kind {
			case "showcase":
				if !known[name] {
					t.Fatalf("%s: unknown block %s", page, key)
				}
				expected, check = want[name]
				wantLang = "text"
			case "showcase.sh":
				expected, check = showcaseRegion(t, string(script), name), true
				wantLang = "bash"
			}
			if lang != wantLang {
				t.Fatalf("%s: block %s is %q, want %q", page, key, lang, wantLang)
			}
			if !check || body == expected {
				continue
			}
			if !update {
				t.Fatalf("%s: block %s drifted from the scripts; run with DIRLOOM_UPDATE_GOLDEN=1 to rewrite it\n got:\n%s\nwant:\n%s", page, key, body, expected)
			}
			rewritten.WriteString(doc[last:m[8]])
			rewritten.WriteString(expected)
			last = m[9]
		}
		if last > 0 {
			rewritten.WriteString(doc[last:])
			if err := os.WriteFile(path, []byte(rewritten.String()), 0o644); err != nil { // #nosec G306 -- a page of this repository.
				t.Fatal(err)
			}
		}
		switch page {
		case "docs/showcase.md":
			for name := range known {
				if seen["showcase:"+name] != 1 {
					t.Fatalf("%s publishes block showcase:%s %d times, want once", page, name, seen["showcase:"+name])
				}
			}
			if seen["showcase.sh:refactor"] != 1 {
				t.Fatalf("%s publishes the refactoring %d times, want once", page, seen["showcase.sh:refactor"])
			}
		case "README.md":
			if !strings.Contains(doc, "](docs/showcase.md)") || seen["showcase:diff"] != 1 {
				t.Fatalf("%s must link docs/showcase.md and publish its diff block once", page)
			}
		}
	}
}

// showcaseRegion returns the lines of script between "# >>> name" and
// "# <<< name".
func showcaseRegion(t *testing.T, script, name string) string {
	t.Helper()
	script = strings.ReplaceAll(script, "\r\n", "\n")
	_, rest, found := strings.Cut(script, "# >>> "+name+"\n")
	region, _, closed := strings.Cut(rest, "# <<< "+name+"\n")
	if !found || !closed {
		t.Fatalf("showcase.sh has no region %s", name)
	}
	return region
}
