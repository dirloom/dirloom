# ADR 0006 — Git sources

> **Status:** proposed for `v0.4.0-a6`
>
> **Date:** 2026-10-29

## Context

`v0.4.0-a5` compares `snapshot:` and `live:` sources and reports moves. The v0.4 roadmap now asks the same engine to compare Git trees: `main` against a feature branch, `HEAD~10` against `HEAD`, a tag against a branch. The comparison core must not know whether it compares a filesystem, a snapshot, or a Git tree.

Four constraints shape the design:

- **One structural truth.** Every source produces a Canonical Structural Artifact. A4 forbade a second scanner, and Capture Semantics v1 (depth, dirs-only, hidden, ignore patterns, default ignores, `.gitignore`) is applied by one traversal in `internal/tree` through `internal/filter`. That traversal is currently tied to the operating-system filesystem (`os.ReadDir`, `os.Lstat`, `os.Readlink`, `os.ReadFile`).
- **Local first, no implicit network.** A missing ref or object is a diagnostic, never a fetch, pull, or clone. A partial clone fetches missing objects lazily on read unless prevented.
- **The recorded tree, not a substitute.** Git can substitute objects when it reads them (`refs/replace/`), so the same commit id can yield different trees from one repository to another.
- **Small dependencies.** Dirloom keeps few direct dependencies, each encapsulated (`docs/dependencies.md`).

## Decision

### Read Git objects through the `git` command, hardened

The Git source runs the user's `git` executable with plumbing commands only: `version`, `rev-parse --verify`, `rev-parse --show-object-format`, `cat-file -t`, and one long-lived `cat-file --batch`. No Go Git library is added. Every invocation:

- is built as an argument list, never through a shell;
- passes `-c protocol.allow=never` and sets `GIT_NO_LAZY_FETCH=1`, `GIT_NO_REPLACE_OBJECTS=1`, `GIT_TERMINAL_PROMPT=0` and `GIT_OPTIONAL_LOCKS=0`;
- runs no command that reads the index or the working tree, so no fsmonitor, hook, or filter runs;
- is bound to the request context; on cancellation or error, the `cat-file` process has its input closed, is killed if it does not exit, and is always waited for.

Git 2.45 or newer is required: `GIT_NO_LAZY_FETCH` and `git --no-lazy-fetch` appear in Git 2.45. A missing `git`, or one older than 2.45, is a diagnostic. On Git 2.56, a `--filter=blob:none` clone reported a missing object as `missing` and the object stayed absent. Without `GIT_NO_LAZY_FETCH`, `protocol.allow=never` still refused the transport. With `GIT_NO_REPLACE_OBJECTS=1`, a replaced commit yielded its recorded tree instead of the replacement's.

### One scanner, two backends, trees read on demand

`internal/tree.Scanner` reads through a small backend interface: list a directory, read a control file (`.gitignore`), read a link target. The filesystem backend keeps today's behavior byte for byte. The Git backend reads each tree object from `cat-file --batch` **only when the scanner enters that directory**, and reads `.gitignore` and link blobs the same way. A subtree that Capture Semantics excludes (depth, ignore rules, dirs-only) is never read, so it can neither cost time nor fail on a missing object. The Git source then reuses `source.FromTree`, Identity Projection v1, the comparison engine and Move Detection `structural-v1` unchanged. Capture Semantics is applied by the same code to a working tree and to a Git tree.

### Strict, bounded batch protocol

The reader checks every `cat-file --batch` response: the echoed object id must be the requested one, the type must be the expected one (`tree` or `blob`), the announced size must be within its limit, exactly that many bytes plus the terminating newline must follow, and `<oid> missing` is reported as missing. A truncated or malformed response is an error, never a partial read. Limits:

| Object | Limit |
| --- | --- |
| tree object | 32 MiB |
| `.gitignore` blob | 1 MiB |
| link target blob | 4096 bytes |

Tree entries are parsed strictly (`<mode> <name>\0<raw id>`, with the id length of the repository's object format).

### Source grammar `git:<revision>`

A Git source is `git:<revision>` in `dirloom diff`, split on the first colon like `snapshot:` and `live:`. The revision is anything `git rev-parse --verify` resolves locally: `HEAD`, a branch, a tag, a full or abbreviated commit id, `HEAD~10`, `main^`, `@{upstream}`, or `<commit>:<path>` for a subtree. The prefix removes any ambiguity between a ref and a file name. A revision that is empty, starts with `-`, contains a NUL or a newline, or names an index entry (`:<path>`, which `rev-parse` resolves by reading the index; `:/<text>` stays allowed) is a usage error. The object is peeled to a tree: a commit or tag yields its tree and records the commit, a tree is used directly, and any other object is an error. The repository is discovered from the current directory, as Git does: subdirectories, linked worktrees and bare repositories work.

### Committed-tree semantics and Capture Semantics

A Git source observes the committed tree, never the index or the working tree: untracked, ignored-but-unadded, and generated files are absent by construction.

| Pairing | Observation |
| --- | --- |
| `git:` against `git:` | Both trees in full, as committed: hidden entries included, no ignore rules, no depth limit, files included |
| `git:` against `snapshot:` (either order) | The Git tree is observed with the snapshot's Capture Semantics v1. With `useGitignore`, the `.gitignore` files contained in that tree apply, as for a checkout of it |
| `git:` against `live:` | Usage error (exit 2), like `live:` against `live:`: neither side carries Capture Semantics to apply |

Hidden means a name starting with `.`; Git trees carry no Windows hidden attribute.

### A structural diff, not `git diff`

A Git source carries no file contents into identity. A change to the contents of a regular file, with the same path and kind, is not a structural change: `dirloom diff git:main git:feature` is `NO_DIFFERENCES` when the branches differ only in file contents. The reference documentation states this with an example, and a test locks it.

### Equivalence with a checkout, and its limits

For the same Capture Semantics, observing `git:<rev>` and observing a clean checkout of `<rev>` produce the same artifact when the checkout materializes the tree without loss. This guarantee does **not** cover, and the contract lists:

- links checked out as plain files (`core.symlinks=false`, the Windows default);
- absolute link targets that point inside the checkout root: the filesystem source rewrites them root-relative, the Git source keeps them as recorded;
- initialized submodules: the filesystem sees their contents, the Git source an empty directory;
- names the platform cannot materialize (case-only collisions on a case-insensitive filesystem, names invalid on Windows);
- a `.gitignore` above the 1 MiB Git limit, which the filesystem source still reads;
- Windows hidden attributes, and files present in the checkout but not committed.

### Object mapping

| Git entry | Artifact node |
| --- | --- |
| tree `40000` | directory |
| blob `100644`, `100755`, legacy `100664` | file; the executable bit is not identity, as in Identity Projection v1 |
| blob `120000` | symlink; target = blob content, at most 4096 bytes of valid UTF-8, NFC only, never resolved. No separator or root rewrite: the same tree gives the same target on every platform |
| commit `160000` (submodule) | empty directory, never followed, as in a fresh clone |
| any other mode | `git_tree_invalid` |

Entry names go through the existing canonical path validation. A name that is empty, `.`, `..`, contains `/` or NUL, or NFC-collides with a sibling is rejected, never repaired.

### Diff Result Schema v3

Diff Result Schema v2 states that `sources.*.kind` "is `snapshot` or `filesystem`" and defines no extension rule, so a consumer that validates it would reject a `git` kind. Following the arbitration "keep v2 if its rules allow the extension, otherwise a new version", A6 introduces **Diff Result Schema v3**:

- identical to v2, plus `sources.*.kind: git`, plus `sources.*.tree` (observed tree id) and, when the revision went through a commit, `sources.*.commit`, plus the Git diagnostic codes;
- an explicit evolution policy: consumers ignore object fields they do not know and treat unknown `sources.*.kind` values and diagnostic codes as opaque; new source kinds, optional fields and diagnostic codes are additive within v3; a new operation, status, exit code, or a change in the meaning of an existing field requires a new version;
- v2 is frozen like v1. For `snapshot:` and `live:` sources, a v3 document equals the v2 document except for `schemaVersion`, and a test proves it against the frozen v2 goldens.

Revision expressions, repository paths, remotes and branch names are never emitted. Statuses and exit codes do not change: Git failures are `OBSERVATION_ERROR` (exit 5) with `git_unavailable`, `git_repository_not_found`, `git_revision_not_found`, `git_object_unavailable`, or `git_tree_invalid`.

### Scope

A6 exposes Git sources through `dirloom diff` only. `fingerprint`, `snapshot`, and `verify` are unchanged. A7 History reuses the Git source as a library.

## Alternatives considered

- **`go-git`.** No external binary, but dozens of transitive modules against a policy of few, encapsulated dependencies, and a second implementation of Git behavior to trust. Rejected.
- **Reading `.git` directly (loose objects, packfiles, deltas).** Full control, but a large, security-sensitive parser to maintain. Rejected for v1.
- **`git ls-tree -r -t` once per tree.** One call, but it reads every subtree before the scanner filters, so excluded subtrees can cost time or fail on missing objects. Rejected for on-demand tree reads through `cat-file --batch`.
- **`ls-tree` per directory.** On-demand, but one process per directory, which is too slow for large trees on Windows. Rejected.
- **A dedicated Git walker beside the filesystem scanner.** Faster to write, but a second application of Capture Semantics that would drift. Rejected: it is the second scanner A4 forbade.
- **Git 2.25 with `protocol.allow=never` only.** Wider reach (older LTS distributions), but no tested proof of the no-network guarantee on those versions. Deferred until a CI job runs an older Git.
- **Bare refs (`dirloom diff main feature`).** Shorter, but ambiguous with file names and inconsistent with the A4 grammar. Rejected.
- **`git:` against `live:` using Git's index.** Useful for comparing a working tree to `HEAD`, but it needs its own working-tree contract. Deferred.
- **Amending v2 in place.** No version bump, but it would change a published contract whose enumeration is closed. Rejected under the arbitration rule.

## Consequences

- New contracts: Git Tree Source v1 and Diff Result Schema v3. Frozen: Diff Result Schema v2. Unchanged: Fingerprint v1, Identity Projection v1, Canonical Identity Encoding v1, Snapshot Schema v1, Capture Semantics v1, Verify Result Schema v1, Move Detection `structural-v1`, and all exit codes.
- `git:` sources need `git` 2.45 or newer on `PATH`. Every other feature stays free of that requirement. Several common defaults are older (Debian 12: 2.39, Ubuntu 22.04: 2.34, Ubuntu 24.04: 2.43, Apple's command line tools: 2.39) and need a newer Git, such as the Git PPA or Homebrew, for `git:` sources. The diagnostic names the installed and required versions.
- `internal/tree` gains a backend interface. Every existing scanner, CLI and golden test must stay green, unchanged.
- Known limits are documented rather than worked around: the equivalence exceptions above, no working-tree or index source, no submodule recursion, LFS pointers seen as plain files, and deprecated grafts or shallow boundaries that can change what `HEAD~n` resolves to (never the tree of an explicit commit id).
