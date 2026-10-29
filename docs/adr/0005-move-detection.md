# ADR 0005 — Move detection

> **Status:** accepted for `v0.4.0-a5`
>
> **Date:** 2026-10-29

## Context

`v0.4.0-a4` answers what changed between two sources with a closed `ADDED`/`REMOVED`/`CHANGED` vocabulary over Identity Projection v1 records. A relocation is reported path by path: moving `src/old/payment/` (4 nodes) to `src/features/payment/` yields 4 `REMOVED` plus 5 `ADDED` (the new intermediate `src/features` included). The diff is correct but does not say that one subtree moved.

Identity Projection v1 carries only canonical path, kind, and target. File contents are never read, and the v0.4 roadmap forbids making them mandatory for move detection v1. A directory carries structural evidence (the names and kinds of its descendants); a lone file carries only its name and kind.

Diff Result Schema v1 freezes the operation vocabulary as closed and the summary invariant as `total = added + removed + changed`. ADR 0004 left move detection to a later increment with its own contract.

## Decision

### A lossless relabelling pass over the A4 diff

Move detection is a pure post-pass over the two Identity Projection v1 record lists and the A4 merge result. It never observes a source, never reads contents, never alters a `CHANGED` entry, and never touches a path present in both sources. It only relabels sets of `REMOVED` and `ADDED` records as `MOVED`.

The relabelling is lossless **relative to the two projections**: given the A and B records, expanding every `MOVED` back into the `REMOVED` and `ADDED` records it covers yields exactly the A4 change list. A Diff Result v2 document alone is not reversible: a `MOVED` with `nodes: 1000` does not list the 1,000 absorbed paths and states. Consumers that need every path use `--no-moves`, whose change list is exactly the A4 change list.

### A structural correspondence, not proof of a move

A `MOVED` states that, under `structural-v1`, a removed node and an added node correspond deterministically and uniquely by structure. It is not historical evidence that the filesystem object was actually moved. Without contents or history, a deleted file and an unrelated new file with the same name, kind, and target are indistinguishable, even under global uniqueness. This matters most for `evidence: name`. The public contract states this limit explicitly.

### One `MOVED` operation, reported at the subtree root

The vocabulary becomes `ADDED`, `REMOVED`, `CHANGED`, `MOVED`. There is no `RENAMED`: a rename is a move whose parent is unchanged, and consumers can derive it from the two paths. A `MOVED` change names the destination as `path` and the origin as `from`, carries identical `before` and `after` node states, the number of records it covers (`nodes`, root included), and the `evidence` that justified it.

A `MOVED` covers an identical subtree: every descendant at the same relative path with the same kind and target. Descendants are absorbed and never emitted separately. A subtree that was moved and also structurally edited is not a `MOVED` at its root in `structural-v1`; its intact, uniquely identified parts can still be reported as `MOVED`, and the rest stays `ADDED`/`REMOVED`.

### Strategy `structural-v1`: exact shapes, global uniqueness, no scores

A node's shape is its kind, its target, and, recursively, the names and shapes of its children. The node's own name is excluded, so shape is rename-invariant. Shapes are interned exactly: a shape ID is assigned per distinct key made of the node state plus its children's (name, shape ID) pairs. The key never contains a full recursive subtree representation, and equality never rests on a hash alone, so a collision cannot produce a `MOVED`. Shape equality is never a similarity score or a percentage.

A removed node `p` and an added node `q` are paired only by one of two rules:

- **Relocation.** Same name and same shape, and that (name, shape) pair occurs exactly once in source A and exactly once in source B, counting every node of each tree, not only the changed ones.
- **Rename in place.** Same parent path, different name, `p` is a non-empty directory, and its shape occurs exactly once in source A and exactly once in source B.

Evidence is `subtree` when the moved node has descendants and `name` when it has none (file, symlink, junction, empty directory). Global uniqueness makes the detector reject generic, repeated names and shapes (`index.ts`, `config.yaml`, empty `build/` folders) instead of guessing. Any ambiguity leaves `REMOVED` + `ADDED`. Uniqueness makes pairing one-to-one and independent of iteration order.

`structural-v1` does not detect: a move combined with a rename, a file rename, a structurally edited moved subtree at its root, or a copy. Each stays `REMOVED` + `ADDED`.

### Diff Result Schema v2 replaces v1

Adding an operation breaks v1's closed vocabulary and summary invariant, so `dirloom diff --format json` emits Diff Result Schema v2 from `v0.4.0-a5` on: `schemaVersion: 2`, `metadata.moveDetection` (`structural-v1` or `none`), `summary.moved`, and `total = added + removed + changed + moved`. `metadata.comparisonVersion` stays `1` because path-anchored comparison semantics do not change. Statuses, exit codes, diagnostics, and stream rules do not change: a `MOVED`-only diff is `DIFFERENCES`, exit 1. v1 is no longer emitted. It was published only in the `v0.4.0-a4` prerelease, and the RC is the contract freeze point.

### Detection on by default, explicit opt-out

`dirloom diff` runs `structural-v1` by default. `--no-moves` disables it and emits a v2 document with `moveDetection: none` and no `MOVED` entries. Its `changes` array and its `added`/`removed`/`changed` counts are exactly A4's. The envelope differs from A4 (`schemaVersion: 2`, `moveDetection`, `summary.moved: 0`), and so does the human summary line (`0 moved`).

### Versioned strategy

Every rule above belongs to `structural-v1`. Changing a rule, a threshold, or an evidence class requires a new strategy name. For identical inputs, results under `structural-v1` must stay byte-identical across releases and platforms.

## Alternatives considered

- **Keep v1 and add MOVED behind an opt-in flag.** Keeps a prerelease contract alive but defers the breaking default flip to the RC. Rejected: two schemas from one command, and a later and larger break.
- **One `MOVED` per node.** Rejected: moving a 1,000-file directory would still produce 1,000 lines.
- **Similarity thresholds (for example, "80% of children match").** Rejected for v1: opaque percentages, arbitrary cut-offs, and false moves on partially rewritten trees.
- **Uniqueness among changed nodes only.** Detects more file moves, but pairs unrelated generic files (`a/__init__.py` deleted, `b/__init__.py` created). Rejected as not conservative.
- **Parent inference (promote `payment/` to `MOVED` because a uniquely identified child moved with it).** Would handle partial rewrites. Deferred to a later strategy because it introduces residual changes inside a `MOVED` subtree.
- **Content hashing.** Out of scope for v0.4 move detection v1. Any future content strategy must be explicitly optional.

## Consequences

- New contracts: Diff Result Schema v2 and Move Detection `structural-v1`. Diff Result Schema v1 is marked superseded.
- Unchanged: Fingerprint v1, Identity Projection v1, Canonical Identity Encoding v1, Snapshot Schema v1, Snapshot Artifact Projection v1, Capture Semantics v1, Verify Result Schema v1, and diff and verify exit codes.
- v1 consumers fail loudly on `schemaVersion: 2`. With `--no-moves`, they get the A4 change list in the v2 envelope.
- The v1 goldens are kept frozen as A4 regression references: `--no-moves` output is checked against their `changes` and counts.
- Detection is linear by construction: one pass to intern shapes, one to count keys, one to pair, and one to absorb, each over the two record lists. There is no pairwise comparison, so adversarial trees (thousands of identical generated folders) cannot make it quadratic. Benchmarks confirm this empirically but do not replace the argument.
- Known false negatives are accepted and documented, never compensated by guessing: repeated names or shapes, move with rename, file rename, partially edited moved subtrees, and copies.
