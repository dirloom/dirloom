package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTools stand in for every external program that update-scoop.sh runs
// before and after its version decision, so that the real script runs end to
// end without the network, a token or a Git repository. Each call is appended
// to $FAKE_LOG.
var fakeTools = map[string]string{
	"gh": `#!/usr/bin/env bash
echo "gh $*" >>"$FAKE_LOG"
case "$1 $2" in
"release download")
  pattern="$7"
  dir="$9"
  if [[ "$pattern" == checksums.txt ]]; then
    printf '%s  %s\n%s  %s\n' "$HASH_X64" dirloom_Windows_x86_64.zip "$HASH_ARM" dirloom_Windows_arm64.zip >"$dir/checksums.txt"
  else
    : >"$dir/$pattern"
  fi
  ;;
"repo clone")
  mkdir -p "$4"
  if [[ "$FAKE_CURRENT" == no-version ]]; then
    printf '{\n  "version": ""\n}\n' >"$4/dirloom.json"
  elif [[ -n "$FAKE_CURRENT" ]]; then
    printf '{\n  "version": "%s"\n}\n' "$FAKE_CURRENT" >"$4/dirloom.json"
  fi
  ;;
"pr list")
  echo 0
  ;;
esac
`,
	"git": `#!/usr/bin/env bash
echo "git $*" >>"$FAKE_LOG"
if [[ "$1" == diff ]]; then
  exit 1 # the manifest changed
fi
`,
	"sha256sum": `#!/usr/bin/env bash
for f in "$@"; do
  case "$f" in
  *x86_64.zip) echo "$HASH_X64  $f" ;;
  *arm64.zip) echo "$HASH_ARM  $f" ;;
  esac
done
`,
	"unzip": `#!/usr/bin/env bash
echo "  1  2026-11-28 00:00   dirloom.exe"
`,
	"python3": `#!/usr/bin/env bash
if [[ "$1" == -c ]]; then
  sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' dirloom.json
elif [[ "$1" == - ]]; then
  cat >/dev/null
  echo "python3 write manifest" >>"$FAKE_LOG"
fi
`,
}

func TestUpdateScoopScriptDecision(t *testing.T) {
	cases := []struct {
		name, current, tag string
		status             int
		output             string
		updates            bool
	}{
		{"rc to stable (A6)", "0.4.0-rc1", "v0.4.0", 0, "", true},
		{"stable after stable", "0.3.3", "v0.4.0", 0, "", true},
		{"patch", "0.4.0", "v0.4.1", 0, "", true},
		{"alpha to rc", "0.4.0-a8", "v0.4.0-rc1", 0, "", true},
		{"first manifest", "", "v0.3.3", 0, "", true},
		{"already there", "0.4.0", "v0.4.0", 0, "Scoop manifest already at 0.4.0", false},
		{"stable to its rc", "0.4.0", "v0.4.0-rc1", 1, "Refusing to move Scoop from 0.4.0 to 0.4.0-rc1: not a newer SemVer version", false},
		{"rc to alpha", "0.4.0-rc1", "v0.4.0-a8", 1, "Refusing to move Scoop from 0.4.0-rc1 to 0.4.0-a8: not a newer SemVer version", false},
		{"patch regression", "0.4.1", "v0.4.0", 1, "Refusing to move Scoop from 0.4.1 to 0.4.0: not a newer SemVer version", false},
		{"manifest not SemVer", "latest", "v0.4.0", 2, "not a SemVer 2.0.0 version: 'latest'", false},
		{"tag not SemVer", "0.3.3", "v0.4", 1, "Not a SemVer 2.0.0 version: 0.4", false},
		// FAKE_CURRENT=no-version writes a manifest whose version is empty.
		{"manifest without version", "no-version", "v0.3.2", 1, "Refusing to update Scoop: dirloom.json has no version", false},
	}
	bin := t.TempDir()
	for name, body := range fakeTools {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.ToSlash(filepath.Join(repoRoot(t), ".github", "scripts", "update-scoop.sh"))
	// Fail rather than reach a real gh or git if the fakes do not shadow them.
	runner := `set -u
PATH="$(cd "$FAKE_BIN" && pwd):$PATH"
for tool in gh git sha256sum unzip python3; do
  case "$(command -v "$tool")" in
  "$(cd "$FAKE_BIN" && pwd)/$tool") ;;
  *) echo "fake $tool is not first on PATH: $(command -v "$tool")"; exit 99 ;;
  esac
done
exec bash "$1"`
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			logPath := filepath.Join(t.TempDir(), "calls.log")
			cmd := bashCommand(t, "-c", runner, "update-scoop-runner", script)
			cmd.Env = append(os.Environ(),
				"FAKE_BIN="+filepath.ToSlash(bin),
				"FAKE_LOG="+filepath.ToSlash(logPath),
				"FAKE_CURRENT="+c.current,
				"HASH_X64="+strings.Repeat("a", 64),
				"HASH_ARM="+strings.Repeat("b", 64),
				"GH_TOKEN=fake-token-for-tests",
				"TAG="+c.tag,
				"GITHUB_REPOSITORY=dirloom/dirloom",
				"SCOOP_BUCKET_REPO=dirloom/scoop-bucket",
			)
			out, err := cmd.CombinedOutput()
			status := 0
			if err != nil {
				status = cmd.ProcessState.ExitCode()
			}
			if status == 99 {
				if os.Getenv("CI") != "" {
					t.Fatalf("cannot shadow the real tools on CI: %s", out)
				}
				t.Skipf("cannot shadow the real tools: %s", out)
			}
			if status != c.status {
				t.Fatalf("exit status %d, want %d\n%s", status, c.status, out)
			}
			if c.output != "" && !strings.Contains(string(out), c.output) {
				t.Fatalf("output lacks %q:\n%s", c.output, out)
			}
			logBody, _ := os.ReadFile(logPath)
			calls := string(logBody)
			wrote := strings.Contains(calls, "python3 write manifest")
			opened := strings.Contains(calls, "gh pr create")
			if wrote != c.updates || opened != c.updates {
				t.Fatalf("manifest written %v, PR opened %v, want %v\n--- calls ---\n%s--- output ---\n%s",
					wrote, opened, c.updates, calls, out)
			}
			if c.tag == "v0.4" && calls != "" {
				t.Fatalf("an invalid tag must stop the script before any call:\n%s", calls)
			}
		})
	}
}
