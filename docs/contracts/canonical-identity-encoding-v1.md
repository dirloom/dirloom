# Canonical Identity Encoding v1

> **Status:** normative contract for Dirloom `v0.4.0-a1`  
> **Encoding version:** `1`  
> **Related:** [Identity Projection v1](identity-projection-v1.md)

Identity bytes are a purpose-built binary document written to an `io.Writer`. Dirloom MUST NOT hash `json.Marshal` output, `fmt` formatting, or any map iteration order.

## Stream layout

All integers are unsigned big-endian. Strings are UTF-8 length-prefixed by byte length (not rune count). No padding.

```text
magic              4 bytes   "DLMI"
projection_version 1 byte    1
encoding_version   1 byte    1
record_count       4 bytes   uint32
record 1
record 2
...
record N
```

Records are the Identity Projection v1 list, already sorted by canonical path.

## Record layout

```text
path_len   4 bytes   uint32
path       path_len  UTF-8 canonical path
kind       1 byte    kind code
flags      1 byte    bit 0 = has_target
[target_len 4 bytes  uint32]          if has_target
[target     target_len UTF-8]         if has_target
```

`has_target` MUST be set if and only if kind is `symlink` or `junction`. The target field is then always present, including when the target string is empty.

## Kind codes

These values are part of the contract. They are **not** Go `iota` values and MUST NOT be reassigned.

| Kind | Code |
| --- | --- |
| `directory` | `0x01` |
| `file` | `0x02` |
| `symlink` | `0x03` |
| `junction` | `0x04` |

Unknown kinds MUST fail encoding with `unsupported node type` rather than being coerced.

## Flags

| Bit | Meaning |
| --- | --- |
| 0 (`0x01`) | Target field follows |
| 1–7 | Reserved; MUST be zero in v1 |

## Forbidden techniques

- JSON or YAML serialization as the hash input
- `map` ranging without an explicit sorted key list (v1 encoding has no maps)
- `fmt` to produce record bytes
- Little-endian integers
- Architecture-dependent padding or alignment
- Hashing presentation or configuration documents

## Streaming

The encoder writes directly to `io.Writer`. The fingerprint engine uses `crypto/sha256` as that writer so a second full copy of the document is unnecessary. Short writes and writer errors MUST be returned, never ignored.

## Versioning

`encoding_version` `1` is paired with projection version `1`. Changing either incompatibly requires a new fingerprint version token. Bytes that claim `DLMI` / v1 MUST match this layout.
