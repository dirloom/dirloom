# ADR 0007 — Structural history

> **Status:** proposed for `v0.4.0-a7`, revised after the CTO framing review of 2026-10-30
>
> **Date:** 2026-10-30

## Context

`v0.4.0-a6` reads committed Git trees as structural sources, but only two at a time, through `dirloom diff`. The v0.4 internal roadmap (§10, "History Primitives") asks for the next step: observe how a structure evolves across commits, without building the full Time Machine yet. Its exit gate asks for a structural history model, a fingerprint per revision, concise change summaries, JSON output, documented merge semantics, a tested cache if one exists, and content-only commits that are distinguishable from structural ones.

Five constraints shape the design:

- **One structural truth.** A commit's structure must be the artifact `git:<commit>` already yields: the same scanner, the same Git backend, Identity Projection v1, Fingerprint v1, the comparison engine and Move Detection `structural-v1`. History adds a walk, not a second model.
- **The recorded history, not a rewritten one.** Git can rewrite what it shows as parents (`refs/replace/`, deprecated grafts, shallow boundaries). A structural history must follow what the commit objects record, and say where it stops.
- **Local, bounded, hardened.** Same guarantees as Git Tree Source v1: no network, no index or working tree, no substituted objects, strict parsing, per-object limits, cancellation that reaps the child process. The cost of a run must be bounded by the user's request.
- **Process starts are expensive.** The a6 benchmarks measured about 45 ms per `git` process on Windows. A walk cannot afford a resolution and a batch process per commit.
- **Privacy and determinism.** Results carry no host, user, path or run date; the same repository and arguments give the same bytes on every platform.

## Decision

### `dirloom history`, Git only

A new command:

```text
dirloom history [<revision>] [--from <revision>] [--limit <n>] [--structural-only] [--no-moves] [--format text|json]
```

- `<revision>` (default `HEAD`) must resolve to a commit, directly or through tags. It follows the `git:` revision rules of Git Tree Source v1 without the prefix: empty, leading `-`, NUL, newline and index entries (`:<path>`) are usage errors, and Git is not run.
- The repository is the one Git discovers from the current directory.
- History observes **whole committed trees with no Capture Semantics**, like `git:` against `git:`.

### First-parent walk over recorded parents

The walk starts at `<revision>` and follows the **first parent** of each commit. Parents are read from the raw commit objects through the same `cat-file --batch` process: `refs/replace/` is disabled as in A6, and neither deprecated grafts nor `.git/shallow` rewrite them. The walk stops at the first of:

| End | Meaning |
| --- | --- |
| `limit` | `--limit` commits were examined (default 20) |
| `from` | the next commit is `--from`, which is not listed |
| `root` | the last listed commit has no parent |
| `shallow` | the last listed commit is a recorded shallow boundary and its first parent is absent |

A shallow end requires both facts: the listed commit appears in the repository's shallow boundary list (the file Git names with `git rev-parse --git-path shallow`, read once per run), **and** its first parent object is absent. A repository being shallow proves nothing about a given commit. Any other missing parent, in a shallow repository or not, is an error (`git_object_unavailable`), never a silent end. A shallow boundary list that cannot be read or parsed is `git_shallow_invalid`.

`--from` is an **exclusive** lower bound. With `A ← B ← C ← D`, `dirloom history D --from B` lists D and C, then ends at `from`. An explicit limit that stops the walk earlier is not an error: `--limit 1` lists D and ends at `limit`. When both apply at the same step (`--limit 2`), the end is `from`, because the requested range is complete. Only when the walk ends at `root` or `shallow` without meeting `--from` is the result an error (`history_from_not_reached`): a partial range is never presented as complete. When `--from` equals the revision, the history is empty and ends at `from`. Merge commits are listed once, with all their parents recorded; full-graph traversal and parent selection are out of scope for v1.

### One history entry per commit

Each listed commit gives one entry:

| Field | Value |
| --- | --- |
| `commit` | the commit id |
| `tree` | its tree id |
| `parents` | every recorded parent id, in order |
| `committedAt` | the committer timestamp, in UTC (RFC 3339) |
| `fingerprint` | Fingerprint v1 of the commit's whole tree |
| `nodeCount` | Identity Projection v1 records, root included |
| `structural` | whether the structure differs from the base |
| `summary` | `added`, `removed`, `changed`, `moved`, `total` against the base |

The **base** is the first parent's tree; for a root commit it is the empty tree, so every path is `ADDED`. A commit at a `shallow` end has no base: its `structural` and `summary` are absent. Author and committer names and emails, and commit messages, are never read beyond the timestamp and never emitted.

`structural` is false exactly when the entry's fingerprint equals the base's. A contents-only or mode-only commit is therefore listed with `structural: false`, the case the exit gate calls "distinguishable". An entry with no base has no `structural` value at all: absent is "unknown", never `false`.

`--structural-only` keeps only entries with `structural: true`: it hides both `structural: false` entries and entries without a base, while `range.end: shallow` still reports that the history is incomplete. It changes `entries` only, never `range`, and does not change how many commits `--limit` examines.

Summaries are counts only. The detailed change list stays one command away: `dirloom diff git:<parent> git:<commit>`.

### One session, two artifacts in memory

A history run uses one Git session: one resolution of `<revision>` (and of `--from`), one shallow check, and **one `cat-file --batch` process** for every commit and tree object. Each commit's artifact is built once and compared with its parent's, so at most two artifacts and their projections are held at a time. The A6 Git source is refactored to observe a tree id through such a session; `dirloom diff` keeps its behavior and its output byte for byte.

### No persistent cache in v1

The roadmap allows a `revision → fingerprint` cache "if necessary". v1 has none: no file is written, nothing can be stale, and the gate's "cache correctness tested if cache exists" does not apply. The A7 benchmarks decide whether a later increment needs one. If it does, it will be keyed by tree id (content-addressed, so never invalidated), stored outside the repository, optional, and without any effect on results.

### History Result Schema v1

`dirloom history --format json` writes **History Result Schema v1**: `schemaVersion`, `status`, `metadata` (history, comparison and identity versions, traversal `first-parent`, move detection), `range` (resolved `to` commit, `from` commit if any, `limit`, `end`), `entries` (newest first) and, on failure, `diagnostic`. It adopts the Diff Result Schema v3 evolution policy from its first version. Revision text, repository paths, remotes and branch names are never emitted.

The output is transactional: the whole walk completes and validates before anything is written. A failure or a cancellation at any commit yields one failure document (`status` and `diagnostic`, no `entries`), never a partial history presented as a success.

Exit codes: `0` for a successful history, whatever it contains; `2` for usage errors (stderr, even under `--format json`); `5` for Git failures (`OBSERVATION_ERROR`, with the Git Tree Source v1 codes, plus `git_commit_invalid`, `git_shallow_invalid` and `history_from_not_reached`); `6` for broken invariants. There is no exit 1: a history is not a pass/fail check.

### Git invocation

History adds three command forms to the Git Tree Source v1 list, all hardened the same way: `git rev-parse --verify --quiet <revision>^{commit}`, `git rev-parse --git-path shallow`, and `commit` objects read through the existing `cat-file --batch`. The shallow boundary list is the only file history reads directly: Git metadata, never the index or the working tree; at most 16 MiB of object ids, one per line; an absent file means no boundary.

A commit object is parsed strictly and is limited to 1 MiB: a `tree` header, then `parent` headers, then exactly one `committer` header among the others. Its timestamp is an unsigned decimal without leading zeros, at most 253402300799 (9999-12-31T23:59:59Z), and its zone is `+HHMM` or `-HHMM` with hours 00–14 and minutes 00–59. The timestamp is emitted in UTC (RFC 3339); the zone is validated and dropped. Anything else is `git_commit_invalid`, never a panic.

## Alternatives considered

- **`git rev-parse --is-shallow-repository` to explain a missing parent.** One call, but it says only that the repository has shallow boundaries somewhere, so it could turn a missing or corrupt object into a normal-looking `shallow` end. Rejected after the framing review for the boundary list itself.
- **`git log` or `git rev-list` for the walk.** One process for the whole walk, but a second output format to parse, Git's own parent rewriting (grafts, shallow), and options that read the index or the working tree. Rejected for raw commit objects through the batch process already in use.
- **Full-graph traversal (all parents, topological order).** Complete, but the output is a DAG that a summary per entry cannot describe simply, and merge entries would need a choice of base. Deferred: first-parent is the readable default the roadmap asks for.
- **Summaries against every parent of a merge.** More information, but one merge would yield several summaries and the totals would no longer add up. Rejected for v1.
- **Structural-only by default.** Closer to "history of the structure", but content-only commits would vanish, and the exit gate asks to distinguish them. Kept as `--structural-only`.
- **A persistent fingerprint cache now.** Faster repeated runs, but files to place, size and clean, and a correctness risk before any measurement shows the need. Deferred.
- **Emitting the commit subject and author.** Friendlier text output, but personal data in a structural, privacy-neutral result. Rejected; ids link back to `git log`.
- **`git:<revision>` as the operand.** Consistent with `diff`, but history is Git-only, so the prefix adds nothing. Rejected.

## Consequences

- New contracts: Structural History v1 (walk, ends, entry model, summary semantics, Git commands, limits, diagnostics) and History Result Schema v1. Unchanged: Git Tree Source v1, Diff Result Schema v3, Fingerprint v1, Identity Projection v1, Canonical Identity Encoding v1, Snapshot Schema v1, Capture Semantics v1, Verify Result Schema v1, Move Detection `structural-v1`, and every existing command's output.
- `internal/gitsource` gains a session (one runner, one batch) and a commit parser; `source.Git` observes through it. `internal/comparison` gains a comparison of already-observed artifacts. Both refactors must leave every existing test and golden unchanged.
- A history fingerprint is the Fingerprint v1 of the whole committed tree. It equals `dirloom fingerprint` of a checkout only under a capture that keeps every committed entry and drops `.git`, and only for a lossless checkout; the docs say so.
- Cost grows with `--limit` times the tree size; there is no cache. Known limits are documented: first-parent only, recorded parents only (grafts and replace refs ignored), a shallow history ends at its boundary, no per-path history, no Capture Semantics.
- Framing review decisions (2026-10-30): Q1–Q6 as recommended; Go stays 1.26.10 (D0); `golang.org/x/text` moves to v0.41.0 or newer in a separate pull request to `main` before A7 (D1), and A7 starts from the `main` that includes it; the 20-commit, 10,000-entry, 2-second Windows figure is an initial performance target, not a contract.
