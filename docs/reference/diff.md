# `dirloom diff`

Diff answers one question: what structurally changed between two sources?

It lists the added, removed, changed, and moved canonical paths. It does not
hash file contents. A `git:` source reads a committed Git tree, never the
working tree or the index; see [Git sources](#git-sources). Moves are
detected from structure alone, conservatively; see [Moves](#moves).

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
dirloom diff git:main git:feature
dirloom diff git:v0.4.0-a4 git:HEAD --format json
dirloom diff snapshot:architecture.dlm.json git:HEAD
```

A source is `snapshot:<path>`, `live:<directory>`, or `git:<revision>`. The
expression is split on the first colon only, so a revision may contain colons
(`git:HEAD:src`); the prefixes are lowercase and case-sensitive.

| Pairing | Observation |
| --- | --- |
| `snapshot:` and `snapshot:` | both stored artifacts |
| `snapshot:` and `live:`, either order | the live side, with the snapshot's Capture Semantics |
| `snapshot:` and `git:`, either order | the Git side, with the snapshot's Capture Semantics |
| `git:` and `git:` | both whole committed trees, without filtering |
| `live:` and `live:` | usage error, exit 2 |
| `git:` and `live:`, either order | usage error, exit 2 |

`live:` against `live:` and `git:` against `live:` are refused because
neither side carries Capture Semantics to select the observation. Compare a
revision with a snapshot of the working tree, or with a second revision.

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

A `live:` or `git:` side is observed with the Capture Semantics stored in the
opposite snapshot, exactly like [verify](verify.md):

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

On a `git:` side, `useGitignore` applies the `.gitignore` blobs of the
observed tree at their tree paths, and `hidden` means a name that starts with
`.`. A `git:` side against another `git:` side has no Capture Semantics:
every entry of both trees is observed.

## Git sources

`git:<revision>` observes the tree a revision names, in the repository of the
current directory. Git finds the repository as it always does: from a
subdirectory, a linked worktree, or a bare repository, honoring `GIT_DIR` and
`GIT_WORK_TREE`.

| Revision | Observed tree | `commit` in JSON |
| --- | --- | --- |
| a branch, a tag, `HEAD`, `HEAD~3`, a full or abbreviated commit id | the commit's tree | that commit |
| a tag pointing to a tree | that tree | absent |
| `<commit>:<path>`, such as `HEAD:src` | that subtree | absent |
| a file (`HEAD:README.md`), or a revision that does not resolve | none | exit 5, `git_revision_not_found` |

These revisions are refused before Git runs, with exit 2: an empty revision,
one that starts with `-`, one that contains a NUL or a newline, and
an index entry (`:<path>`, `:<stage>:<path>`), because resolving it would
read the index. `:/<text>`, which searches commit messages, is accepted.

What a Git source reads, and what it does not:

- committed objects only: never the working tree and never the index, so
  uncommitted and untracked changes do not exist for it;
- nothing over the network: in a partial clone, a missing object is an error
  and is never fetched;
- the recorded objects: `refs/replace/` never substitutes a commit;
- a submodule is an empty directory, never entered;
- a link target as recorded in the tree, NFC-normalized, on every platform:
  no separator and no root rewrite;
- only the trees of the directories the observation enters: a subtree that
  depth or ignore rules exclude is never read and cannot fail.

`git` 2.45 or newer must be on `PATH`. Dirloom runs `git version`,
`git rev-parse` and `git cat-file` only, without a shell. The normative rules
are in [Git Tree Source v1](../contracts/git-tree-source-v1.md).

### Not `git diff`

A Git source is still structural: paths, kinds and link targets. File
contents never matter, and neither does the executable bit:

```text
main:     src/app.go  (contents A)
feature:  src/app.go  (contents B)

dirloom diff git:main git:feature   ->   No structural differences.  (exit 0)
```

A `MOVED` between two revisions is a structural correspondence. It is
neither Git's rename detection nor history.

### A revision against a snapshot of its checkout

Under the same Capture Semantics, `git:<rev>` and a snapshot of a clean
checkout of `<rev>` are `NO_DIFFERENCES` when the checkout holds the tree
without loss. These differences are expected and listed in
[Git Tree Source v1](../contracts/git-tree-source-v1.md#equivalence-with-a-checkout):

| In the checkout | In `git:<rev>` |
| --- | --- |
| a link checked out as a file (`core.symlinks=false`, the Windows default) | a link |
| an absolute link target inside the checkout, rewritten relative to its root | the target as recorded |
| the files of an initialized submodule | an empty directory |
| one of two names that differ only by case, on Windows or macOS | both names |
| a `.gitignore` above 1 MiB, read | `git_tree_invalid` |
| an entry hidden by the Windows hidden attribute | the entry |
| untracked or uncommitted files, and `.git` itself without default ignores | absent |

### Git diagnostics

Every Git failure is `OBSERVATION_ERROR`, exit 5, for the side that failed:

| Code | Cause |
| --- | --- |
| `git_unavailable` | no `git` on `PATH`, or a version below 2.45 |
| `git_repository_not_found` | the current directory is not inside a repository, or Git refuses it (`safe.directory`) |
| `git_revision_not_found` | the revision does not resolve locally, or names a file |
| `git_object_unavailable` | a needed object is missing (for example in a partial clone) or unreadable |
| `git_tree_invalid` | a malformed tree, an unsupported mode, an invalid or duplicate name, or an object above its limit: tree 32 MiB, `.gitignore` 1 MiB, link 4096 bytes |


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
| unreadable snapshot, live observation failure, or Git source failure | `Error:` | 5 |
| internal invariant | `Error: internal error:` | 6 |

Exit 1 is a normal result, not an error: stderr stays empty. A diff that only
contains `MOVED` entries is exit 1.

## JSON output

`--format json` writes Diff Result Schema v3 to stdout. See
[diff-result-v3.md](../contracts/diff-result-v3.md). The document is UTF-8,
two-space indented, without HTML escaping, and ends with a newline.
`changes` is `[]` when empty, never `null`. Failure documents carry `status`
and a `diagnostic` object with `source` (`a` or `b` when the failing side is
known), `code`, and `message`. Usage failures stay exit 2 with a human stderr
diagnostic even under `--format json`.

A `MOVED` summarizes its subtree: the document lists how many records moved,
not which. Use `--no-moves` when every changed path is needed.

A Git source object adds `tree`, the observed tree id, and `commit` when the
revision resolved through a commit. Revision text, repository paths, remotes
and branch names are never emitted:

```json
"sources": {
  "a": { "kind": "git", "nodeCount": 9, "tree": "<40 hex>", "commit": "<40 hex>" },
  "b": { "kind": "snapshot", "nodeCount": 8 }
}
```

### From Diff Result Schema v2

`v0.4.0-a5` wrote [Diff Result Schema v2](../contracts/diff-result-v2.md),
which is no longer emitted. For `snapshot:` and `live:` sources, a v3
document is the v2 document with `"schemaVersion": 3`. Version 3 adds the
`git` source kind with `tree` and `commit`, the five `git_*` diagnostic
codes, and an evolution policy: from v3 on, consumers ignore unknown fields
and treat an unknown source kind or diagnostic code as opaque. A consumer
that checks `schemaVersion` fails on `3` instead of misreading the document.

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
of the same tree elsewhere, reports no differences. Between two revisions,
a file whose contents changed is no difference either. Symlink and junction
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

To compare the structure of a pull request with its base branch, both
revisions must already be in the local repository, because Dirloom never
fetches. With a shallow checkout, fetch the base first (for example
`fetch-depth: 0` in GitHub Actions):

```bash
dirloom diff git:origin/main git:HEAD --format json > diff.json
```

## Verify and diff

[verify](verify.md) answers whether the structure still matches, with a
fingerprint. Diff answers what changed, with a path list and moves. Both
observe a live side at most once, with the snapshot's Capture Semantics, and
neither reads current configuration.
