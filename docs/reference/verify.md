# `dirloom verify`

Verify answers one question: does the structure currently observed under the selected root still match this valid structural snapshot?

It does not explain what changed. That is a later diff increment. It does not hash file contents.

## Syntax

```text
dirloom verify <snapshot> [directory]
```

```bash
dirloom verify architecture.dlm.json
dirloom verify architecture.dlm.json .
dirloom verify architecture.dlm.json ./src
dirloom verify architecture.dlm.json --root ./src
dirloom verify architecture.dlm.json --format json
```

`snapshot` is a required file path. `-` is not a stdin source. `directory` defaults to the current directory. `--root` is the alternative to the positional directory; combining them is a usage error. More than one directory argument is a usage error.

`--format` is `text` (default) or `json`.

## What is not accepted

Structural overrides are not part of verify. These flags are rejected:

```text
--preset --depth --dirs-only --hidden --ignore
--no-default-ignore --no-gitignore
--config --no-user-config --no-config
--color --icons --theme --style
--output --copy
```

`--config`, `--no-user-config`, and `--no-config` are rejected even though other commands inherit them. Snapshot Capture Semantics are the observation scope. A malformed `.dirloom.yaml` in the tree does not change that scope and does not block verification.

## Capture Semantics

Verify reads `capture` from the validated snapshot and scans once with those rules:

| Snapshot field | Live observation |
| --- | --- |
| `depth` | maximum depth; JSON `null` means unlimited |
| `dirsOnly` | directories only |
| `hidden` | include hidden entries that survive other filters |
| `ignore` | custom patterns, in order |
| `useDefaultIgnores` | built-in directory exclusions |
| `useGitignore` | apply the `.gitignore` files that exist now |

The snapshot does not store the historical text of every `.gitignore`. When `useGitignore` is true, verify applies the current files. If those rules change the observed structure, the fingerprint can change.

The reference file is omitted from the live scan only when it sits inside the selected root and that relative path is not already a node in the expected artifact. A snapshot that legitimately describes a file at the reference path keeps that node.

## Text output

Match, exit 0, stderr empty:

```text
Structure matches snapshot.
```

Mismatch, exit 1, stderr empty:

```text
Structure differs from snapshot.
Expected: dlm:v1:sha256:<digest>
Actual:   dlm:v1:sha256:<digest>
```

There is no node list, no rename or move explanation, and no suggestion to run `dirloom diff`.

Other outcomes leave stdout empty and print one `Error:` line on stderr:

| Situation | stderr prefix | Exit |
| --- | --- | ---: |
| invalid arguments | `Error:` | 2 |
| corrupt snapshot | `Error: invalid snapshot:` | 3 |
| unsupported schema, artifact, or required feature | `Error: unsupported snapshot:` | 4 |
| unreadable snapshot or live observation failure | `Error:` | 5 |
| internal invariant | `Error: internal error:` | 6 |

An embedded fingerprint that does not match the snapshot artifact is exit 3, not exit 1.

## JSON output

`--format json` writes Verify Result Schema v1 to stdout. See [verify-result-v1.md](../contracts/verify-result-v1.md). Stderr stays empty for every classified outcome that was fully written. The document is UTF-8, two-space indented, without HTML escaping, and ends with a newline. `MATCH` and `MISMATCH` always include `expectedFingerprint` and `actualFingerprint`.

## Content and relocation

File content changes do not affect the result. Renaming the physical root, or copying the same relative tree elsewhere, still matches. Adding, removing, renaming, or moving a path mismatches, without saying which of those happened. Symlink and junction identity follow the existing scanner: kind and target are compared; targets are not followed.

## CI

The contract is the process exit plus `--format json`. Human prose is not required.

POSIX:

```bash
set +e
dirloom verify architecture.dlm.json --format json > verify.json
code=$?
set -e

case "$code" in
  0) echo "match" ;;
  1) echo "structural mismatch" ;;
  2) echo "usage error" ;;
  3) echo "invalid snapshot" ;;
  4) echo "unsupported snapshot" ;;
  5) echo "operational error" ;;
  6) echo "internal error" ;;
esac
```

PowerShell:

```powershell
& dirloom verify architecture.dlm.json --format json |
    Set-Content -Encoding utf8 verify.json
$code = $LASTEXITCODE
```

`$LASTEXITCODE` uses the same numbers. GitLab CI, Azure DevOps, and Jenkins can use this process contract directly: separate stdout and stderr, UTF-8, no TTY, and no ANSI.

## A3 and A4

Verify reports whether the fingerprints are equal. A later diff command can name added paths, removed paths, type changes, and target changes. Verify does not emit that data and does not reserve a fake diff field in the JSON result.
