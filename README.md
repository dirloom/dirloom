# Dirloom

Clean project trees for humans and tools.

Dirloom turns a real directory into a clean, deterministic, filterable and shareable project structure. It is a local-only, cross-platform CLI: no telemetry, no network calls, no project files modified unless you explicitly use `--output`.

```text
my-project/
├── src/
│   ├── components/
│   └── index.ts
├── package.json
└── README.md
```

Unlike a raw `tree` listing, the output is designed to be a reproducible artifact for terminals, Markdown documents, pull requests, CI pipelines and machine consumers.

## Installation

Download the archive for Windows, Linux or macOS from the GitHub release, extract `dirloom` (`dirloom.exe` on Windows), and place it on your `PATH`.

With Go 1.25.12 or newer:

```bash
go install github.com/dirloom/dirloom/cmd/dirloom@latest
```

For local development:

```bash
go build -o ./bin/dirloom ./cmd/dirloom
```

On PowerShell, use `-o .\bin\dirloom.exe` for the build output path.

## Quick start

```bash
# Inspect the current directory
dirloom

# Inspect another directory, at most three levels deep
dirloom ./src --depth 3

# Produce copy-ready Markdown
dirloom --format markdown

# Produce the versioned machine contract
dirloom --format json

# Write safely to a file (stdout remains empty)
dirloom --format markdown --output structure.md
```

PowerShell composition works naturally:

```powershell
dirloom --format markdown | Set-Clipboard
dirloom --style ascii > structure.txt
```

## CLI reference

```text
dirloom [directory] [flags]
```

`directory` defaults to the current working directory.

| Option | Meaning |
| --- | --- |
| `-d, --depth N` | Maximum depth. `0` prints only the root; omitted means unlimited. |
| `--dirs-only` | Include directories only. |
| `--hidden` | Include hidden entries that survive all other filters. |
| `--ignore PATTERN` | Add an exclusion. Repeat the option to add more rules. |
| `--no-default-ignore` | Disable built-in directory exclusions. |
| `--no-gitignore` | Do not load `.gitignore` files. |
| `--format text\|markdown\|json` | Select the public output contract. Default: `text`. |
| `--style unicode\|ascii` | Select the drawing style for text and Markdown. Default: `unicode`. |
| `-o, --output FILE` | Transactionally write to a file instead of stdout. |
| `-h, --help` | Show integrated help. |
| `-v, --version` | Show the version. |

`--style` is intentionally rejected when explicitly combined with `--format json`; JSON has no drawing style.

## Filtering

Dirloom evaluates every descendant in this fixed order. The first exclusion is final:

1. the `--output` destination;
2. built-in directory exclusions;
3. repeated `--ignore` rules;
4. scoped `.gitignore` rules;
5. hidden-entry visibility.

The explicit root itself is always retained. For example, `dirloom node_modules` still inspects that selected root.

### Built-in exclusions

The v0.1 list is deliberately conservative:

```text
.git  node_modules  .next  .nuxt  coverage  .cache  .turbo
```

`dist` and `build` are not excluded automatically. Use `--no-default-ignore` to disable the list.

### `--ignore` rules

Rules are case-sensitive and use `/` as their normalized separator:

```bash
dirloom --ignore node_modules --ignore "*.log" --ignore "src/**/generated?.go"
```

- A literal name matches that name anywhere.
- `*` and `?` match within one path segment.
- `**` crosses zero or more path segments.
- A relative pattern containing `/` is matched from the inspected root.
- A matched directory is pruned immediately.
- Commas are literal; each rule needs its own `--ignore` occurrence.
- Absolute and root-escaping patterns are rejected.
- There is no `!` re-inclusion syntax for CLI rules in v0.1.

### `.gitignore`

Dirloom reads `.gitignore` at the inspected root and in every traversed subdirectory, even outside a Git repository. Nested files are scoped to their own directory. Ordering, anchoring, wildcards, directory rules and `!` negations use Git-compatible last-match-wins semantics.

Dirloom intentionally does not read `.git/info/exclude`, files above the selected root, or the user's global Git excludes file. Use `--no-gitignore` to disable this layer.

As with Git matching, a malformed pattern is a non-match and does not make the directory scan fail.
Dirloom also follows Git's safety behavior by not following a symlink used as a working-tree `.gitignore` file.

## Output contracts

All formats are UTF-8 without BOM, contain LF line endings on every platform, and end with exactly one LF.

### Text

Unicode is always the default; Dirloom never changes styles by inspecting the terminal or redirection target.

```bash
dirloom --style unicode
dirloom --style ascii
```

### Markdown

Markdown wraps the selected text drawing in a fenced `text` block:

```bash
dirloom --format markdown --style ascii
```

### JSON schema v1

```json
{
  "schemaVersion": 1,
  "root": {
    "name": "src",
    "type": "directory",
    "children": [
      {
        "name": "index.ts",
        "type": "file"
      }
    ]
  }
}
```

Directories always have `children`, including empty directories. Files never do. Symlinks use `"type": "symlink"` and may expose their recorded target. Absolute paths, timestamps, permissions and other non-deterministic metadata are never included.

## Safe filesystem behavior

- The complete scan and sort finish before any output is written.
- A read error fails the whole operation; Dirloom never presents a partial tree as complete.
- Symlinks, Windows symbolic links and junctions are terminal nodes and are not traversed.
- An explicitly selected symlink root may be resolved once when it targets a directory.
- `--output` renders to a temporary file in the same directory, syncs it, then uses the platform's atomic replacement primitive.
- Existing output remains intact if safe replacement fails. Missing parent directories are never created.
- A symlink output destination is refused.

## Determinism

Directories are listed before terminal entries. Each group is sorted case-insensitively with deterministic Unicode casing, then by the original UTF-8 name and normalized relative path. Filesystem enumeration order, locale and platform do not control the output.

Dirloom preserves filesystem names exactly and does not silently normalize Unicode between NFC and NFD.

## Architecture

```text
CLI arguments
    → application service
    → filter-aware filesystem scanner
    → renderer-independent tree model
    → deterministic sort
    → text / Markdown / JSON renderer
    → stdout or transactional file output
```

The headless application service and model are independent from Cobra and from renderers, keeping the future `browse`, snapshot and diff interfaces able to reuse the same core.

See [docs/architecture.md](docs/architecture.md) for package boundaries and [docs/dependencies.md](docs/dependencies.md) for dependency decisions.

## Development

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/dirloom
```

CI repeats formatting, vet, tests and builds on Windows, Linux and macOS. It also runs the race detector, `golangci-lint` and `govulncheck` on Linux.

See [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a change.

## Release

Tags matching `v*` invoke GoReleaser. A `v0.1.0` tag builds:

```text
dirloom_Windows_x86_64.zip
dirloom_Windows_arm64.zip
dirloom_Linux_x86_64.tar.gz
dirloom_Linux_arm64.tar.gz
dirloom_Darwin_x86_64.tar.gz
dirloom_Darwin_arm64.tar.gz
```

Releases are created as drafts so maintainers can verify artifacts and checksums before publication.

## Roadmap

- v0.2: configuration and ergonomic presets;
- v0.3: explicit `dirloom browse` TUI reusing the same core;
- v0.4: deterministic AI-oriented presets and budgets, still local-only;
- v0.5: versioned snapshots and structural diffs;
- v0.6: structural annotations.

TUI, GUI, HTTP, MCP, cloud, telemetry, code analysis, watch mode and file-content summaries are intentionally outside v0.1.

## License

Dirloom is available under the [MIT License](LICENSE). Third-party notices are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
