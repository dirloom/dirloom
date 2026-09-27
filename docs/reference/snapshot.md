# `dirloom snapshot`

Snapshot Schema v1 persists the **structural view** Dirloom observes after applying
the active filters. The result is a portable, self-verifying JSON document that
later `verify` and `diff` can load. A2 does **not** compare a snapshot to a live
tree.

## Purpose

```bash
dirloom snapshot
dirloom snapshot .
dirloom snapshot --root .
dirloom snapshot --output architecture.dlm.json
```

A valid snapshot is:

- persistent and portable across absolute roots;
- deterministic for the same Canonical Structural Artifact and Capture Semantics;
- self-describing for the structural observation rules that applied;
- self-verifying against its embedded Fingerprint v1;
- independent from presentation, host identity and current time.

## Syntax

```text
dirloom snapshot [directory] [flags]
```

`directory` defaults to the current directory. `--root` is an alternative to the
positional argument; they cannot be combined.

Structural flags match fingerprint / inspection: `--depth`, `--dirs-only`,
`--hidden`, `--ignore`, `--no-default-ignore`, `--no-gitignore`, `--preset`, plus
`--config` / `--no-user-config` / `--no-config`.

`--color`, `--icons` and `--theme` are accepted and ignored. They never enter
Capture Semantics, never change snapshot bytes and never inject ANSI.

There is one public serialization format in A2: JSON. There is no `--format`
matrix.

## stdout

Without `--output`, stdout is the complete Snapshot Schema v1 JSON document
(2-space indent, final newline). On success stderr is empty. The bytes are
directly parseable by a JSON parser and contain no ANSI.

## `--output`

```bash
dirloom snapshot --output architecture.dlm.json
```

Writes the snapshot transactionally via Dirloom's existing output writer:

- stdout is empty on success;
- stderr is empty on success;
- an existing regular file may be replaced atomically;
- symlink destinations, directories and other non-regular destinations are refused;
- a missing parent directory is refused;
- no partial file survives a failed write;
- there is no `--force` in A2.

When the destination lies inside the inspected root, Dirloom excludes it from
the observed artifact (same self-exclusion contract as tree `--output`). Running
the command repeatedly to the same path must not include the previous snapshot
file or change the structural fingerprint.

## Capture Semantics

The `capture` object records **effective** structural rules:

| Field | Meaning |
| --- | --- |
| `depth` | `null` unlimited; `0` root only; `N` finite |
| `dirsOnly` | directories-only |
| `hidden` | include hidden entries that survive other filters |
| `useDefaultIgnores` | built-in exclusions |
| `useGitignore` | apply `.gitignore` |
| `ignore` | effective custom patterns **in order** |

Not recorded: absolute root, config file paths, which layer supplied a value,
presentation, hostname, username, timestamps, or the operational output path.

## Structural artifact

`artifact.nodes` is Snapshot Artifact Projection v1: flat `path` / `kind` /
optional `target` records. Hierarchy is reconstructed on load. Paths are already
canonical (`/`, NFC, synthetic root `.`). Malformed persisted paths are rejected
rather than silently normalized.

## Fingerprint and self-verification

`fingerprint` is Fingerprint v1 (`dlm:v1:sha256:<digest>`) of the reconstructed
Canonical Structural Artifact. It is **not** a hash of the snapshot JSON bytes.

Load path:

1. decode JSON (duplicate keys and trailing documents rejected);
2. check schema/artifact versions and `requiredFeatures`;
3. reconstruct and `artifact.Validate`;
4. recompute Fingerprint v1;
5. require exact equality with the embedded value.

Mismatch → corrupt / invalid snapshot (`snapshot_fingerprint_mismatch`).

This is **not** a live structure mismatch. Comparing a valid snapshot to a live
tree belongs to future A3 `verify`.

Changing only Capture Semantics does not change Fingerprint v1. The embedded
fingerprint attests the structural artifact, not capture or provenance metadata.

## Corruption vs future verify mismatch

| Concept | Meaning | Owner |
| --- | --- | --- |
| Invalid / corrupt snapshot | Fails schema, artifact, feature or fingerprint self-verification | A2 |
| Valid snapshot whose structure differs from live structure | Future product behavior | A3 |

## Filters and configuration

Snapshots hash and persist the view Dirloom actually observes. Different
configuration mechanisms that produce the same effective view and Capture
Semantics produce the same canonical snapshot bytes.

## Schema compatibility

- `schemaVersion` / `artifactVersion` must be `1` for this reader;
- `requiredFeatures` is required and empty for initial v1 writers;
- unknown required features reject;
- unknown additive JSON fields in an otherwise supported v1 document are ignored;
- duplicate object keys reject.

See [Snapshot Schema v1](../contracts/snapshot-schema-v1.md) and
[ADR 0002](../adr/0002-snapshot-persistence-and-compatibility.md).

## What snapshot does not do

- live structure comparison / `verify` / `diff`
- file-content hashing or checksums
- signatures, compression, binary formats
- remote storage or snapshot databases
- volatile provenance in canonical v1 output

## Exit status

- `0` — success
- `1` — runtime/data/output error
- `2` — invalid arguments

## Contracts

- [Snapshot Schema v1](../contracts/snapshot-schema-v1.md)
- [Canonical Structural Artifact v1](../canonical-structural-artifact-v1.md)
- [Fingerprint reference](fingerprint.md)
