# Verify Result Schema v1

> **Status:** candidate for the v0.4 freeze (`v0.4.0-rc1`), see the [contract register](README.md); normative since Dirloom `v0.4.0-a3`

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

Live artifact diagnostics reuse `artifact.Error.Code` when present:

- `OBSERVATION_ERROR` (exit 5): `source_read_failure`, `canonical_path_collision`, `invalid_canonical_path` (an entry name that is not a valid canonical name), `unsupported_node_type` (a named pipe, socket or device in the observed view);
- `INTERNAL_ERROR` (exit 6): `internal_invariant_failure`, `artifact_invariant_violation` (an artifact Dirloom built from the observation breaks an artifact rule), `canonical_encoding_failure`, `snapshot_internal_failure`.

The same artifact rule broken inside a snapshot is `invalid_snapshot_artifact` (`INVALID_SNAPSHOT`, exit 3). Every code, with its category, is listed in [Exit codes and machine codes](../reference/exit-codes.md).

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

Usage errors and a stdout write failure are stderr diagnostics instead. So is an unexpected panic, with exit 6: stdout then keeps what was written before it, possibly nothing or a truncated document, and gets no failure document ([exit codes](../reference/exit-codes.md#unexpected-internal-failures)).

## Evolution policy

Consumers must:

- ignore object fields they do not know;
- treat a diagnostic `code` they do not know as opaque, and still read `status`.

Within v1, these changes are additive and keep `schemaVersion: 1`:

- a new optional field on an existing object;
- a new diagnostic code under an existing status.

These changes require a new schema version:

- a new `status`;
- a new exit code, or a different exit code for an existing status;
- removing or renaming a field, or changing the meaning, type, or presence rule of an existing field;
- changing the order of fields.

Diagnostic messages are not part of the contract; programs branch on `status`, `code` and the exit code. The golden fixtures under `testdata/verify/v1/golden` are normative for bytes, except for the operating-system text of a diagnostic message. See also the [compatibility and freeze policy](../compatibility-policy.md).
