# Snapshot Schema v1

> **Status:** normative contract for Dirloom `v0.4.0-a2`
>
> **Date:** 2026-09-24

## Purpose

Snapshot Schema v1 is the durable, portable, self-verifying serialization of a
Canonical Structural Artifact v1 together with the effective structural Capture
Semantics that produced that artifact.

A snapshot is a **reference**, not a live comparison. Loading a snapshot that
fails self-verification is **corruption / invalid snapshot**. Comparing a valid
snapshot to a live structure is reserved for future `verify` (A3).

## Top-level document

```json
{
  "schemaVersion": 1,
  "artifactVersion": 1,
  "fingerprint": "dlm:v1:sha256:...",
  "requiredFeatures": [],
  "capture": {
    "depth": null,
    "dirsOnly": false,
    "hidden": false,
    "useDefaultIgnores": true,
    "useGitignore": true,
    "ignore": []
  },
  "artifact": {
    "nodes": [
      { "path": ".", "kind": "directory" },
      { "path": "src", "kind": "directory" },
      { "path": "src/main.go", "kind": "file" },
      { "path": "link", "kind": "symlink", "target": "src/main.go" }
    ]
  }
}
```

### Required fields

| Field | Type | Notes |
| --- | --- | --- |
| `schemaVersion` | number (integer) | Must be `1` for this contract |
| `artifactVersion` | number (integer) | Must be `1` for Artifact Projection v1 |
| `fingerprint` | string | Exact A1 fingerprint of the reconstructed artifact |
| `requiredFeatures` | array of strings | Empty for initial v1 writers |
| `capture` | object | Capture Semantics v1 |
| `artifact` | object | Snapshot Artifact Projection v1 |

Missing required fields MUST be distinguishable from valid zero-values. Readers
MUST NOT infer presence from Go zero values alone.

### Schema and artifact versions

- Unknown `schemaVersion` → `unsupported_snapshot_schema`
- Unknown `artifactVersion` → `unsupported_snapshot_artifact_version`

These versions may evolve independently.

## Capture Semantics v1

Records **effective** structural observation rules. Does **not** record config
provenance, presentation, absolute root, hostname, username, time, or the
operational output path used for self-exclusion.

| Field | Meaning |
| --- | --- |
| `depth` | `null` = unlimited; `0` = root only; `N` = finite depth |
| `dirsOnly` | directories-only view |
| `hidden` | include hidden entries that survive other filters |
| `useDefaultIgnores` | built-in directory exclusions |
| `useGitignore` | apply `.gitignore` files |
| `ignore` | effective custom ignore patterns **in order** |

`ignore` order is semantically significant and MUST be preserved. Do not sort.

## Snapshot Artifact Projection v1

Flat list of nodes. Hierarchy is reconstructable from canonical parent paths.
`name` is not persisted; it is derived from the path.

| Field | Required | Notes |
| --- | --- | --- |
| `path` | yes | Already-canonical relative path (`/` is the only separator, NFC, root `.`). A `\` inside a segment is a POSIX filename character and must be preserved. |
| `kind` | yes | `directory`, `file`, `symlink`, or `junction` |
| `target` | symlink/junction only | Present even when empty; forbidden on other kinds |

Writer order: ascending canonical UTF-8 path bytes. Readers MAY accept another
order but MUST normalize before reuse. Persisted paths that are not already
canonical MUST be rejected — never silently normalized.

`DisplayRootName` and absolute roots MUST NOT appear in the snapshot.

## Fingerprint

The embedded `fingerprint` is Fingerprint v1 (`dlm:v1:sha256:<digest>`) of the
reconstructed Canonical Structural Artifact. It is computed through Identity
Projection v1 and Canonical Identity Encoding v1 — **not** by hashing snapshot
JSON bytes.

Self-verification:

1. Parse embedded fingerprint
2. Reconstruct `artifact.Artifact` from the projection
3. Run `artifact.Validate`
4. Recompute `identity.Compute`
5. Require exact equality

Mismatch → `snapshot_fingerprint_mismatch` (corrupt / invalid snapshot).

Changing only Capture Semantics does not change Fingerprint v1. The embedded
fingerprint attests the structural artifact, not capture metadata or future
provenance metadata.

## requiredFeatures

Every v1 writer emits `"requiredFeatures": []`.

- Strings only, no duplicates
- Canonical writers sort the array
- Unknown required feature → `unsupported_snapshot_feature`

## Compatibility policy

| Situation | Behavior |
| --- | --- |
| Unknown `schemaVersion` | Reject |
| Unknown `artifactVersion` | Reject |
| Unknown required feature | Reject |
| Unknown additive JSON field in a supported v1 document | Accept and ignore |
| Duplicate JSON object key | Reject |
| Extra JSON document / trailing non-whitespace | Reject |

Snapshot bytes must be valid UTF-8. Invalid UTF-8 is rejected before parsing and is never replaced with U+FFFD.

Readers also reject capture metadata that the artifact itself disproves: `dirsOnly: true` with a file, symlink, or junction, and `depth: N` with a node deeper than N.

“Strict JSON” means syntactically valid JSON, exactly one document, required
fields present with correct types, duplicate keys rejected, canonical path
rules enforced, and schema/artifact/features validated. It does **not** mean
`DisallowUnknownFields`.

## Provenance

Volatile provenance (`capturedAt`, hostname, username, absolutePath, …) is
intentionally omitted from canonical Snapshot v1 output.

## Non-goals (A2)

Live structure comparison, `verify`, structural `diff`, content hashing,
signatures, compression, binary formats, and snapshot migration engines are
out of scope.
