# Fingerprint JSON Schema v1

> **Status:** candidate for the v0.4 freeze (`v0.4.0-rc1`); emitted since Dirloom `v0.4.0-a1`
>
> **Related:** [Identity Projection v1](identity-projection-v1.md), [Canonical Identity Encoding v1](canonical-identity-encoding-v1.md), [`dirloom fingerprint`](../reference/fingerprint.md), [Compatibility and freeze policy](../compatibility-policy.md)

Machine-readable output of `dirloom fingerprint --format json`. This document does not define identity: Identity Projection v1 and Canonical Identity Encoding v1 do. The JSON bytes are not fingerprinted.

## Document

```json
{
  "schemaVersion": 1,
  "fingerprint": "dlm:v1:sha256:e60c2e084d85e9e5288a9e4e3eb5afc1ecda32d5cba4eb503ea3668b7a3c530d",
  "identityVersion": 1,
  "algorithm": "sha256",
  "nodeCount": 4
}
```

| Field | Value |
| --- | --- |
| `schemaVersion` | the integer `1` |
| `fingerprint` | the Fingerprint v1 string, `dlm:v1:sha256:` followed by 64 lowercase hexadecimal digits |
| `identityVersion` | the integer `1`, the identity projection version |
| `algorithm` | `sha256` |
| `nodeCount` | the number of canonical nodes, root included; metadata, not hashed |

The fields are always present, in this order. The encoding is UTF-8, two-space indentation, with exactly one final line feed.

## Failures and exit codes

`fingerprint` uses the historical exit codes: `0` success, `1` runtime error, `2` invalid usage. On failure, stdout is empty, even with `--format json`: there is no JSON error document, and the diagnostic goes to stderr (decision Q4 of the `v0.4.0-rc1` plan). Diagnostic messages are not contractual.

## Pinned by

- `testdata/identity/golden/*.fingerprint.json`: the bytes of the document;
- `testdata/identity/golden/*.fingerprint`: the fingerprint values in the text output.

## Versioning

An incompatible change to this document needs a new `schemaVersion`. A change to what a fingerprint means needs a new fingerprint version (see Identity Projection v1 and the [compatibility and freeze policy](../compatibility-policy.md), rule D13).
