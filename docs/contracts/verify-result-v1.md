# Verify Result Schema v1

Machine-readable output of `dirloom verify --format json`. This schema does not modify Snapshot Schema v1 or Fingerprint v1. The JSON bytes are not fingerprinted.

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
| `expectedFingerprint` | `MATCH`, `MISMATCH`, and `OBSERVATION_ERROR` after a validated snapshot |
| `actualFingerprint` | `MATCH` and `MISMATCH` only |
| `diagnostic` | failure statuses |

`diagnostic` is `{ "code": string, "message": string }`.

An untrusted fingerprint from a snapshot that failed `snapshot.Load` is never copied into `expectedFingerprint`.

## Status vocabulary

| `status` | Exit | Meaning |
| --- | ---: | --- |
| `MATCH` | 0 | Valid snapshot, live observation succeeded, fingerprints equal |
| `MISMATCH` | 1 | Valid snapshot, live observation succeeded, fingerprints differ |
| `INVALID_SNAPSHOT` | 3 | Snapshot Schema v1 input is corrupt or semantically invalid |
| `UNSUPPORTED_SNAPSHOT` | 4 | Schema, artifact version, or required feature is not supported |
| `SNAPSHOT_READ_ERROR` | 5 | The reference path could not be read |
| `OBSERVATION_ERROR` | 5 | The live tree did not yield a trustworthy fingerprint |
| `INTERNAL_ERROR` | 6 | Broken Dirloom invariant |

Exit `2` is reserved for invalid CLI usage and is not a verify JSON document. A failed write of the JSON document itself is an operational exit `5` on stderr; that partial stdout is not a successful result.

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

`snapshot_fingerprint_mismatch` is always `INVALID_SNAPSHOT`, never `MISMATCH`.

Live artifact diagnostics reuse `artifact.Error.Code` when present, including `source_read_failure`, `canonical_path_collision`, and `internal_invariant_failure`.

Fallback codes, used only when no typed snapshot or artifact code exists:

```text
verify_observation_failure
verify_internal_failure
```

## Stream rules

When `--format json` is accepted:

- stdout is exactly one JSON document plus a final newline;
- stderr is empty for that classified outcome;
- no ANSI sequence is emitted;
- no human sentence is appended.

Usage errors and a stdout write failure are stderr diagnostics instead.
