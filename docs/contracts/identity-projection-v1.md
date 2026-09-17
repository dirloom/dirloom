# Identity Projection v1

> **Status:** normative contract for Dirloom `v0.4.0-a1`  
> **Projection version:** `1`  
> **Fingerprint namespace:** `dlm`  
> **Hash:** SHA-256  
> **Related:** [Canonical Structural Artifact v1](../canonical-structural-artifact-v1.md), [Canonical Identity Encoding v1](canonical-identity-encoding-v1.md)

Identity Projection v1 maps a validated Canonical Structural Artifact to an ordered list of identity records. Those records — not JSON, not the observation tree, not presentation — are what Canonical Encoding v1 serializes and what SHA-256 hashes.

```text
Canonical Artifact
        ↓
Identity Projection v1
        ↓
ordered node records
```

If this projection changes incompatibly, the fingerprint MUST NOT continue to use `dlm:v1`.

## Included

Each record contains only:

| Field | Source | Notes |
| --- | --- | --- |
| Canonical path | Artifact path | NFC, `/` separators, root is `.` |
| Node kind | Artifact kind | `directory`, `file`, `symlink`, `junction` |
| Target | Artifact target | Present for `symlink` and `junction` only (possibly empty) |

The root directory is included as a record with path `.`.

## Excluded

Identity Projection v1 excludes at least:

```text
mtime, ctime, atime
owner, uid, gid, ACL
platform-specific permissions
executable bit
absolute root
hostname, username
capture timestamp
terminal, TTY
theme, colors, icons, presentation styles
output indentation
file contents
file size
display root label / source label
configuration provenance (CLI vs YAML vs preset)
scanner index, memory address
child slice order of the artifact
source kind (filesystem, …)
```

Configuration that **changes the observed structure** (ignore, depth, hidden, gitignore, dirs-only) changes the artifact and therefore the projection. Configuration that only selects presentation does not.

## Normalized

| Input | Normalization |
| --- | --- |
| Paths | NFC, lexical `.`/`..` handling, `/` separators |
| Names | Validated against the path; not serialized |
| Symlink targets | NFC, portable separators, root-relative rewrite when the target is absolute inside the source root |
| Record order | Ascending canonical path, UTF-8 byte order |

## Derived

| Value | Derivation |
| --- | --- |
| Name | Last path segment; validated, not encoded |
| Fingerprint string | `dlm:v1:sha256:` plus lowercase hex of SHA-256 over Canonical Encoding v1 |
| Node count in JSON | Count of projected records; not hashed |

## Name policy

`Name` is validated so it cannot drift from `Path`, then omitted from the record. The canonical path is the identity truth for naming.

## Symlink semantics

- Kind `symlink` (or `junction`) is encoded explicitly.
- The target string is part of identity when the kind is `symlink` or `junction`.
- Target contents are never read.
- A broken symlink is a normal record, not an error.
- Identity does not follow the link.

## Ordering

Records are sorted by canonical path as UTF-8 bytes. This is independent of `internal/tree.Sort`, locale collation, and filesystem readdir order. Permuting artifact `Children` MUST NOT change the projection.

## Versioning

| Item | Value |
| --- | --- |
| Identity Projection Version | `1` |
| Encoding Version | `1` (see encoding contract) |
| Fingerprint Namespace | `dlm` |
| Hash Algorithm | `sha256` |

Public fingerprints look like:

```text
dlm:v1:sha256:8e5b2d7f...
```

The `v1` token is the identity projection version. A new projection or encoding MUST use a new version token.
