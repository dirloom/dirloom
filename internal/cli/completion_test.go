package cli

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	configuration "github.com/dirloom/dirloom/internal/config"
)

func TestCompletionScriptsAreDeterministicAndTerminated(t *testing.T) {
	for _, shell := range completionShells {
		first, stderr, code := executeForTest(t, "completion", shell)
		if code != 0 || stderr != "" || first == "" || !strings.HasSuffix(first, "\n") {
			t.Fatalf("%s=(len=%d, stderr=%q, code=%d)", shell, len(first), stderr, code)
		}
		second, _, code := executeForTest(t, "completion", shell)
		if code != 0 || first != second {
			t.Fatalf("%s generation is not deterministic", shell)
		}
	}
}

func TestCompletionRejectsInvalidShellAndArity(t *testing.T) {
	stdout, stderr, code := executeForTest(t, "completion", "cmd")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported shell") {
		t.Fatalf("shell=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "completion")
	if code != 2 || stdout != "" {
		t.Fatalf("arity=(%q,%q,%d)", stdout, stderr, code)
	}
	stdout, stderr, code = executeForTest(t, "completion", "bash", "zsh")
	if code != 2 || stdout != "" {
		t.Fatalf("extra=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestCompletionWriterFailure(t *testing.T) {
	loader := configuration.NewLoader()
	var stderr bytes.Buffer
	code := executeWithLoader(context.Background(), []string{"completion", "bash"}, failingWriter{}, &stderr, "v0.1.0-test", loader)
	if code != 1 || !strings.Contains(stderr.String(), "write completion script") {
		t.Fatalf("writer failure code=%d stderr=%q", code, stderr.String())
	}
}

func TestCompleteProtocolOffersSemanticValues(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"__complete", "--format", ""}, []string{"markdown", "json", "mermaid"}},
		{[]string{"__complete", "--style", ""}, []string{"unicode", "ascii"}},
		{[]string{"__complete", "--theme", ""}, []string{"default", "vivid"}},
		{[]string{"__complete", "--color", ""}, []string{"auto", "always", "never"}},
		{[]string{"__complete", "--icons", ""}, []string{"auto", "unicode", "nerd", "never"}},
		{[]string{"__complete", "--depth", ""}, []string{"unlimited"}},
		{[]string{"__complete", "--diagram-view", ""}, []string{"structure"}},
		{[]string{"__complete", "--diagram-direction", ""}, []string{"top-down", "left-right"}},
		{[]string{"__complete", "--preset", ""}, []string{"docs", "ai", "none"}},
		{[]string{"__complete", "completion", ""}, []string{"bash", "zsh", "fish", "powershell"}},
	}
	for _, test := range cases {
		stdout, _, code := executeForTest(t, test.args...)
		if code != 0 {
			t.Fatalf("%v code=%d stdout=%q", test.args, code, stdout)
		}
		for _, want := range test.want {
			if !strings.Contains(stdout, want) {
				t.Errorf("%v missing %q\n%s", test.args, want, stdout)
			}
		}
		if !strings.Contains(stdout, ":") {
			t.Errorf("%v missing directive\n%s", test.args, stdout)
		}
	}
}

func TestCompletionScriptSyntax(t *testing.T) {
	if runtime.GOOS == "windows" {
		stdout, stderr, code := executeForTest(t, "completion", "powershell")
		if code != 0 {
			t.Fatal(stderr)
		}
		command := exec.Command("powershell", "-NoProfile", "-Command", "[scriptblock]::Create($input) | Out-Null")
		command.Stdin = strings.NewReader(stdout)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("powershell syntax: %v\n%s", err, output)
		}
		return
	}
	checks := []struct {
		shell   string
		program string
		args    []string
	}{
		{"bash", "bash", []string{"-n"}},
		{"zsh", "zsh", []string{"-n"}},
		{"fish", "fish", []string{"-n"}},
	}
	for _, check := range checks {
		if _, err := exec.LookPath(check.program); err != nil {
			t.Logf("skipping %s syntax: %v", check.program, err)
			continue
		}
		script, stderr, code := executeForTest(t, "completion", check.shell)
		if code != 0 {
			t.Fatalf("%s generate: %s", check.shell, stderr)
		}
		command := exec.Command(check.program, append(check.args, "/dev/stdin")...)
		command.Stdin = strings.NewReader(script)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s syntax: %v\n%s", check.program, err, output)
		}
	}
}
