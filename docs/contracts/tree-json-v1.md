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
| `name` | every node | the entry name as observed, never a path |
| `type` | every node | `directory`, `file` or `symlink` |
| `target` | `symlink` only, when the link target was read | the target as recorded, with `/` separators; never resolved or made absolute |
| `children` | `directory` only | an array, empty for an empty or depth-limited directory |

- A `file` or `symlink` node never has `children`.
- A Windows junction, or a folder where a volume is mounted, is rendered as a `symlink` with its target. There is no `junction` type in this schema; the canonical artifact records kind `junction`.
- A named pipe, a socket or a device is rendered as a `file`, as in v0.1–v0.3. The structural commands refuse such an entry (`unsupported_node_type`); the tree view does not.
- The order of `children` is the order of the text renderings: directories first, then other entries, by deterministic Unicode case folding, with bytewise names as tie-breakers.
- Absolute paths, timestamps, sizes, permissions, owners and every presentation setting (color, icons, theme, style) are never present.

## Encoding

- UTF-8 without a byte order mark.
- Two-space indentation, one field per line, in the field order above.
- No HTML escaping: `<`, `>` and `&` are written as is.
- The document ends with exactly one line feed, and contains no carriage return.

## Failures and exit codes

Tree JSON uses the historical exit codes: `0` success, `1` runtime error, `2` invalid usage. On failure, stdout is empty: there is no JSON error document, and the diagnostic goes to stderr. With `--output`, an existing file is left unchanged.

**Known defect, not part of the contract:** a name that is not valid UTF-8 (for example a Windows name holding an unpaired UTF-16 surrogate) is currently written with U+FFFD replacement characters and exits 0, while the text output and the structural commands refuse it (anomaly A11). The correction, a refusal before any byte is written, is planned for `v0.4.0-rc1` (LOT 3). No golden fixture captures the replacement.

## Pinned by

- `internal/render/testdata/json.golden` and `json-escaping.golden`: the encoding of every node shape, escaping and the junction rendering;
- `testdata/identity/golden/*.tree.json`: the CLI output for real directories;
- `TestJSONContractV1` (`internal/render`): the structural rules above.

## Versioning

An incompatible change to this document needs a new `schemaVersion` and a new version of this contract. See the [compatibility and freeze policy](../compatibility-policy.md).
