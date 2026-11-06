# Canonical Structural Artifact v1

> **Status:** candidate for the v0.4 freeze (`v0.4.0-rc1`), see the [contract register](contracts/README.md); normative since Dirloom `v0.4.0-a1`
>
> **Audience:** engineering
>
> **Related:** [Identity Projection v1](contracts/identity-projection-v1.md), [Canonical Identity Encoding v1](contracts/canonical-identity-encoding-v1.md), [fingerprint command](reference/fingerprint.md)

This document defines the **Canonical Structural Artifact**: the shared structural truth used by Structural Version Control. Presentation, configuration provenance and filesystem location are outside this model.

## Role

The artifact is the deterministic, renderer-independent structure Dirloom observed after applying the active structural rules (depth, hidden, ignores, `.gitignore`, directories-only).

It is **not**:

- a dump of every physical filesystem object;
- a presentation tree (theme, icons, colors, indentation);
- an identity document (that is a projection of this artifact);
- a snapshot store (persistence is out of scope for a1).

## Observation vs artifact vs identity

```text
Resolved configuration
        ↓
Structural source (filesystem in a1)
        ↓
Observation (internal/tree.Node)
        ↓
Canonical Structural Artifact   ← this contract
        ↓
Identity Projection v1
        ↓
Canonical Identity Encoding v1
        ↓
SHA-256 → dlm:v1:sha256:<digest>
```

- **Observation** is what the existing scanner returns, including a display root label derived from the physical basename.
- **Artifact** rewrites that observation into portable canonical nodes. The physical root name is stored only as an explicit display label and is not structural identity.
- **Identity** is a flattened, ordered projection of the artifact. The artifact may later gain snapshot metadata without changing Identity Projection v1.

## Root

There is exactly one root node.

| Field | Canonical value |
| --- | --- |
| Path | `.` |
| Name | `.` |
| Kind | `directory` |

The physical folder name (`project`, `project-copy`, a temp directory) MUST NOT appear in the root path or root name. Implementations MAY keep `DisplayRootName` on the artifact for diagnostics; Identity Projection v1 MUST ignore it.

## Node

Each node has:

| Field | Role |
| --- | --- |
| Path | Canonical path (see below) |
| Name | Last path segment; `.` for the root |
| Kind | Closed enumeration |
| Target | Symlink or junction target only |
| Children | Direct children; directories only |

After canonicalization the artifact is logically immutable. Identity code MUST NOT reorder or mutate the shared artifact; it copies what it needs.

## Path

A canonical path is:

- relative to the synthetic root;
- separated only by `/`;
- Unicode NFC;
- case-preserving (no folding);
- free of `.` and `..` segments after lexical normalization.

Examples of valid paths: `.`, `src`, `src/core`, `src/core/main.go`.

Forbidden in a canonical path string used as a separator or as an absolute location:

- native absolute paths (`C:\project\src`, `/home/user/project/src`);
- `..` escaping the root;
- backslash used as a path separator.

A POSIX filename may contain `\`; that character is then part of a single segment, not a separator.

## Name

For every non-root node, `Name` MUST equal the last `/`-separated segment of `Path`. The name is validated and is **not** an independent identity field. The root name is always `.`.

## Type (kind)

The closed kind set is:

| Kind | Meaning |
| --- | --- |
| `directory` | Directory |
| `file` | Regular file |
| `symlink` | Symbolic link (POSIX or Windows) |
| `junction` | Windows junction |

On Windows, FilesystemSource records an entry whose reparse tag is `IO_REPARSE_TAG_MOUNT_POINT` with kind `junction`: a junction, and a folder where a volume is mounted, which uses the same tag. The tag decides, not the file mode Go derives from it (`os.ModeIrregular` by default since Go 1.23, `os.ModeSymlink` with `GODEBUG=winsymlink=0`). A Windows symbolic link (`IO_REPARSE_TAG_SYMLINK`) is kind `symlink`. Other reparse points are not reclassified as links. A Git source never records a junction.

Up to Dirloom `v0.4.0-a8`, the scanner recorded a junction with kind `file` and no target. A snapshot taken then still validates; compared with the same tree observed now, the junction is a `CHANGED` entry (`file` → `junction`).

Special types that are not in this set MUST NOT be silently stored as `file`. They MUST fail with `unsupported node type`.

Files and symlinks/junctions MUST NOT have children. Directories MAY have an empty child list (including a depth-limited directory).

## Hierarchy and children

Children belong to their parent: joining the parent path with the child name MUST equal the child path. The parent of every non-root node MUST exist in the tree as a `directory`. Duplicate canonical paths are forbidden.

## Symlink

A symlink remains a symlink. Identity MUST NOT follow the target, MUST NOT read target contents, MUST NOT depend on target `mtime` or on whether the target currently exists, and MUST NOT recursively expand the target.

The stored target is a deterministic string:

- Unicode NFC;
- `/` as separator when the source uses a native separator;
- if the raw target is absolute and lexically inside the source root, it is rewritten as a root-relative canonical path so relocating the repository does not change identity;
- otherwise the recorded target is kept (broken links included).

`..` in a target string is allowed; it is part of the recorded target, not a traversal of the artifact.

## Junction

When present as kind `junction`, the node is terminal and uses the same target rules as a symlink. It is never traversed. Its reparse tag is read through a handle opened on the junction itself, never on its target, so a junction whose target is missing is still a junction. Its target is always absolute when recorded (for example `C:/work/shared`, or `//?/Volume{…}/` for a mounted volume), and becomes root-relative when it points inside the source root.

## Special files

Device nodes, sockets, FIFOs and other types outside the kind enumeration are unsupported in the artifact. They produce `unsupported node type` (`unsupported_node_type`) rather than a regular-file approximation.

The filesystem source refuses an entry whose Go file mode is a named pipe (`os.ModeNamedPipe`), a socket (`os.ModeSocket`), a device (`os.ModeDevice`) or a character device (`os.ModeCharDevice`), on every operating system. A Windows AF_UNIX socket is included. The error names the entry's path and its type.

**Not refused:**

- A non-directory `os.ModeIrregular` entry is a `file`. Go gives that mode to Windows files behind a reparse point, such as app execution aliases, cloud placeholders and compressed files, and never sets it on Linux or macOS.
- An entry excluded before observation (ignore rules, default ignores, hidden entries, `--dirs-only`, depth) is never refused.

The tree views (text, Markdown, JSON, graph exports) keep showing such an entry as a file, as they did in v0.1–v0.3. Only the canonical artifact refuses it.

Up to Dirloom `v0.4.0-a8`, the filesystem source recorded these entries as `file`. A snapshot taken then still validates, but observing the same tree now fails with `unsupported_node_type`.

Logical artifacts (tests, future sources) MUST also reject unknown kinds.

## Ordering

Child slice order in the artifact is **not** identity. Visual tree sorting (directories first, Unicode case folding) stays in `internal/tree` and MUST NOT be used as the identity order.

Identity Projection v1 sorts records by canonical path as raw UTF-8 bytes after NFC. That order is independent of locale, filesystem, terminal and OS.

## Invariants

An artifact is valid only when all of the following hold:

- exactly one root;
- root path is `.`;
- root kind is `directory`;
- no absolute path;
- no path that escapes the root;
- `/` is the only separator;
- every path is NFC UTF-8;
- canonical paths are unique (Unicode NFC collisions are errors, never silent merges);
- names match paths;
- every parent exists and is a directory;
- kinds are in the closed set;
- children belong to their parent;
- only directories have children;
- only symlink and junction nodes have a target.

## Forbidden data

The artifact MUST NOT carry:

- mtime, ctime, atime;
- owner, uid, gid, ACL, platform permissions, executable bit;
- absolute root, hostname, username;
- capture timestamp;
- terminal, TTY, theme, colors, icons, presentation styles, output indentation;
- file contents;
- scanner indexes or memory addresses.

## Canonicalization

Native source paths are converted through a single API (`artifact.Canonicalize`). Callers MUST NOT scatter `strings.ReplaceAll` / `filepath.Clean` for identity paths.

Windows sources treat both `\` and `/` as separators. POSIX sources treat only `/` as a separator.

Lexical `.` segments are dropped. `..` that would leave the root is rejected. Remaining segments are NFC-normalized.

If two distinct raw entries normalize to the same canonical path, canonicalization/build MUST fail with `canonical path collision` naming both entries. Dirloom never merges those nodes.
