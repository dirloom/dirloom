# Structural Version Control

> **Status:** user guide for Dirloom v0.4 (`v0.4.0-rc1`). The contracts it links to are candidates for the v0.4 freeze; `watch` is experimental.

Dirloom v0.4 makes the structure of a project something you can record, check and compare, the way Git records and compares its contents. The structure is what Dirloom observes: the names, the kinds (directory, file, symlink, junction), the nesting and the link targets of the entries that survive the filters. **File contents are never read.** Editing a file changes nothing; adding, removing, renaming, moving or retargeting an entry does.

Six commands work on that structure:

| Command | Question it answers | Result | Machine output |
| --- | --- | --- | --- |
| [`dirloom fingerprint`](reference/fingerprint.md) | What is the identity of this structure? | `dlm:v1:sha256:<digest>` | [Fingerprint JSON v1](contracts/fingerprint-json-v1.md) |
| [`dirloom snapshot`](reference/snapshot.md) | How do I record it? | a self-verifying JSON file | [Snapshot Schema v1](contracts/snapshot-schema-v1.md) |
| [`dirloom verify`](reference/verify.md) | Does this directory still match the snapshot? | `MATCH` or `MISMATCH` | [Verify Result Schema v1](contracts/verify-result-v1.md) |
| [`dirloom diff`](reference/diff.md) | What changed between two structures? | `ADDED`, `REMOVED`, `CHANGED`, `MOVED` paths | [Diff Result Schema v3](contracts/diff-result-v3.md) |
| [`dirloom history`](reference/history.md) | Which commits changed the structure, and by how much? | one entry per commit | [History Result Schema v1](contracts/history-result-v1.md) |
| [`dirloom watch`](reference/watch.md) (**experimental**) | What is changing right now? | numbered generations of changes | [Watch Event Schema v1](contracts/watch-event-v1.md) |

`dirloom help structural` prints a summary of these commands in the terminal.

## Walkthrough

The commands below ran in a new Git repository holding `README.md`, `docs/guide.md`, `src/payment/service.go` and `src/payment/api/handler.go`. Every output is copied from that run, with fingerprints shortened to `<digest>`; commit ids and dates differ in another repository.

**Record the structure.** The snapshot file sits inside the tree it describes, so it is excluded from its own capture. `snapshot --output` already leaves out its destination; `--ignore` also records that exclusion in the snapshot, so that a Git tree that contains the committed file is observed the same way.

```console
$ dirloom snapshot --ignore architecture.dlm.json --output architecture.dlm.json
$ git add architecture.dlm.json && git commit -m "record the structure"
$ dirloom verify architecture.dlm.json
Structure matches snapshot.
```

**Change the structure.** On a branch `feature`, `src/payment` moves under `src/features` and `src/features/checkout.go` is added. `verify` now reports a mismatch, exit status 1:

```console
$ dirloom verify architecture.dlm.json
Structure differs from snapshot.
Expected: dlm:v1:sha256:<digest>
Actual:   dlm:v1:sha256:<digest>
```

**See what changed.** `diff` compares two sources. Here the snapshot and the live directory; the move of a whole subtree is one `MOVED` entry. Differences also exit 1:

```console
$ dirloom diff snapshot:architecture.dlm.json live:.
Structural Diff

Summary: 2 added, 0 removed, 0 changed, 1 moved, 3 total

ADDED
  + src/features
  + src/features/checkout.go

MOVED
  > src/payment -> src/features/payment (subtree, 4 nodes)
```

**Compare two branches.** A `git:` source reads the committed tree of a revision, without a checkout:

```console
$ dirloom diff git:main git:feature
```

It prints the same result. `--no-moves` lists every path instead: 6 added, 4 removed.

**Read the history.** `history` walks the first parents of `HEAD` and summarizes each commit as added, removed, changed and moved paths:

```console
$ dirloom history
Structural History (first parent, 3 commits examined, end: root)

33218feb53eb  2026-11-22T09:00:00Z  +2 -0 ~0 >1
a6192035fb34  2026-11-21T09:00:00Z  +1 -0 ~0 >0
090787944588  2026-11-20T09:00:00Z  +8 -0 ~0 >0  root

Changed paths of a commit: dirloom diff git:<commit>^ git:<commit>
```

The second commit counts one addition: `history` observes committed trees whole, so the snapshot file is part of them.

## Concepts

### The observed structure and Capture Semantics

`fingerprint`, `snapshot` and `watch` observe a directory through the same filters as the tree output: built-in exclusions, `--ignore`, `.gitignore`, hidden entries, `--depth`, `--dirs-only`, presets and configuration ([Filtering](../README.md#filtering)). The effective rules are the **Capture Semantics**. A snapshot stores them next to the structure.

`verify` and `diff` take no filter flag and read no configuration. They observe a live directory, or a `git:` side, with the Capture Semantics stored in the snapshot they compare it with, so the same rules apply to both sides. `history`, and `diff` between two `git:` sources, compare committed trees whole.

### Identity

A fingerprint is computed from the canonical structure ([Identity Projection v1](contracts/identity-projection-v1.md)): canonical paths in NFC, kinds and link targets. It does not depend on the absolute root, the platform, the time, the presentation flags or the file contents. The same observed tree gives the same fingerprint on Windows, Linux and macOS. A snapshot embeds the fingerprint of its own structure, and every reader recomputes it: a snapshot whose structure no longer matches its fingerprint is refused as invalid (exit 3), never compared.

### Sources

`diff` names each side with a prefix (`dirloom help sources`):

| Source | What it reads |
| --- | --- |
| `snapshot:<path>` | a Snapshot Schema v1 file |
| `live:<directory>` | a directory, observed now with the opposite snapshot's Capture Semantics |
| `git:<revision>` | the committed tree of a branch, tag, commit, `HEAD~n` or `<commit>:<path>` in the repository of the current directory |

`snapshot:` can face any source. `live:` cannot face `live:` or `git:`: neither side would supply the Capture Semantics. `verify` takes a plain snapshot path, and `history` a plain revision.

### Git sources

A `git:` source and `history` read committed Git objects only: never the working tree, the index or the network. They need Git 2.45 or newer on `PATH`. The comparison is structural, not `git diff`: a commit that only edits file contents is "no structural change". A submodule is an empty directory, and link targets are kept as Git records them. See [Git Tree Source v1](contracts/git-tree-source-v1.md) for what a Git tree and a checkout of it observe differently.

### Moves

Move detection, strategy `structural-v1`, reports a removed and an added entry as one `MOVED` when their name and structure occur exactly once in each source, or when a non-empty directory with a unique structure is renamed in place. Ambiguous cases stay `REMOVED` plus `ADDED`. A `MOVED` is a structural correspondence, not a proof that something was moved: Dirloom reads no content. `--no-moves` turns detection off in `diff`, `history` and `watch` ([Move Detection structural-v1](contracts/move-detection-structural-v1.md)).

### Watch (experimental)

`dirloom watch` observes a directory and reports each change of its structure as a numbered generation, with the same paths as `diff` between the two states. Native notifications decide when to observe, and a reconciliation every 60 seconds catches what they miss; `--poll <interval>` observes at a fixed interval instead. `--format ndjson` writes one JSON event per line. Structural Watch v1 and Watch Event Schema v1 are **experimental**: they may change in any v0.x release, and they are excluded from the v0.4 freeze. Read the [watch reference](reference/watch.md) for its platform limits before you depend on it.

## Scripts and CI

Programs branch on three things, which the [compatibility policy](compatibility-policy.md#2-what-is-stable-and-what-is-not) keeps stable: the exit code, the `status` of a JSON result and the machine `code` of a diagnostic. Messages, on stdout or stderr, are not stable.

| Exit | `verify` | `diff` | `history` |
| --- | --- | --- | --- |
| `0` | match | no differences | success |
| `1` | mismatch | differences | — |
| `2` | invalid usage | invalid usage | invalid usage |
| `3` | invalid snapshot | invalid snapshot | — |
| `4` | unsupported snapshot | unsupported snapshot | — |
| `5` | read, observation or write error | read, observation or write error | Git observation or write error |
| `6` | internal error | internal error | internal error |

`fingerprint` and `snapshot` keep the `0`/`1`/`2` codes of the tree output, and `watch` exits `0` when interrupted. [Exit codes and machine codes](reference/exit-codes.md) has the full table, with every machine code. [CI integration](ci-integration.md) puts these exit codes to work in a complete GitHub Actions workflow.

## Limits

Snapshots and Git trees are untrusted input, so their size is bounded:

- a snapshot is refused beyond 256 MiB, 2,000,000 nodes, 64 levels of JSON nesting, 4,096 ignore patterns or 64 required features (`snapshot_limit_exceeded`, exit 3), and `dirloom snapshot` refuses to write one beyond them ([Reader limits](contracts/snapshot-schema-v1.md#reader-limits));
- a Git observation stops beyond 2,000,000 nodes or 256 MiB of paths (`git_tree_too_large`, exit 5);
- a `.gitignore` above 1 MiB, or with a line of 64 KiB or more, fails the observation, in a directory as in a Git tree.

Structural commands refuse named pipes, sockets and devices (`unsupported_node_type`); exclude them with `--ignore`. A Windows junction is recorded as `junction` with its target and is never entered. On Windows, while Dirloom reads an entry, the directories above it cannot be renamed; renaming again succeeds ([platform limits](reference/watch.md#platform-limits)).

## Contracts and stability

Every output above has a written contract, golden fixtures and tests. The [contract register](contracts/README.md) lists them, with the command and flag that emits each one. They are **candidates for the v0.4 freeze**: the freeze takes effect at the release candidate's freeze commit, after which an incompatible change needs a new contract version ([compatibility policy](compatibility-policy.md)). Dirloom ships no JSON Schema file for v0.4: the Markdown contracts, the golden fixtures and the tests are the normative reference.

## Read next

- [CI integration](ci-integration.md): a snapshot committed to the repository, checked on every pull request.
- [Upgrading from v0.3.3](upgrading-to-v0.4.md): what changes when you move from the stable release.
- [Security policy](../SECURITY.md): what Dirloom reads, and how to verify a release.
- The architecture decisions behind each piece: [ADR 0001](adr/0001-structural-fingerprint-identity.md) to [ADR 0008](adr/0008-experimental-watch.md).
