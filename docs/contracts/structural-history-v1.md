# Structural History v1

> **Status:** normative contract for Dirloom `v0.4.0-a7`
>
> **Related:** [Git Tree Source v1](git-tree-source-v1.md), [History Result Schema v1](history-result-v1.md), [Identity Projection v1](identity-projection-v1.md), [Move Detection structural-v1](move-detection-structural-v1.md), [ADR 0007](../adr/0007-structural-history.md)

Structural History lists how the structure of a Git repository evolved along the first-parent chain of a revision: one entry per commit, with the Fingerprint v1 of its whole committed tree and a count of the structural changes against its first parent. Every tree is observed as a Git Tree Source v1 artifact with no Capture Semantics. History reads Git objects only: never the index, the working tree or the network, and never a substituted object.

## Request

```text
dirloom history [<revision>] [--from <revision>] [--limit <n>] [--structural-only] [--no-moves] [--format text|json]
```

| Operand or flag | Meaning |
| --- | --- |
| `<revision>` | the newest commit of the history; default `HEAD` |
| `--from <revision>` | exclusive lower bound on the first-parent chain |
| `--limit <n>` | number of commits examined, an integer of at least 1; default 20 |
| `--structural-only` | output only entries with `structural: true` |
| `--no-moves` | summaries use move detection `none` instead of `structural-v1` |
| `--format` | `text` (default) or `json` ([History Result Schema v1](history-result-v1.md)) |

A revision follows the revision rules of [Git Tree Source v1](git-tree-source-v1.md#expression-and-repository), without the `git:` prefix:

| Revision | Result |
| --- | --- |
| empty, starts with `-`, contains NUL or a newline | usage error, exit 2; Git is not run |
| an index entry: starts with `:` but not `:/` | usage error, exit 2; Git is not run |
| anything else | resolved by Git; must name a commit, directly or through tags |

A `--limit` that is not an integer of at least 1 is a usage error. Capture Semantics flags (`--preset`, `--depth`, `--dirs-only`, `--hidden`, `--ignore`, `--no-default-ignore`, `--no-gitignore`, `--root`), configuration flags (`--config`, `--no-user-config`, `--no-config`), presentation flags (`--color`, `--icons`, `--theme`) and destinations (`--output`, `--copy`) are rejected with exit 2.

The repository is the one Git discovers from the process working directory, as in Git Tree Source v1.

## Git invocation

Every Git call is hardened exactly as in [Git Tree Source v1](git-tree-source-v1.md#git-invocation): `-c protocol.allow=never`, `GIT_NO_LAZY_FETCH=1`, `GIT_NO_REPLACE_OBJECTS=1`, `GIT_TERMINAL_PROMPT=0`, `GIT_OPTIONAL_LOCKS=0`, an argument list without a shell, and Git 2.45 or newer. History runs only these commands:

```text
git version
git rev-parse --show-object-format
git rev-parse --verify --quiet <revision>^{commit}
git rev-parse --git-path shallow
git cat-file --batch
```

One `cat-file --batch` process serves the whole run: every commit object and every tree object, with the batch protocol, the per-object limits and the process lifecycle of Git Tree Source v1. Commit objects are an additional object kind of that protocol, with the expected type `commit` and a limit of 1 MiB.

## Resolution

1. Version gate and object format, as in Git Tree Source v1.
2. `git rev-parse --verify --quiet <revision>^{commit}` gives the newest commit, `to`. A revision that does not resolve, or names a tree or a blob, is `git_revision_not_found`.
3. The same for `--from` when present.
4. The shallow boundary list is read once (below).

## Shallow boundary list

`git rev-parse --git-path shallow` names the file in which Git records shallow boundaries. A relative path is relative to the directory Git ran in; linked worktrees, bare repositories and subdirectories are handled by Git. History reads that file once, and it is the only file history reads directly:

- an absent file means the repository has no shallow boundary;
- otherwise it must be at most 16 MiB and contain one full object id per line, in the repository's object format, and nothing else;
- an unreadable, oversized or malformed file is `git_shallow_invalid`.

The list never changes the walk. It is used only to explain a missing parent.

## Walk

The walk starts at `to` and follows the **first parent** recorded in each raw commit object. `refs/replace/` is disabled; deprecated grafts and the shallow list do not rewrite parents. Each examined commit is listed, newest first. The walk ends at the first of:

| End | Condition |
| --- | --- |
| `from` | the next commit to examine is `--from`; it is not listed |
| `limit` | `--limit` commits have been examined |
| `root` | the last listed commit has no parent |
| `shallow` | the last listed commit is in the shallow boundary list **and** its first parent object is absent |

In detail, for each commit `C`, newest first:

1. If `C` is `--from` (only possible for the first commit, `to`), the walk ends at `from` with no entries.
2. `C`'s commit object is read and its tree observed. `C` is listed.
3. If `C` has no parent, its base is the empty tree; the walk ends at `root`.
4. If `C`'s first parent object is absent: when `C` is in the shallow boundary list, `C` has no base and the walk ends at `shallow`; otherwise the result is `git_object_unavailable`.
5. Otherwise the first parent's tree is observed and is `C`'s base.
6. If the first parent is `--from`, the walk ends at `from`. Otherwise, if `--limit` commits have been listed, the walk ends at `limit`.
7. The walk continues with the first parent.

When both `from` and `limit` apply to the same step, the end is `from`: the requested range is complete.

A repository that is shallow somewhere proves nothing about a given commit: a missing parent is a `shallow` end only for a commit the boundary list names, and any other missing object stays an error. A boundary commit whose first parent object is present is walked through.

`--from` is an exclusive bound, and an explicit limit takes precedence:

```text
A ← B ← C ← D (HEAD)

dirloom history D --from B             → D, C            end: from
dirloom history D --from B --limit 2   → D, C            end: from
dirloom history D --from B --limit 1   → D               end: limit
dirloom history D --from D             → no entries      end: from
dirloom history D --from X             → history_from_not_reached
```

When the walk ends at `root` or `shallow` and `--from` was given, `--from` was not on the first-parent chain: the result is `history_from_not_reached`, never a history presented as complete. An end at `limit` is not an error.

A merge commit is examined once; all its parents are recorded; its base is its first parent. Commits reachable only through second or later parents are not examined.

Each commit's tree is observed once: the base of one entry is the tree of the next. The run is **transactional**: the walk completes and every entry is validated before any output is written. A failure or a cancellation at any commit yields no history.

## Commit objects

A commit object is untrusted input, parsed strictly:

1. the first header is `tree <id>`;
2. zero or more `parent <id>` headers follow immediately;
3. other headers follow, up to the first empty line; exactly one of them is `committer <identity> <timestamp> <zone>`;
4. the message after the empty line is not read.

Ids must be full lowercase hex in the repository's object format. The timestamp is an unsigned decimal integer without a leading zero (except `0`) and at most 253402300799 (9999-12-31T23:59:59Z). The zone is `+HHMM` or `-HHMM`, with hours from 00 to 14 and minutes from 00 to 59. The timestamp is reported in UTC; the zone is validated, then dropped.

A second `tree` or `committer` header, a `parent` header after another header, a missing `tree` or `committer`, an invalid id, timestamp or zone, or a commit object above 1 MiB is `git_commit_invalid`. The identity and the message are skipped, never stored and never emitted.

## Entries

| Field | Value |
| --- | --- |
| commit | the commit id |
| tree | its tree id |
| parents | every recorded parent id, in order |
| committed at | the committer timestamp, UTC, RFC 3339 (`2026-10-30T19:40:09Z`) |
| fingerprint | Fingerprint v1 of the commit's tree, observed as a Git Tree Source v1 artifact with no Capture Semantics |
| node count | the Identity Projection v1 records of that artifact, root included |
| structural | whether the structure differs from the base |
| summary | `added`, `removed`, `changed`, `moved` and `total` of the comparison of the base (source A) with the commit (source B) |

The **base** is the first parent's tree; for a root commit it is the empty tree, whose artifact is the root alone. A commit at a `shallow` end has no base: its structural flag and summary are **absent**, which means unknown and never false.

The summary is computed by the comparison engine of [Diff Result Schema v3](diff-result-v3.md) under Move Detection `structural-v1`, or `none` with `--no-moves`. Its counts equal those of `dirloom diff git:<base> git:<commit>` with the same move detection, and `total` is their sum.

`structural` is true exactly when `total` is greater than zero, which holds exactly when the fingerprint differs from the base's fingerprint. A violation of that equivalence is `INTERNAL_ERROR`.

`--structural-only` keeps only entries with `structural: true`. It hides entries with `structural: false` and entries without a base, and it changes neither the range nor the number of commits examined.

## A structural history

No file contents enter identity. A commit that changes only file contents or only the executable bit has `structural: false` and the same fingerprint as its first parent; so does a commit with the same tree. Structural history is not `git log`: it reports when paths, kinds and link targets changed, and how many.

A history fingerprint is the Fingerprint v1 of the whole committed tree. It equals the fingerprint of a checkout only under a capture that keeps every committed entry and excludes `.git`, and only when the checkout is lossless ([Git Tree Source v1](git-tree-source-v1.md#equivalence-with-a-checkout)).

## Diagnostics

A failure is `OBSERVATION_ERROR` (exit 5) unless stated otherwise. The Git Tree Source v1 codes apply: `git_unavailable`, `git_repository_not_found`, `git_revision_not_found`, `git_object_unavailable`, `git_tree_invalid`, and `canonical_path_collision`. History adds:

| Code | When |
| --- | --- |
| `git_commit_invalid` | a malformed commit object (headers, ids, timestamp, zone), or one above 1 MiB |
| `git_shallow_invalid` | the shallow boundary list is unreadable, above 16 MiB, or holds a line that is not a full object id |
| `history_from_not_reached` | the walk ended at `root` or `shallow` without meeting `--from` |

Codes come from exit statuses, batch markers and the parsers, never from Git's error text. A broken invariant (summary sum, structural equivalence, ordering, ends) is `INTERNAL_ERROR` (exit 6). Usage errors are exit 2.

## Security

- Every commit header, id, timestamp, zone, tree entry, name and link target is checked before use; the shallow list is size-bounded and strictly parsed; no Git path is joined onto the local filesystem.
- The parents are those recorded in the commit objects: no replace refs, no grafts, no shallow rewriting. A shallow end is reported, never hidden, and never stands in for a missing or corrupt object.
- No network, no index, no working tree, no shell, as in Git Tree Source v1.
- Bounded: `--limit` caps the commits examined; one batch process per run; cancellation stops the walk and reaps the process.
- Output carries object ids, timestamps, fingerprints and counts only: never revision text, repository paths, remotes, branch names, identities or messages.

## Complexity

Let L be the number of commits examined, E the tree entries of one commit, and P the processes started.

| Step | Cost |
| --- | --- |
| resolution | a constant P: at most six processes per run, whatever L |
| commits | L + 1 commit objects read through the batch |
| trees | L + 1 tree observations, each linear in E (Git Tree Source v1) |
| summaries | L comparisons, each linear in the two record lists (Move Detection `structural-v1` included) |
| memory | two artifacts and their projections at a time, plus the L accumulated entries |

There is no persistent cache and no step that compares commits pairwise beyond each commit and its base.

## Reference outcomes

| Fixture | Expected |
| --- | --- |
| linear history with adds, removes and kind changes | one entry per commit; each summary equals `dirloom diff git:<parent> git:<commit>` |
| directory relocated in one commit | `moved: 1`; with `--no-moves`, the plain counts |
| contents-only commit, mode-only commit, commit with the same tree | `structural: false`, fingerprint equal to the parent's |
| `--structural-only` | those entries hidden; the range unchanged |
| root commit; root commit with an empty tree | summary against the empty tree; `structural: false` for the empty one |
| merge (two parents), octopus (three) | listed once with all parents; summary against the first parent; side-branch commits absent |
| `--limit 1`; `--limit` above the history length | `end: limit`; `end: root` |
| `A ← B ← C ← D`: `--from B`, `--from B --limit 2`, `--from B --limit 1`, `--from D`, `--from` off the chain | D, C and `end: from`; D, C and `end: from`; D and `end: limit`; no entries and `end: from`; `history_from_not_reached` |
| annotated tag, detached `HEAD`, abbreviated id | same as the commit id |
| tree revision (`HEAD:src`, tag to a tree), blob revision, unknown revision, unborn `HEAD` | `git_revision_not_found` |
| shallow clone (`--depth 2`) | `end: shallow`; the last entry has no structural flag and no summary; exit 0 |
| shallow clone, and a commit whose missing parent is not a recorded boundary | `git_object_unavailable`, not `end: shallow` |
| missing parent in a repository that is not shallow | `git_object_unavailable` |
| shallow list read from a linked worktree, a bare repository, a subdirectory; a malformed or oversized list | the same boundaries; `git_shallow_invalid` |
| `--structural-only` with a `shallow` end | the base-less entry hidden; `end: shallow` kept |
| failure at the third commit | one failure result, no entries, exit 5 |
| replace ref on a listed commit; graft file | the recorded parents and trees |
| crafted commit objects (no `tree`, `parent` after `author`, two `committer`, zone `+1500` or `+0060` or `0000`, timestamp with a sign, a leading zero or above 253402300799, above 1 MiB) | `git_commit_invalid` |
| partial clone `blob:none` with a link | `git_object_unavailable` at the first commit whose tree needs the blob |
| SHA-256 repository | 64-character ids |
| linked worktree, bare repository, subdirectory | the same results |
| cancellation during the walk | prompt return; the batch process reaped; no output |
| revision `-x`, `:path`, empty; `--limit 0`; a rejected flag | exit 2; Git not run |
| the same repository on Linux, Windows and macOS | byte-identical JSON |

## Versioning

This contract is Structural History v1. A change to the traversal, the ends, the base, the entry fields or their meaning, the commit parsing rules, the limits, or the Git commands requires a new version. Git Tree Source v1 is unchanged by this contract.
