# `dirloom history`

History answers one question: along the history of a revision, when did the
structure change, and by how much?

It walks the first-parent chain of a Git revision and lists one entry per
commit: the Fingerprint v1 of its whole committed tree and the number of
added, removed, changed, and moved paths against its first parent. It reads
committed Git objects only, never the working tree, the index or the network.
It does not hash file contents, so it is not `git log`; see
[Not `git log`](#not-git-log).

## Syntax

```text
dirloom history [<revision>] [--from <revision>] [--limit <n>] [--structural-only] [--no-moves] [--format text|json]
```

```bash
dirloom history
dirloom history main --limit 50 --structural-only
dirloom history HEAD --from origin/main --format json
dirloom history v0.4.0-a6 --no-moves
```

| Operand or flag | Meaning |
| --- | --- |
| `<revision>` | the newest commit of the history; default `HEAD` |
| `--from <revision>` | stop before this commit: an exclusive bound on the first-parent chain |
| `--limit <n>` | number of commits examined, an integer of at least 1; default 20 |
| `--structural-only` | show only the commits that change the structure |
| `--no-moves` | count moves as `REMOVED` plus `ADDED` (move detection `none`) |
| `--format` | `text` (default) or `json` |

The repository is the one Git finds from the current directory: a
subdirectory, a linked worktree, or a bare repository, honoring `GIT_DIR`
and `GIT_WORK_TREE`. There is no directory operand.

A revision is what [`git:<revision>`](diff.md#git-sources) accepts in
`dirloom diff`, without the `git:` prefix, and it must name a commit,
directly or through tags: a branch, a tag, `HEAD~3`, a full or abbreviated
commit id, `:/<text>`. A tree (`HEAD:src`), a file, an unborn `HEAD`, or a
revision that does not resolve locally is `git_revision_not_found`. These
revisions are refused before Git runs, with exit 2: an empty revision, one
that starts with `-`, one that contains a NUL or a newline, and an index
entry (`:<path>`, `:<stage>:<path>`).

`git` 2.45 or newer must be on `PATH`. History runs `git version`,
`git rev-parse` and one `git cat-file --batch` process, at most eight
processes per run whatever the number of commits, without a shell. The
normative rules are in
[Structural History v1](../contracts/structural-history-v1.md).

## What is not accepted

History observes every committed tree whole, without Capture Semantics or
configuration. These flags are rejected with exit 2:

```text
--preset --depth --dirs-only --hidden --ignore
--no-default-ignore --no-gitignore --root
--config --no-user-config --no-config
--color --icons --theme
--output --copy
```

A `--limit` that is not an integer of at least 1, an empty `--from`, or more
than one revision is also a usage error.

### A command, not a directory

`history` is a command name. `dirloom history` runs this command; it no
longer inspects a directory named `history` in the current directory. To
inspect such a directory, give it a path:

```bash
dirloom ./history
```

## The walk

The walk starts at the revision and follows the **first parent** recorded in
each commit, newest first. It stops at the first of these ends, which the
output always names:

| End | Why the walk stopped |
| --- | --- |
| `limit` | `--limit` commits were examined |
| `from` | the next commit is `--from`, which is not listed |
| `root` | the last listed commit has no parent |
| `shallow` | the last listed commit is a shallow boundary of this clone |

`--from` is exclusive, and an explicit limit can stop the walk earlier:

```text
A ← B ← C ← D (HEAD)

dirloom history D --from B             → D, C            end: from
dirloom history D --from B --limit 2   → D, C            end: from
dirloom history D --from B --limit 1   → D               end: limit
dirloom history D --from D             → no commits      end: from
dirloom history D --from X             → history_from_not_reached
```

When the walk reaches a root commit or a shallow boundary without meeting
`--from`, that revision is not on the first-parent chain: the result is the
`history_from_not_reached` error, never a history presented as complete.
Stopping at `limit` is not an error.

A merge commit is listed once, with all its parents, and is compared with
its first parent. Commits reachable only through a merged branch are not
listed, as with `git log --first-parent`.

The parents are those recorded in the commit objects: `refs/replace/` and
grafts never change the walk.

## Entries

| Field | Meaning |
| --- | --- |
| commit, tree, parents | the commit id, its tree id, and every recorded parent |
| committed at | the committer date, in UTC |
| fingerprint | Fingerprint v1 of the whole committed tree |
| node count | records of that tree, root included |
| structural | whether the structure differs from the base |
| summary | added, removed, changed, and moved paths against the base, and their total |

The **base** of a commit is its first parent's tree; a root commit is
compared with the empty tree. For a commit with a parent, the summary has
the counts of `dirloom diff git:<commit>^ git:<commit>` under the same move
detection.
`structural` is true exactly when the total is above zero, which is exactly
when the fingerprint differs from the base's.

`--structural-only` hides the commits whose structure did not change. It
changes neither the range nor the number of commits examined.

Commit messages, author and committer identities, revision text, branch
names, remotes and repository paths are never shown.

### Not `git log`

Only paths, kinds and link targets count. A commit that changes file
contents only, or only the executable bit, or that keeps the same tree, is
`no structural change` and has its parent's fingerprint:

```text
7f8faa081feb  2026-03-01T11:00:00Z  no structural change
```

The history fingerprint of a commit equals `dirloom fingerprint` of a clean
checkout of it only under a capture that keeps every committed entry and
drops `.git`, and only when the checkout holds the tree without loss (see
[A revision against a snapshot of its checkout](diff.md#a-revision-against-a-snapshot-of-its-checkout)):

```bash
dirloom fingerprint --no-config --hidden --no-default-ignore --no-gitignore --ignore .git
```

### Shallow clones

In a shallow clone, such as a CI checkout with `fetch-depth: 1` or
`git clone --depth 2`, the walk stops at the boundary with `end: shallow`.
The last listed commit has no base: its structure against its parent is
unknown, never reported as unchanged, and `--structural-only` hides it.
History never fetches the missing commits. A missing parent that this clone
does not record as a shallow boundary is an error,
`git_object_unavailable`, not a shallow end.

## Text output

A history, exit 0, stderr empty:

```text
Structural History (first parent, 5 commits examined, end: root)

5c260114ef8e  2026-03-01T12:00:00Z  +2 -0 ~0 >0  merge (2 parents)
f7bdaa24db2a  2026-03-01T12:00:00Z  +0 -1 ~0 >1
7f8faa081feb  2026-03-01T11:00:00Z  no structural change
14fcbae4e7d8  2026-03-01T10:00:00Z  +2 -2 ~0 >0
e3a7a3a8d09c  2026-03-01T09:00:00Z  +8 -0 ~0 >0  root

Changed paths of a commit: dirloom diff git:<commit>^ git:<commit>
```

Each line shows the first 12 characters of the commit id, the committer date
in UTC, then `+added -removed ~changed >moved`, `no structural change`, or
`structure unknown (shallow boundary)`, and `root` or `merge (n parents)`
when it applies. With `--structural-only`, the header also counts the
commits shown (`5 commits examined, 4 shown`). A history with no commit to
show prints `No commits to show.` after the header. The output never
contains ANSI.

Other outcomes leave stdout empty and print one `Error:` line on stderr:

| Situation | stderr prefix | Exit |
| --- | --- | ---: |
| invalid arguments or revisions | `Error:` | 2 |
| Git, repository, object or walk failure | `Error:` | 5 |
| internal invariant | `Error: internal error:` | 6 |

There is no exit 1: a history is a report, not a check. The output is
transactional: the walk completes and every entry is validated before
anything is written, so a failure at any commit prints no partial history.

## JSON output

`--format json` writes History Result Schema v1 to stdout. See
[history-result-v1.md](../contracts/history-result-v1.md). The document is
UTF-8, two-space indented, without HTML escaping, and ends with a newline.
It carries `metadata` (the contracts followed, including `moveDetection`),
`range` (`to`, `from` when given, `limit`, `end`) and `entries`, newest
first, `[]` when empty:

```json
{
  "commit": "<40 hex>",
  "tree": "<40 hex>",
  "parents": ["<40 hex>"],
  "committedAt": "2026-03-01T12:00:00Z",
  "fingerprint": "dlm:v1:sha256:<64 hex>",
  "nodeCount": 10,
  "structural": true,
  "summary": { "added": 0, "removed": 1, "changed": 0, "moved": 1, "total": 2 }
}
```

Ids have 40 characters in a SHA-1 repository and 64 in a SHA-256 one. At a
`shallow` end, the last entry has neither `structural` nor `summary`. A
failure is one document with `status` and a `diagnostic` (`code`,
`message`), and no `range` or `entries`. Usage failures stay exit 2 with a
human stderr diagnostic even under `--format json`. Consumers ignore fields
they do not know and treat an unknown diagnostic code as opaque.

## Diagnostics

Every failure is `OBSERVATION_ERROR`, exit 5, unless it is an internal
invariant (exit 6). The [Git diagnostics](diff.md#git-diagnostics) of
`git:` sources apply (`git_unavailable`, `git_repository_not_found`,
`git_revision_not_found`, `git_object_unavailable`, `git_tree_invalid`),
plus:

| Code | Cause |
| --- | --- |
| `git_commit_invalid` | a malformed commit object (headers, ids, committer date or zone), or one above 1 MiB |
| `git_shallow_invalid` | the shallow boundary list of the repository is unreadable, above 16 MiB, or malformed |
| `history_from_not_reached` | the walk reached a root commit or a shallow boundary without meeting `--from` |

## CI

The contract is the process exit plus `--format json`. To list the commits
of a branch that changed the structure, the history and its base must
already be in the local repository, because Dirloom never fetches (for
example `fetch-depth: 0` in GitHub Actions):

```bash
base=$(git merge-base origin/main HEAD)
dirloom history HEAD --from "$base" --limit 200 --structural-only --format json > history.json
```

`--from` must be on the first-parent chain of the revision: use the merge
base, not the tip of a base branch that may have moved on. In a GitHub
Actions pull request, the default checkout is a merge commit whose first
parent is the base branch; walk the head of the pull request instead.

POSIX:

```bash
set +e
dirloom history --format json > history.json
code=$?
set -e

case "$code" in
  0) echo "history written" ;;
  2) echo "usage error" ;;
  5) echo "operational error" ;;
  6) echo "internal error" ;;
esac
```

PowerShell:

```powershell
& dirloom history --format json | Set-Content -Encoding utf8 history.json
$code = $LASTEXITCODE
```

`$LASTEXITCODE` uses the same numbers. GitLab CI, Azure DevOps, and Jenkins
can use this process contract directly: separate stdout and stderr, UTF-8, no
TTY, and no ANSI. If the branch has more commits than `--limit`, the walk
ends at `limit`, not `from`: check `range.end`.

## History and diff

History tells which commits changed the structure, and how much. To see the
paths a commit changed, compare it with its first parent:

```bash
dirloom diff git:<commit>^ git:<commit>
```

Both read the same committed trees, with the same comparison and move
detection, so the counts agree. A history examines each tree once, in one
Git session, and costs far less than one diff per commit; see the
[v0.4-a7 benchmarks](../benchmarks/v0.4-a7.md). There is no cache: running
it again reads the objects again.
