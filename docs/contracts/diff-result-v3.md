# Diff Result Schema v3

> **Status:** normative contract for Dirloom `v0.4.0-a6`; supersedes [Diff Result Schema v2](diff-result-v2.md)
>
> **Related:** [Git Tree Source v1](git-tree-source-v1.md), [Move Detection structural-v1](move-detection-structural-v1.md), [Identity Projection v1](identity-projection-v1.md), [ADR 0006](../adr/0006-git-sources.md)

Machine-readable output of `dirloom diff --format json` from `v0.4.0-a6` on. Schema v3 replaces v2 as the only diff JSON output: v2 is no longer emitted. v3 keeps every v2 field with the same meaning and adds Git sources. It is the first diff schema with an explicit [evolution policy](#evolution-policy). This schema does not modify Snapshot Schema v1, Identity Projection v1, Canonical Identity Encoding v1, Fingerprint v1, or Verify Result Schema v1. The JSON bytes are not fingerprinted.

## Document

```text
schemaVersion: 3
encoding: UTF-8
indentation: 2 spaces
html escaping: disabled
terminator: one final newline
```

Top-level field order is fixed by the encoder struct:

| Field | Presence |
| --- | --- |
| `schemaVersion` | always `3` |
| `status` | always |
| `metadata` | success statuses (`NO_DIFFERENCES`, `DIFFERENCES`) |
| `sources` | success statuses |
| `summary` | success statuses |
| `changes` | success statuses; `[]` when empty, never null |
| `diagnostic` | failure statuses |

`metadata` describes the comparison contract, never the environment and never the compared inputs. Field order:

| Field | Value |
| --- | --- |
| `comparisonVersion` | always `1` |
| `identityProjectionVersion` | always `1` |
| `moveDetection` | `structural-v1` (default) or `none` (`--no-moves`) |

`comparisonVersion` stays `1`: comparison is still path-anchored over Identity Projection v1 records, and `ADDED`, `REMOVED`, and `CHANGED` keep their v1 meaning. Move detection is a separate, separately versioned layer named by `moveDetection`.

`sources` names the two compared inputs:

| Field | Value |
| --- | --- |
| `a` | source object for source A |
| `b` | source object for source B |

A source object has these fields, in encoder order:

| Field | Presence |
| --- | --- |
| `kind` | always: `snapshot`, `filesystem`, or `git` |
| `nodeCount` | always: the Identity Projection v1 records of that side, root included |
| `tree` | `git` only: the observed tree id, lowercase hex (40 characters for SHA-1, 64 for SHA-256) |
| `commit` | `git` only, when the revision resolved through a commit: that commit id, lowercase hex |

For `snapshot` and `filesystem`, a source object is exactly `{ "kind", "nodeCount" }`, as in v2. Dates, hosts, users, absolute source roots, source expressions, revision text, repository paths, remotes, and branch names are never emitted.

`summary` is `{ "added": n, "removed": n, "changed": n, "moved": n, "total": n }` in that order, with the exact invariant `total = added + removed + changed + moved`, which equals the length of `changes`.

`diagnostic` is `{ "source": "a" | "b", "code": string, "message": string }`. `source` names the side whose resolution or observation failed; it is omitted only when no side is responsible: an internal error before either side was resolved, or a comparison, move detection, or validation invariant failure after both sides were observed.

## Change vocabulary

A change compares source A (before) with source B (after). The public JSON field for the verb is `"operation"`. Change fields, in encoder order:

| Field | Presence |
| --- | --- |
| `path` | always; canonical Identity Projection v1 path, never the root `.`. For `MOVED`, the destination in B |
| `operation` | `ADDED`, `REMOVED`, `CHANGED`, or `MOVED` |
| `from` | `MOVED` only; canonical origin path in A, never the root `.` |
| `before` | `REMOVED`, `CHANGED`, `MOVED`: the node existed in A |
| `after` | `ADDED`, `CHANGED`, `MOVED`: the node exists in B |
| `evidence` | `MOVED` only: `subtree` or `name` |
| `nodes` | `MOVED` only: number of Identity Projection v1 records the move covers, root included |

A node state is `{ "kind": string }` plus `target` for kinds that carry one (`symlink`, `junction`). `target` is present with an explicit value whenever the kind carries one, including the empty string; it is absent otherwise. A target is emitted as Identity Projection v1 records it and can be absolute when a link points outside the source root. A Git link target is emitted as recorded in the tree, NFC-normalized ([Git Tree Source v1](git-tree-source-v1.md#link-targets)).

| `operation` | Meaning |
| --- | --- |
| `ADDED` | `path` exists in B and not in A, and is not covered by a `MOVED`; only `after` is present |
| `REMOVED` | `path` exists in A and not in B, and is not covered by a `MOVED`; only `before` is present |
| `CHANGED` | `path` exists in both and the Identity Projection v1 record differs (kind, or target of a target-carrying kind); `before != after` |
| `MOVED` | the node at `from` in A and the node at `path` in B correspond under the strategy named by `metadata.moveDetection`; both `before` and `after` are present |

Rules:

- Changes are sorted by ascending `path` as UTF-8 bytes. There is at most one change per `path`.
- Each A-side path appears at most once across `REMOVED` paths and `MOVED` `from` values.
- `from` and `path` of one `MOVED` differ, and neither is an ancestor of the other.
- No `REMOVED` path and no `MOVED` `from` lies below another `MOVED` `from`. No `ADDED` path and no `MOVED` `path` lies below another `MOVED` `path`.
- Under `structural-v1`, `before` equals `after` for every `MOVED`; `evidence` is `name` exactly when `nodes` is `1`; `evidence: subtree` implies `kind: directory` and `nodes ≥ 2`.
- When `moveDetection` is `none`, `changes` contains no `MOVED` and `summary.moved` is `0`.
- File contents are never hashed; a content-only edit produces no change, for Git sources as for filesystem sources.

### Absorbed descendants

A `MOVED` covers its whole subtree: every descendant of `from` in A and of `path` in B is absorbed into it and never emitted separately. A v3 document is therefore **not reversible on its own**: `nodes: 1000` says how many records moved, not which. Reconstructing every changed path requires the A and B Identity Projection v1 records, or running `dirloom diff --no-moves`.

### A correspondence, not proof of a move

A `MOVED` reports a deterministic, unique structural correspondence under the named strategy. It is not historical evidence that the object was moved, even between two Git revisions. See [Move Detection structural-v1](move-detection-structural-v1.md#a-correspondence-not-proof-of-a-move).

## `--no-moves` and A4 equivalence

With `--no-moves`, `metadata.moveDetection` is `none`. For `snapshot:` and `live:` sources, the `changes` array and the `added`, `removed`, and `changed` counts are exactly those of `v0.4.0-a4` (Diff Result Schema v1) for the same inputs.

## Status vocabulary

| `status` | Exit | Meaning |
| --- | ---: | --- |
| `NO_DIFFERENCES` | 0 | Both sources observed, no structural change |
| `DIFFERENCES` | 1 | Both sources observed, at least one structural change, `MOVED` included |
| `INVALID_SNAPSHOT` | 3 | A `snapshot:` input is corrupt or semantically invalid Snapshot Schema v1 |
| `UNSUPPORTED_SNAPSHOT` | 4 | A `snapshot:` input uses a schema, artifact version, or required feature this release cannot accept |
| `SNAPSHOT_READ_ERROR` | 5 | A `snapshot:` input could not be read |
| `OBSERVATION_ERROR` | 5 | A source could not be read or did not yield a trustworthy artifact, including every Git source failure |
| `INTERNAL_ERROR` | 6 | Broken Dirloom invariant, including a move detection or Git backend invariant |

Statuses and exit codes are unchanged from v2. Exit `2` is reserved for invalid CLI usage and is not a diff JSON document: the human diagnostic stays on stderr even when `--format json` was requested. These are usage failures: `live:` against `live:`, `git:` against `live:` in either order, and a `git:` source whose revision is empty, starts with `-`, contains a NUL or a newline, or names an index entry (starts with `:` but not `:/`). A failed write of the JSON document itself is an operational exit `5` on stderr; that partial stdout is not a successful result.

`DIFFERENCES` (exit 1) is a normal result, not an error: stderr stays empty.

Failure documents have the same shape as in v2, with `schemaVersion: 3`.

## Diagnostic codes

Snapshot validation reuses `snapshot.CodeOf`:

```text
invalid_snapshot_json
missing_snapshot_field
invalid_snapshot_field
invalid_snapshot_artifact
snapshot_fingerprint_mismatch
unsupported_snapshot_schema
unsupported_snapshot_artifact_version
unsupported_snapshot_feature
snapshot_read_failure
```

`snapshot_fingerprint_mismatch` is always `INVALID_SNAPSHOT`, never `DIFFERENCES`.

Live observation diagnostics reuse `artifact.Error.Code` when present, including `source_read_failure`, `canonical_path_collision`, and `internal_invariant_failure`.

Git source diagnostics, all `OBSERVATION_ERROR` (exit 5), defined by [Git Tree Source v1](git-tree-source-v1.md#diagnostics):

```text
git_unavailable
git_repository_not_found
git_revision_not_found
git_object_unavailable
git_tree_invalid
```

Fallback codes, used only when no typed snapshot, artifact, or Git code exists:

```text
diff_observation_failure
diff_internal_failure
```

## Stream rules

When `--format json` is accepted:

- stdout is exactly one JSON document plus a final newline;
- stderr is empty for that classified outcome;
- no ANSI sequence is emitted;
- no human sentence is appended.

Usage errors and a stdout write failure are stderr diagnostics instead.

## Evolution policy

From v3 on, consumers must:

- ignore object fields they do not know;
- treat a `sources.*.kind` value they do not know as opaque, and still read `nodeCount`;
- treat a diagnostic `code` they do not know as opaque, and still read `status`.

Within v3, these changes are additive and keep `schemaVersion: 3`:

- a new `sources.*.kind` value, with optional fields of its own;
- a new optional field on an existing object;
- a new diagnostic code under an existing status.

These changes require a new schema version:

- a new `operation`, `status`, `evidence` value, or `moveDetection` strategy;
- a new exit code;
- removing or renaming a field, or changing the meaning, type, or presence rule of an existing field;
- changing the order of fields or of changes.

## Examples

Illustrative only; whitespace is condensed here, and the v3 golden fixtures are normative for bytes.

A directory relocated between two commits:

```json
{
  "schemaVersion": 3,
  "status": "DIFFERENCES",
  "metadata": { "comparisonVersion": 1, "identityProjectionVersion": 1, "moveDetection": "structural-v1" },
  "sources": {
    "a": { "kind": "git", "nodeCount": 7, "tree": "<40 hex>", "commit": "<40 hex>" },
    "b": { "kind": "git", "nodeCount": 8, "tree": "<40 hex>", "commit": "<40 hex>" }
  },
  "summary": { "added": 1, "removed": 0, "changed": 0, "moved": 1, "total": 2 },
  "changes": [
    { "path": "src/features", "operation": "ADDED", "after": { "kind": "directory" } },
    {
      "path": "src/features/payment",
      "operation": "MOVED",
      "from": "src/old/payment",
      "before": { "kind": "directory" },
      "after": { "kind": "directory" },
      "evidence": "subtree",
      "nodes": 4
    }
  ]
}
```

A snapshot against a live tree is the v2 document with `"schemaVersion": 3`:

```json
{
  "schemaVersion": 3,
  "status": "NO_DIFFERENCES",
  "metadata": { "comparisonVersion": 1, "identityProjectionVersion": 1, "moveDetection": "structural-v1" },
  "sources": { "a": { "kind": "snapshot", "nodeCount": 7 }, "b": { "kind": "filesystem", "nodeCount": 7 } },
  "summary": { "added": 0, "removed": 0, "changed": 0, "moved": 0, "total": 0 },
  "changes": []
}
```

## Migration from v2

| v2 | v3 |
| --- | --- |
| `schemaVersion: 2` | `schemaVersion: 3` |
| `sources.*.kind`: `snapshot`, `filesystem` | + `git` |
| source object `{ kind, nodeCount }` | + `tree`, `commit` for `git` only |
| snapshot and live diagnostic codes | + five `git_*` codes |
| no extension rule | evolution policy |

For `snapshot:` and `live:` sources, a v3 document equals the v2 document for the same inputs except `schemaVersion`. Consumers that check `schemaVersion` fail loudly on v3, never silently.
