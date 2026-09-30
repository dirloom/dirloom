# `dirloom diff`

Diff answers one question: what structurally changed between two sources?

It lists the added, removed, and changed canonical paths. It does not hash
file contents, does not detect renames or moves, and does not read Git refs.

## Syntax

```text
dirloom diff <source-a> <source-b>
```

```bash
dirloom diff snapshot:architecture.dlm.json live:.
dirloom diff live:. snapshot:architecture.dlm.json
dirloom diff snapshot:before.dlm.json snapshot:after.dlm.json
dirloom diff snapshot:architecture.dlm.json live:./src --format json
```

A source is `snapshot:<path>` or `live:<directory>`. The expression is split
on the first colon only; the prefixes are lowercase and case-sensitive. At
least one side must be a snapshot: `live:` against `live:` is a usage error,
because no Capture Semantics would select the observations.

`--format` is `text` (default) or `json`.

## What is not accepted

Structural, configuration, and presentation overrides are not part of diff.
These flags are rejected:

```text
--preset --depth --dirs-only --hidden --ignore
--no-default-ignore --no-gitignore --root
--config --no-user-config --no-config
--color --icons --theme
--output --copy
```

`--config`, `--no-user-config`, and `--no-config` are rejected even though
other commands inherit them. The snapshot's Capture Semantics are the
observation scope. A malformed `.dirloom.yaml` in a live tree does not change
that scope and does not block the comparison.

## Capture Semantics

A `live:` side is observed with the Capture Semantics stored in the opposite
snapshot, exactly like [verify](verify.md):

| Snapshot field | Live observation |
| --- | --- |
| `depth` | maximum depth; JSON `null` means unlimited |
| `dirsOnly` | directories only |
| `hidden` | include hidden entries that survive other filters |
| `ignore` | custom patterns, in order |
| `useDefaultIgnores` | built-in directory exclusions |
| `useGitignore` | apply the `.gitignore` files that exist now |

The opposite snapshot file is omitted from the live scan only when it sits
inside the live root and that relative path is not already a node in the
snapshot artifact. This self-exclusion works in both directions.

## Operations

The vocabulary is frozen at three operations:

| Operation | Meaning |
| --- | --- |
| `ADDED` | the path exists only in source B |
| `REMOVED` | the path exists only in source A |
| `CHANGED` | the path exists in both, with a different kind or target |

`CHANGED` is anchored to Identity Projection v1: only kind and target are
compared. A rename or a move produces one `REMOVED` plus one `ADDED`; there is
no `MOVED` or `RENAMED` operation. The root `.` never appears in the change
list. Changes are sorted by canonical path, byte-wise.

## Text output

No differences, exit 0, stderr empty:

```text
No structural differences.
```

Differences, exit 1, stderr empty:

```text
Structural Diff

Summary: 1 added, 1 removed, 1 changed, 3 total

ADDED
  + b.txt

REMOVED
  - a.txt

CHANGED
  ~ link (target: a -> b)
```

Empty sections are omitted. `CHANGED` entries list only the attributes that
differ; `<none>` marks an absent target and is presentation only. The output
never contains ANSI.

Other outcomes leave stdout empty and print one `Error:` line on stderr:

| Situation | stderr prefix | Exit |
| --- | --- | ---: |
| invalid arguments or source expressions | `Error:` | 2 |
| corrupt snapshot | `Error: invalid snapshot:` | 3 |
| unsupported schema, artifact, or required feature | `Error: unsupported snapshot:` | 4 |
| unreadable snapshot or live observation failure | `Error:` | 5 |
| internal invariant | `Error: internal error:` | 6 |

Exit 1 is a normal result, not an error: stderr stays empty.

## JSON output

`--format json` writes Diff Result Schema v1 to stdout. See
[diff-result-v1.md](../contracts/diff-result-v1.md). The document is UTF-8,
two-space indented, without HTML escaping, and ends with a newline.
`changes` is `[]` when empty, never `null`. Failure documents carry `status`
and a `diagnostic` object with `source` (`a` or `b` when the failing side is
known), `code`, and `message`. Usage failures stay exit 2 with a human stderr
diagnostic even under `--format json`.

## Content and relocation

File content changes do not affect the result: a tree with identical paths,
kinds, and targets reports `NO_DIFFERENCES` no matter how many bytes were
rewritten. Renaming the physical root, or comparing a snapshot against a copy
of the same tree elsewhere, reports no differences. Symlink and junction
identity follow the existing scanner: kind and target are compared; targets
are not followed.

## CI

The contract is the process exit plus `--format json`. Human prose is not
required.

POSIX:

```bash
set +e
dirloom diff snapshot:architecture.dlm.json live:. --format json > diff.json
code=$?
set -e

case "$code" in
  0) echo "no structural differences" ;;
  1) echo "structural differences" ;;
  2) echo "usage error" ;;
  3) echo "invalid snapshot" ;;
  4) echo "unsupported snapshot" ;;
  5) echo "operational error" ;;
  6) echo "internal error" ;;
esac
```

PowerShell:

```powershell
& dirloom diff snapshot:architecture.dlm.json live:. --format json |
    Set-Content -Encoding utf8 diff.json
$code = $LASTEXITCODE
```

`$LASTEXITCODE` uses the same numbers. GitLab CI, Azure DevOps, and Jenkins
can use this process contract directly: separate stdout and stderr, UTF-8, no
TTY, and no ANSI.

## Verify and diff

[verify](verify.md) answers whether the structure still matches, with a
fingerprint. Diff answers what changed, with a path list. Both observe a live
side at most once, with the snapshot's Capture Semantics, and neither reads
current configuration.
