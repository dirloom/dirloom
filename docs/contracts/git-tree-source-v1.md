# Git Tree Source v1

> **Status:** normative contract for Dirloom `v0.4.0-a6`
>
> **Related:** [Canonical Structural Artifact v1](../canonical-structural-artifact-v1.md), [Identity Projection v1](identity-projection-v1.md), [Diff Result Schema v3](diff-result-v3.md), [ADR 0006](../adr/0006-git-sources.md)

A Git Tree Source turns one committed Git tree into a Canonical Structural Artifact, through the same scanner and Capture Semantics as a filesystem source. It reads Git objects only, never the index or the working tree, never the network, and never a substituted object.

## Expression and repository

A Git source is written `git:<revision>` and is split on the first colon, like `snapshot:<path>` and `live:<directory>`. `<revision>` is the text after that colon and may itself contain colons (`git:HEAD:src`).

| Revision | Result |
| --- | --- |
| empty, starts with `-`, contains NUL or a newline | usage error, exit 2; Git is not run |
| anything else | resolved by Git, below |

The repository is the one Git discovers from the process working directory, including from a subdirectory, a linked worktree, or a bare repository. `GIT_DIR` and `GIT_WORK_TREE` are honored as Git honors them.

## Git invocation

Dirloom runs the `git` executable found on `PATH` (never the current directory), with an argument list and no shell. Every invocation:

```text
git -c protocol.allow=never <command> …
GIT_NO_LAZY_FETCH=1
GIT_NO_REPLACE_OBJECTS=1
GIT_TERMINAL_PROMPT=0
GIT_OPTIONAL_LOCKS=0
```

The other environment variables are inherited. The only commands run are:

```text
git version
git rev-parse --verify --quiet <revision>
git rev-parse --show-object-format
git cat-file -t <oid>
git cat-file --batch
```

None of them reads the index or the working tree, so no fsmonitor, hook, or filter runs.

Git 2.45 or newer is required (`GIT_NO_LAZY_FETCH` and `git --no-lazy-fetch` appear in 2.45). `git version` is read once per process. A missing `git`, an unparsable version, or a version below 2.45 is `git_unavailable`, and the message names the installed and the required versions.

## Resolution

1. `git rev-parse --verify --quiet <revision>` returns an object id, or the revision is `git_revision_not_found`.
2. `git cat-file -t <oid>` decides:

| Object type | Observed tree | `commit` recorded |
| --- | --- | --- |
| `commit` | `<oid>^{tree}` | `<oid>` |
| `tag` pointing (possibly through tags) to a commit | that commit's tree | the commit |
| `tag` pointing to a tree | that tree | none |
| `tree` (for example `HEAD:src`) | `<oid>` | none |
| `blob`, or a tag pointing to a blob | — | `git_revision_not_found` |

`git rev-parse --show-object-format` gives the object format: `sha1` uses 20-byte ids (40 hex), `sha256` uses 32-byte ids (64 hex). Ids are reported in lowercase hex.

Because `GIT_NO_REPLACE_OBJECTS=1` is set, `refs/replace/` never substitutes an object: a commit id always yields its recorded tree. Deprecated `.git/info/grafts` and shallow boundaries are not overridden; they can change what an ancestry expression such as `HEAD~3` resolves to, never the tree of an explicit commit id.

## Reading trees on demand

One `git cat-file --batch` process serves one observation. The scanner asks for a directory only when it enters it; the Git backend then reads that directory's tree object. A subtree that Capture Semantics excludes (depth, explicit or default ignores, `.gitignore`, dirs-only for files) is never read: it costs nothing and cannot fail, even if its objects are missing.

### Batch protocol

A request is one object id followed by a newline. Each response is checked in order:

1. `<oid> missing` → `git_object_unavailable`.
2. Otherwise the header must be `<oid> <type> <size>` followed by a newline, with `<oid>` equal to the requested id; anything else → `git_tree_invalid`.
3. `<type>` must be the expected one: `tree` for a directory, `blob` for a `.gitignore` or a link target; otherwise → `git_tree_invalid`.
4. `<size>` must be a decimal integer within the limit of that object; otherwise → `git_tree_invalid`, naming the limit.
5. Exactly `<size>` bytes followed by a newline must follow. End of input or any other byte instead → `git_object_unavailable` (truncated response).

| Object | Limit |
| --- | --- |
| tree object | 32 MiB |
| `.gitignore` blob | 1 MiB |
| link target blob | 4096 bytes |

### Process lifecycle

The batch process is bound to the request context. When the observation ends, fails, or is cancelled, Dirloom closes its input, kills it if it has not exited after a short grace period, and always waits for it. No process or goroutine outlives the observation.

### Tree entries

A tree object is a sequence of entries `<mode> SP <name> NUL <raw id>`, where the id length follows the object format. A malformed entry, a mode not listed below, or two entries with the same name in one tree → `git_tree_invalid`.

## Object mapping

| Git mode | Git type | Artifact node |
| --- | --- | --- |
| `40000` | tree | `directory` |
| `100644`, `100755`, `100664` | blob | `file`. The executable bit is not identity (Identity Projection v1) |
| `120000` | blob | `symlink`; target = blob content |
| `160000` | commit (submodule) | `directory` with no children; never entered |
| any other | — | `git_tree_invalid` |

### Names

Each entry name goes through the canonical validation of Canonical Structural Artifact v1: valid UTF-8, NFC, not empty, not `.` or `..`, no `/`, no NUL. An invalid name is `git_tree_invalid`. Two sibling names that NFC to the same canonical name are `canonical_path_collision`. Names are never repaired.

### Link targets

A link target is the blob content: valid UTF-8, normalized to NFC, at most 4096 bytes, and never resolved. No separator is rewritten and no absolute target is rewritten relative to a root, so the same tree yields the same target on every platform. A target that is not valid UTF-8 is `git_tree_invalid`.

## Capture Semantics

A Git source observes the committed tree: untracked, ignored-but-unadded and generated files are absent by construction.

| Pairing in `dirloom diff` | Capture applied to the Git side |
| --- | --- |
| `git:` against `git:` | none: unlimited depth, files and directories, hidden entries included, no default ignores, no explicit patterns, no `.gitignore` |
| `git:` against `snapshot:`, either order | the snapshot's Capture Semantics v1 |
| `git:` against `live:`, either order | usage error, exit 2: neither side carries Capture Semantics |

With a snapshot's Capture Semantics:

- depth, dirs-only, default ignores and explicit patterns behave as for a filesystem source, through the same scanner code;
- hidden means a name that starts with `.`; Git trees carry no Windows hidden attribute;
- with `useGitignore`, each `.gitignore` blob found in an entered tree applies at its tree path. A `.gitignore` entry that is a link or a submodule is not read, as for a working tree;
- a `.gitignore` blob above 1 MiB is `git_tree_invalid`.

## Equivalence with a checkout

With the same Capture Semantics, observing `git:<rev>` and observing a clean checkout of `<rev>` yield the same artifact **when the checkout materializes the tree without loss**. The guarantee does not cover:

| Exception | Filesystem source | Git source |
| --- | --- | --- |
| links checked out as files (`core.symlinks=false`, the Windows default) | `file` | `symlink` |
| absolute link target pointing inside the checkout root | rewritten relative to the root | kept as recorded |
| initialized submodule | its contents | empty directory |
| names the platform cannot materialize (case-only collisions on a case-insensitive filesystem, names invalid on Windows) | absent or merged | present |
| `.gitignore` above 1 MiB | read | `git_tree_invalid` |
| Windows hidden attribute | can hide an entry | no effect |
| files present in the checkout but not committed | present | absent |

## A structural source

No file contents enter identity. A file whose contents differ between two revisions, with the same path and kind, produces no change:

```text
main:     src/app.go  (contents A)
feature:  src/app.go  (contents B)

dirloom diff git:main git:feature   →   No structural differences.  (exit 0)
```

`dirloom diff` on Git sources is not `git diff`: it reports added, removed, changed and moved paths, kinds and link targets.

## Diagnostics

All Git failures are `OBSERVATION_ERROR` (exit 5) for the side that failed. Codes come from exit statuses, `--quiet` behavior and batch markers, never from parsing Git's error text. A message may quote one sanitized line of Git's standard error.

| Code | When |
| --- | --- |
| `git_unavailable` | no `git` on `PATH`, unparsable version, or version below 2.45 |
| `git_repository_not_found` | the working directory is not inside a repository, or Git refuses it (for example `safe.directory`) |
| `git_revision_not_found` | the revision does not resolve locally, or resolves to a blob |
| `git_object_unavailable` | a needed object is missing (including objects a partial clone does not have), unreadable, or its batch response is truncated |
| `git_tree_invalid` | malformed entry, unsupported mode, duplicate name, invalid name, unexpected object type, malformed header, size above a limit, or invalid link target |

`canonical_path_collision` keeps its existing meaning. A broken internal invariant is `INTERNAL_ERROR` (exit 6).

## Security

- Git data is untrusted: every header, size, entry, name and target is checked before use, and no Git path is ever joined onto the local filesystem.
- No network: `protocol.allow=never` and `GIT_NO_LAZY_FETCH=1`. A missing object in a partial clone stays missing.
- No substitution: `GIT_NO_REPLACE_OBJECTS=1`.
- No code execution: only the commands listed above; no index read, hook, fsmonitor or filter; no shell; the executable is never taken from the current directory.
- No option injection: a revision starting with `-` is refused before Git runs.
- Bounded: per-object limits, on-demand reads, cancellation that stops and reaps the child process.
- Output carries object ids only, never revision text, repository paths, remotes or branch names.

## Complexity

Let E be the number of tree entries in the directories the scanner enters, and B the bytes of the tree objects and blobs it reads.

| Step | Cost |
| --- | --- |
| resolution | a constant number of Git calls |
| tree reads | each entered tree object read once through the batch process: O(B) |
| entry parsing | one pass per tree object: O(E) |
| scanning and artifact | the existing scanner and `FromTree`: linear in observed nodes |

Excluded subtrees cost nothing. There is no step that compares entries pairwise.

## Reference outcomes

| Fixture | Expected |
| --- | --- |
| branch vs branch, tag vs branch, commit vs commit | correct `ADDED` / `REMOVED` / `CHANGED` |
| `HEAD~n` vs `HEAD`, detached `HEAD` | same result as explicit commit ids |
| directory relocated between commits | one `MOVED` (Move Detection `structural-v1`) |
| file contents changed only | `NO_DIFFERENCES` |
| executable bit flipped only | `NO_DIFFERENCES` |
| symlink added, retargeted, relocated | `ADDED` / `CHANGED` / `MOVED` |
| absolute link target | kept as recorded on every platform |
| submodule added | one empty `directory`, not entered |
| directory deleted; empty tree | `REMOVED` subtree / every path `ADDED` |
| `<commit>:<path>` | that subtree; no `commit` reported |
| commit replaced through `refs/replace/` | the recorded tree |
| linked worktree, bare repository, subdirectory | same results |
| unknown revision; revision `-x`; empty revision | exit 5 `git_revision_not_found`; exit 2; exit 2 |
| outside a repository; no `git`; `git` reporting 2.44 | exit 5 `git_repository_not_found`; exit 5 `git_unavailable`; exit 5 `git_unavailable` |
| partial clone, needed blob missing | exit 5 `git_object_unavailable`; the object is still missing afterwards |
| subtree object missing but excluded by the snapshot's Capture Semantics | observation succeeds |
| same subtree not excluded | exit 5 `git_object_unavailable` |
| `.gitignore` above 1 MiB, link above 4096 bytes, tree above 32 MiB | exit 5 `git_tree_invalid` naming the limit |
| truncated or mismatched batch response | controlled error, no hang |
| cancellation during observation | prompt return; `cat-file` reaped |
| crafted tree with `..`, unknown mode, duplicate name; NFC collision | exit 5 `git_tree_invalid`; `canonical_path_collision` |
| names sorting before `.`, interleaving names, NFD names | identical artifact on every platform |
| `git:<rev>` vs snapshot of a lossless checkout, same Capture Semantics | `NO_DIFFERENCES` |
| each equivalence exception | the documented difference |

## Versioning

This contract is Git Tree Source v1. A change to the resolution rules, the object mapping, the limits, the Capture Semantics pairing, or the equivalence rule requires a new version. Raising the minimum Git version is a change of this contract; lowering it requires a CI job proving every guarantee above on the lower version.
