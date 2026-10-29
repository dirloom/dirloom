# `dirloom diff`

Diff answers one question: what structurally changed between two sources?

It lists the added, removed, changed, and moved canonical paths. It does not
hash file contents and does not read Git refs. Moves are detected from
structure alone, conservatively; see [Moves](#moves).

## Syntax

```text
dirloom diff <source-a> <source-b> [--format text|json] [--no-moves]
```

```bash
dirloom diff snapshot:architecture.dlm.json live:.
dirloom diff live:. snapshot:architecture.dlm.json
dirloom diff snapshot:before.dlm.json snapshot:after.dlm.json
dirloom diff snapshot:before.dlm.json snapshot:after.dlm.json --no-moves
dirloom diff snapshot:architecture.dlm.json live:./src --format json
```

A source is `snapshot:<path>` or `live:<directory>`. The expression is split
on the first colon only; the prefixes are lowercase and case-sensitive. At
least one side must be a snapshot: `live:` against `live:` is a usage error,
because no Capture Semantics would select the observations.

`--format` is `text` (default) or `json`. `--no-moves` disables move
detection and lists every changed path, as `v0.4.0-a4` did.

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

| Operation | Meaning |
| --- | --- |
| `ADDED` | the path exists only in source B and is not part of a move |
| `REMOVED` | the path exists only in source A and is not part of a move |
| `CHANGED` | the path exists in both, with a different kind or target |
| `MOVED` | a node removed from A and a node added to B correspond under move detection |

`CHANGED` is anchored to Identity Projection v1: only kind and target are
compared. A path present on both sides is never part of a move. The root `.`
never appears in the change list. Changes are sorted by canonical path,
byte-wise; a `MOVED` is listed at its destination.

## Moves

Move detection uses the strategy `structural-v1`, named in the JSON output as
`metadata.moveDetection`. It pairs a removed node with an added node in two
cases only:

| Case | Rule |
| --- | --- |
| relocation | same name and same structure, and that name with that structure occurs exactly once in each source |
| rename in place | same parent, a non-empty directory, and its structure occurs exactly once in each source |

Structure means the node's kind and target plus, recursively, the names,
kinds, and targets of everything below it. Uniqueness counts every node of
each tree, not only the changed ones. There is no similarity score and no
threshold: any ambiguity stays `REMOVED` plus `ADDED`.

A moved directory is one `MOVED` entry for its whole subtree:

| Field | Meaning |
| --- | --- |
| `from` | origin path in source A |
| `path` | destination path in source B |
| `evidence` | `subtree`: the node has descendants that match exactly; `name`: a file, link, or empty directory matched by name and state |
| `nodes` | records covered by the move, the moved node included |

A `MOVED` is a structural correspondence, not proof that a file was actually
moved. A deleted file and an unrelated new file with the same name, kind, and
target, found nowhere else in either tree, are reported as moved. `evidence:
name` is the weakest case.

These cases are not detected and stay `REMOVED` plus `ADDED`:

- a name or structure that repeats anywhere in either tree (`index.ts`,
  `config.yaml`, generated folders);
- a move combined with a rename;
- a file, link, or empty directory renamed in place, including case-only
  renames;
- a moved directory whose own structure also changed: its intact, uniquely
  identified parts can still appear as `MOVED`;
- a copy, where one removed node matches several added nodes.

The normative rules are in
[Move Detection structural-v1](../contracts/move-detection-structural-v1.md).

## Text output

No differences, exit 0, stderr empty:

```text
No structural differences.
```

Differences, exit 1, stderr empty:

```text
Structural Diff

Summary: 1 added, 1 removed, 1 changed, 1 moved, 4 total

ADDED
  + b.txt

REMOVED
  - a.txt

CHANGED
  ~ link (target: a -> b)

MOVED
  > src/old/payment -> src/features/payment (subtree, 4 nodes)
```

Empty sections are omitted. `CHANGED` entries list only the attributes that
differ; `<none>` marks an absent target and is presentation only. `MOVED`
entries read `from -> path (evidence, nodes)`. The output never contains
ANSI. With `--no-moves`, the summary still shows `0 moved`.

Other outcomes leave stdout empty and print one `Error:` line on stderr:

| Situation | stderr prefix | Exit |
| --- | --- | ---: |
| invalid arguments or source expressions | `Error:` | 2 |
| corrupt snapshot | `Error: invalid snapshot:` | 3 |
| unsupported schema, artifact, or required feature | `Error: unsupported snapshot:` | 4 |
| unreadable snapshot or live observation failure | `Error:` | 5 |
| internal invariant | `Error: internal error:` | 6 |

Exit 1 is a normal result, not an error: stderr stays empty. A diff that only
contains `MOVED` entries is exit 1.

## JSON output

`--format json` writes Diff Result Schema v2 to stdout. See
[diff-result-v2.md](../contracts/diff-result-v2.md). The document is UTF-8,
two-space indented, without HTML escaping, and ends with a newline.
`changes` is `[]` when empty, never `null`. Failure documents carry `status`
and a `diagnostic` object with `source` (`a` or `b` when the failing side is
known), `code`, and `message`. Usage failures stay exit 2 with a human stderr
diagnostic even under `--format json`.

A `MOVED` summarizes its subtree: the document lists how many records moved,
not which. Use `--no-moves` when every changed path is needed.

### From Diff Result Schema v1

`v0.4.0-a4` wrote [Diff Result Schema v1](../contracts/diff-result-v1.md),
which is no longer emitted. Version 2 adds `metadata.moveDetection`,
`summary.moved` (counted in `total`), and the `MOVED` operation with `from`,
`evidence`, and `nodes`. Statuses, exit codes, and diagnostic codes are
unchanged. A consumer that checks `schemaVersion` fails on `2` instead of
misreading the document. With `--no-moves`, the `changes` array and the
`added`, `removed`, and `changed` counts are exactly those of `v0.4.0-a4`.

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
fingerprint. Diff answers what changed, with a path list and moves. Both
observe a live side at most once, with the snapshot's Capture Semantics, and
neither reads current configuration.
