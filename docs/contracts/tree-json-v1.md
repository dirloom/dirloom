# Tree JSON Schema v1

> **Status:** candidate for the v0.4 freeze (`v0.4.0-rc1`); public contract since Dirloom v0.1 ([SPEC-v0.1.md](../../SPEC-v0.1.md) §6.4)
>
> **Related:** [Compatibility and freeze policy](../compatibility-policy.md), [Contract register](README.md)

Machine-readable output of `dirloom [directory] --format json`. It describes the observed tree as rendered, after filters, depth and `--dirs-only`. It is not a structural artifact: it is not fingerprinted, and it is independent of Snapshot Schema v1 and of every v0.4 result schema.

## Document

```json
{
  "schemaVersion": 1,
  "root": {
    "name": "project",
    "type": "directory",
    "children": [
      { "name": "src", "type": "directory", "children": [] },
      { "name": "link", "type": "symlink", "target": "../shared" },
      { "name": "README.md", "type": "file" }
    ]
  }
}
```

Illustrative only; whitespace is condensed here, and the golden fixtures are normative for bytes.

| Field | Presence | Value |
| --- | --- | --- |
| `schemaVersion` | always | the integer `1` |
| `root` | always | the node of the inspected root; its `name` is the base name of the inspected directory |
| `name` | every node | the entry name as observed, never a path; always valid UTF-8 (see [Encoding](#encoding)) |
| `type` | every node | `directory`, `file` or `symlink` |
| `target` | `symlink` only, when the link target was read | the target as recorded, with `/` separators; never resolved or made absolute; always valid UTF-8 |
| `children` | `directory` only | an array, empty for an empty or depth-limited directory |

- A `file` or `symlink` node never has `children`.
- A Windows junction, or a folder where a volume is mounted, is rendered as a `symlink` with its target. There is no `junction` type in this schema; the canonical artifact records kind `junction`.
- A named pipe, a socket or a device is rendered as a `file`, as in v0.1–v0.3. The structural commands refuse such an entry (`unsupported_node_type`); the tree view does not.
- The order of `children` is the order of the text renderings: directories first, then other entries, by deterministic Unicode case folding, with bytewise names as tie-breakers.
- Absolute paths, timestamps, sizes, permissions, owners and every presentation setting (color, icons, theme, style) are never present.

## Encoding

- UTF-8 without a byte order mark.
- Every name and every target is written exactly as observed. A name or a target that is not valid UTF-8, such as a Windows name holding an unpaired UTF-16 surrogate or a Linux name holding arbitrary bytes, cannot be written as a JSON string without changing it: the document is refused (see [Failures](#failures-and-exit-codes)). It is never written with U+FFFD replacement characters.
- Two-space indentation, one field per line, in the field order above.
- No HTML escaping: `<`, `>` and `&` are written as is.
- The document ends with exactly one line feed, and contains no carriage return.

## Failures and exit codes

Tree JSON uses the historical exit codes: `0` success, `1` runtime error, `2` invalid usage. On failure, stdout is empty: there is no JSON error document, and the diagnostic goes to stderr. With `--output`, an existing file is left unchanged; with `--copy`, the clipboard is not written.

A name or a target in the rendered tree that is not valid UTF-8 is a runtime error, exit `1`, as for the text output. Every string is checked before any byte of the document is written, so nothing is written. The diagnostic names the entry by its path from the root, with the invalid bytes escaped, for example `entry "src/bad\xed\xa0\x80.txt": name is not valid UTF-8`; its wording is not part of the contract. Only the rendered tree is checked: an entry excluded by a filter, by depth or by `--dirs-only` is no refusal. Until `v0.4.0-rc1` such a name was written with U+FFFD replacement characters and exit `0` (anomaly A11); no golden fixture ever captured that replacement.

## Pinned by

- `internal/render/testdata/json.golden` and `json-escaping.golden`: the encoding of every node shape, escaping and the junction rendering;
- `testdata/identity/golden/*.tree.json`: the CLI output for real directories;
- `TestJSONContractV1` (`internal/render`): the structural rules above;
- `TestJSONRefusesInvalidUTF8` (`internal/render`) and `TestTreeJSONRefusesInvalidUTF8*` (`internal/cli`, on real NTFS names holding an unpaired surrogate on Windows, raw bytes on Linux): the refusal of invalid UTF-8, with no output;
- `TestTreeJSONGoldensNeverHoldReplacement` (`internal/render`): no golden holds U+FFFD.

## Versioning

An incompatible change to this document needs a new `schemaVersion` and a new version of this contract. See the [compatibility and freeze policy](../compatibility-policy.md).
