# Diff Result Schema v1

Machine-readable output of `dirloom diff --format json`. This schema does not modify Snapshot Schema v1, Identity Projection v1, Canonical Identity Encoding v1, Fingerprint v1, or Verify Result Schema v1. The JSON bytes are not fingerprinted.

## Document

```text
schemaVersion: 1
encoding: UTF-8
indentation: 2 spaces
html escaping: disabled
terminator: one final newline
```

Top-level field order is fixed by the encoder struct:

| Field | Presence |
| --- | --- |
| `schemaVersion` | always `1` |
| `status` | always |
| `metadata` | success statuses (`NO_DIFFERENCES`, `DIFFERENCES`) |
| `summary` | success statuses |
| `changes` | success statuses; `[]` when empty, never null |
| `diagnostic` | failure statuses |

`metadata` describes the comparison, never the environment:

| Field | Value |
| --- | --- |
| `comparisonVersion` | always `1` |
| `identityProjectionVersion` | always `1` |
| `a` | `{ "kind": string, "nodeCount": number }` for source A |
| `b` | `{ "kind": string, "nodeCount": number }` for source B |

`kind` is `snapshot` or `filesystem`. `nodeCount` counts the Identity Projection v1 records of that side, root included. Dates, hosts, users, absolute paths, and source expressions are never emitted.

`summary` is `{ "added": n, "removed": n, "changed": n, "total": n }` with the exact invariant `total = added + removed + changed`.

`diagnostic` is `{ "source": "a" | "b", "code": string, "message": string }`. `source` names the side whose resolution or observation failed; it is omitted only when no side is responsible (internal error before either side was resolved).

## Change vocabulary

A change is one canonical path compared between source A (before) and source B (after):

| Field | Presence |
| --- | --- |
| `path` | always; canonical Identity Projection v1 path, never the root `.` |
| `op` | `ADDED`, `REMOVED`, or `CHANGED` |
| `after` | `ADDED` and `CHANGED` |
| `before` | `REMOVED` and `CHANGED` |

A node state is `{ "kind": string }` plus `target` for kinds that carry one (`symlink`, `junction`). `target` is present with an explicit value whenever the kind carries one, including the empty string; it is absent otherwise.

| `op` | Meaning |
| --- | --- |
| `ADDED` | `path` exists in B and not in A; only `after` is present |
| `REMOVED` | `path` exists in A and not in B; only `before` is present |
| `CHANGED` | `path` exists in both and the Identity Projection v1 record differs (kind, or target of a target-carrying kind); `before != after` |

Changes are sorted by ascending canonical path as UTF-8 bytes. There is at most one change per path. A rename or move is not an operation: it appears as one `REMOVED` plus one `ADDED`. File contents are never hashed; a content-only edit produces no change.

## Status vocabulary

| `status` | Exit | Meaning |
| --- | ---: | --- |
| `NO_DIFFERENCES` | 0 | Both sources observed, no structural change |
| `DIFFERENCES` | 1 | Both sources observed, at least one structural change |
| `INVALID_SNAPSHOT` | 3 | A `snapshot:` input is corrupt or semantically invalid Snapshot Schema v1 |
| `UNSUPPORTED_SNAPSHOT` | 4 | A `snapshot:` input uses a schema, artifact version, or required feature this release cannot accept |
| `OBSERVATION_ERROR` | 5 | A source could not be read or did not yield a trustworthy artifact |
| `INTERNAL_ERROR` | 6 | Broken Dirloom invariant |

Exit `2` is reserved for invalid CLI usage and is not a diff JSON document: the human diagnostic stays on stderr even when `--format json` was requested. `live:<dir>` compared with `live:<dir>` is such a usage failure. A failed write of the JSON document itself is an operational exit `5` on stderr; that partial stdout is not a successful result.

`DIFFERENCES` (exit 1) is a normal result, not an error: stderr stays empty.

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

Fallback codes, used only when no typed snapshot or artifact code exists:

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
