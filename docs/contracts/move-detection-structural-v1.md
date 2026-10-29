# Move Detection structural-v1

> **Status:** normative contract for Dirloom `v0.4.0-a5`
>
> **Strategy name:** `structural-v1` (emitted as `metadata.moveDetection`)
>
> **Related:** [Diff Result Schema v2](diff-result-v2.md), [Identity Projection v1](identity-projection-v1.md), [ADR 0005](../adr/0005-move-detection.md)

`structural-v1` turns some sets of `REMOVED` and `ADDED` records of a structural diff into `MOVED` changes. It is deterministic, explainable, conservative, and versioned. It reads no file contents, computes no similarity score, and leaves `REMOVED` + `ADDED` whenever a correspondence is not unique.

## A correspondence, not proof of a move

A `MOVED` states that, under the rules below, one removed node and one added node correspond deterministically and uniquely by structure. It is **not** historical evidence that the filesystem object was actually moved.

Without contents or history, structurally identical objects are indistinguishable. If `docs/old/guide.md` is deleted and an unrelated `docs/guide.md` is created in the same comparison, and `guide.md` appears nowhere else in either tree, `structural-v1` reports `MOVED docs/old/guide.md -> docs/guide.md` with `evidence: name`. Global uniqueness makes such coincidences rarer; it cannot rule them out. `evidence: name` is the weakest class and consumers should present it accordingly. `evidence: subtree` rests on the names and kinds of every descendant and is stronger, but it is still a structural correspondence.

## Inputs

- the Identity Projection v1 records of source A and of source B, each sorted by canonical path as UTF-8 bytes;
- the path-anchored comparison of those records (Diff Result comparison version 1), which classifies every path as unchanged, `ADDED`, `REMOVED`, or `CHANGED`.

Detection never observes a source, never reads configuration, and never reads file or link-target contents.

Identity records are prefix-closed: every non-root record's parent is a record of the same side. Hence if `p` is `REMOVED`, every descendant of `p` in A is `REMOVED`; if `q` is `ADDED`, every descendant of `q` in B is `ADDED`. The root `.` exists on both sides and is never a candidate.

## Definitions

For each node `n` of a tree T (A or B):

| Term | Definition |
| --- | --- |
| `name(n)` | last segment of the canonical NFC path (`Path.Base()`) |
| `parent(n)` | canonical parent path (`Path.Parent()`) |
| `state(n)` | kind, plus target for target-carrying kinds, with explicit presence |
| `children(n)` | records whose parent is `n`, ordered by name as UTF-8 bytes |
| `shape(n)` | interned identifier of `(state(n), [(name(c), shape(c)) for c in children(n)])` |
| `size(n)` | `1 + Σ size(c)` over `children(n)` |
| `key(n)` | `(name(n), shape(n))` |

`shape(n)` excludes the node's own name, so it is rename-invariant. It includes, recursively, the names, kinds, and targets of all descendants.

Counts are taken over **all non-root nodes** of each tree, not only changed nodes:

| Count | Meaning |
| --- | --- |
| `countA.key(k)`, `countB.key(k)` | number of nodes of A, of B, whose key is `k` |
| `countA.shape(s)`, `countB.shape(s)` | number of nodes of A, of B, whose shape is `s` |

Candidates: `CA` is the set of `REMOVED` paths, `CB` the set of `ADDED` paths. `CHANGED` and unchanged paths are never candidates.

### Exact interning

Shape identifiers come from one intern table shared by A and B, so identifiers are comparable across sides. They are internal and never emitted.

The interning key is an unambiguous, length-prefixed encoding of the kind code, the target when the kind carries one, and, for each child in order, its name and its already assigned shape identifier. It holds child identifiers, never a child's subtree representation, so it costs O(children + child-name bytes) per node. A hash may bucket keys, but two keys are equal only if their full encodings are equal: a hash collision can never merge two shapes.

A non-root parent always sorts before its children (`p` < `p/c`), so shapes are computed in a single pass over the records in reverse order, without recursion. The root `.` does not necessarily sort first (`-a` < `.`); it never moves, has no shape, and is skipped by that pass.

## Pairing rules

A candidate pair `(p, q)` with `p ∈ CA` and `q ∈ CB` is accepted if and only if one of these rules holds:

**R1 — Relocation.**

- `key(p) = key(q)`;
- `countA.key(key(p)) = 1` and `countB.key(key(p)) = 1`.

`parent(p) ≠ parent(q)` follows: equal parents and names would make `p` and `q` the same path, present on both sides.

**R2 — Rename in place.**

- `parent(p) = parent(q)`, which is then a path present in both A and B;
- `name(p) ≠ name(q)`;
- `p` is a directory with at least one child;
- `shape(p) = shape(q)`;
- `countA.shape(shape(p)) = 1` and `countB.shape(shape(p)) = 1`.

No other rule exists. In particular, there is no pairing by similarity, by partial subtree overlap, or by parent inference.

### Exclusivity and one-to-one pairing

1. **R1 and R2 never pair the same node twice.** Suppose `p` pairs with `q1` by R1 and with `q2` by R2. R2 makes `shape(p)` unique in B; `q1` has that shape (equal keys imply equal shapes), so `q1 = q2`. R1 requires `name(q1) = name(p)` and R2 requires `name(q2) ≠ name(p)`: contradiction. The same argument applies to `q`.
2. **Pairing is one-to-one.** Under R1, `p` is the only A node with its key and `q` the only B node with it. Under R2, the same holds for shapes. If `p1` pairs with `q` by R1 and `p2` by R2, then `shape(p1) = shape(q) = shape(p2)` and R2's uniqueness in A gives `p1 = p2`, which point 1 excludes.
3. **The result does not depend on evaluation order.** Each rule is a property of the candidate pair and the global counts, not of previously accepted pairs.

## Absorption and emission

Let `(p, q)` be an accepted pair. Since `shape(p) = shape(q)`, for every relative path `r`, `p/r` exists in A if and only if `q/r` exists in B, with the same state and shape.

**Nested pairs are at corresponding positions.** Let `p' = p/r` (with `r` non-empty) be paired with `q'`.

- By R1, `key(q/r) = key(p/r)` and uniqueness in B give `q' = q/r`.
- By R2, `parent(p')` is `p` or one of its descendants, so its path is in A only (prefix closure). R2 needs that path to be `parent(q')`, which is in B: impossible.

The same holds for descendants of `q`. Absorption by the topmost pair is therefore consistent.

**Root pairs.** An accepted pair `(p, q)` is a root pair if and only if no proper ancestor of `p` is the A side of an accepted pair. Processing accepted pairs in A record order (a non-root ancestor sorts before its descendants, and the root is never paired) and skipping pairs whose `p` is already absorbed computes exactly this set.

For each root pair, emit one change:

| Field | Value |
| --- | --- |
| `path` | `q` |
| `operation` | `MOVED` |
| `from` | `p` |
| `before`, `after` | `state(p)`, which equals `state(q)` |
| `evidence` | `subtree` if `p` has at least one child, else `name` |
| `nodes` | `size(p)`, which equals `size(q)` |

Every record of the A subtree of `p` is absorbed and removed from the `REMOVED` set; every record of the B subtree of `q` is absorbed and removed from the `ADDED` set. `CHANGED` entries are never modified. All changes are then sorted by `path` as UTF-8 bytes.

### Evidence classes

| `evidence` | Produced by | Meaning |
| --- | --- | --- |
| `subtree` | R1 on a non-empty directory, or R2 | identical structure of at least one descendant, unique in each tree |
| `name` | R1 on a file, symlink, junction, or empty directory | identical name and state, unique in each tree; no descendant to compare |

Evidence is a closed vocabulary. It is not a confidence score.

## Invariants

| ID | Invariant |
| --- | --- |
| I1 | Lossless relative to the projections: given the A and B records, replacing each `MOVED` by `REMOVED` for every record of the A subtree of `from` and `ADDED` for every record of the B subtree of `path` yields exactly the change list of detection `none`. The emitted document alone does not list absorbed descendants. |
| I2 | Each A path appears at most once across `REMOVED` paths and `MOVED` `from`; each B path at most once across `ADDED`, `CHANGED`, and `MOVED` paths. |
| I3 | No `REMOVED` path or `MOVED` `from` lies below another `MOVED` `from`; no `ADDED` path or `MOVED` `path` lies below another `MOVED` `path`; `from` and `path` of one move are neither equal nor ancestor and descendant. |
| I4 | Symmetry: the diff of (B, A) equals the inverse of the diff of (A, B), where the inverse swaps `ADDED` and `REMOVED`, `before` and `after`, and a `MOVED`'s `path` and `from`. |
| I5 | Determinism: identical inputs give byte-identical text and JSON across runs, map iteration orders, and platforms. |
| I6 | With detection `none`, there is no `MOVED`, and the change list equals the path-anchored comparison. |
| I7 | `total = added + removed + changed + moved = len(changes)`. |
| I8 | For every `MOVED`: `from ≠ path`, `before = after`, `nodes ≥ 1`, `evidence = name ⇔ nodes = 1`, and `evidence = subtree ⇒ kind = directory`. |
| I9 | The root `.` is never a `path` or a `from`. |

A violation of I2, I3, or I6 through I9 detected at run time is an internal error (`INTERNAL_ERROR`, exit 6).

## Complexity

Let N and M be the record counts of A and B, and L the total name bytes.

| Step | Cost |
| --- | --- |
| tree index: one map insertion and one parent lookup per record | O(N+M) |
| shapes: one reverse pass; each node's key holds its children's identifiers, and every node is a child exactly once | O(N+M+L) |
| counts: one increment per node for `key` and one for `shape` | O(N+M) |
| pairing: one lookup per candidate in `CA` | O(N) |
| absorption: each record is marked at most once, because root pairs cover disjoint subtrees | O(N+M) |
| validation: ancestor walks with set lookups | linear in total canonical path bytes |

There is no candidate-by-candidate comparison: repeated keys and shapes are rejected by counting. Adversarial inputs (thousands of identical generated folders, deep chains, wide fan-out) cannot make detection quadratic. Benchmarks confirm this empirically; they do not replace this argument.

## Not detected

Each case stays `REMOVED` + `ADDED` under `structural-v1`:

- a name, or a (name, shape) pair, repeated anywhere in either tree, including in unchanged locations (`index.ts`, `config.yaml`, empty `build/` folders);
- a shape repeated anywhere in either tree, for a rename in place;
- a move combined with a rename (different parent and different name);
- a file, symlink, junction, or empty directory renamed in place, case-only file renames included;
- a moved subtree whose structure was also edited (a descendant added, removed, renamed, or changed in kind or target): no `MOVED` at its root, although its intact and uniquely identified parts can still be `MOVED`;
- a copy (one removed node and several added nodes with the same key), or any many-to-one correspondence;
- a path reused in place: if a new node is created at the old path, the old path is not `REMOVED` and cannot be a candidate.

These are accepted false negatives, never compensated by guessing. A later strategy may relax them under a new name.

## Reference outcomes

| Fixture | Expected |
| --- | --- |
| Large subtree moved intact | 1 `MOVED` (`subtree`, `nodes = N`); new intermediate directories `ADDED` |
| Same-named empty folder repeated (`a/build/`, `b/build/` in A; `a/build/` relocated to `c/build/`, `b/build/` unchanged) | no `MOVED`: key `(build, empty directory)` occurs twice in A and twice in B |
| Two empty folders with distinct names (`old/cache/`, `old/tmp/`) relocated to `new/` while `old/` stays | 2 `MOVED` (`name`, 1 node each); uniqueness is per (name, shape), not per shape |
| Repeated module shapes, one renamed in place | no `MOVED` (shape not unique) |
| Renamed sibling directories, distinct shapes | 2 `MOVED` (R2) |
| Renamed sibling directories, identical shapes | no `MOVED` |
| Partial subtree rewrite (file added inside a moved directory) | no `MOVED` at the root; uniquely identified intact children `MOVED`; residual `ADDED`/`REMOVED` |
| Same basename in several places, one moved | no `MOVED` with `name` evidence |
| Many generated folders (1,000 or more identical) | no `MOVED`; linear time |
| Ambiguous candidates or copy (1 removed, 2 added with the same key) | no `MOVED` |
| Lone file with a unique name relocated | 1 `MOVED` (`name`, 1 node) |
| File renamed in place | `REMOVED` + `ADDED` |
| Directory moved and renamed | no `MOVED` at its root |
| Swap of two uniquely named files between directories | 2 `MOVED` |
| Symlink relocated, same target / different target | `MOVED` / `REMOVED` + `ADDED` |
| Kind change at the same path | `CHANGED`, never `MOVED` |
| Case-only directory rename / file rename | `MOVED` (R2) / `REMOVED` + `ADDED` |
| NFC and NFD spellings of the same names | identical results after canonicalization |

## Versioning

`structural-v1` is a frozen strategy. For identical Identity Projection v1 inputs, its output must stay byte-identical across releases and platforms. Changing a rule, adding a rule, adding an evidence class, or changing what counts toward uniqueness requires a new strategy name (for example `structural-v2`). Content-based strategies, if any, must be explicitly optional and separately named.
