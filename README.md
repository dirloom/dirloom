# Dirloom

Clean project trees for humans and AI.

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

Published status is independent of each package manager. GitHub Releases is always the artifact source. See [Distribution and trusted releases](docs/distribution.md).

<!-- dirloom-distribution-status -->
```text
RELEASE STATUS
Released at GitHub tag v0.1.1; v0.2.0 composing

DISTRIBUTION STATUS
GitHub     ✅ v0.1.1
Scoop      ✅ v0.1.1
Homebrew   ⏳
Winget     ⏳
```

### Windows with Winget

```powershell
winget install Dirloom.Dirloom
```

### Windows with Scoop

```powershell
scoop bucket add dirloom https://github.com/dirloom/scoop-bucket
scoop install dirloom
```

The official bucket selects the x64 or ARM64 release, verifies its SHA-256 checksum and adds `dirloom` to `PATH`. Upgrade later with `scoop update dirloom`.

### macOS and Linux with Homebrew

```bash
brew install --cask dirloom/tap/dirloom
```

### Release archives

Download the archive for Windows, Linux or macOS from [GitHub Releases](https://github.com/dirloom/dirloom/releases), extract `dirloom` (`dirloom.exe` on Windows), and place it on your `PATH`. Verify `checksums.txt`, SBOMs and attestations as described in [Distribution](docs/distribution.md).

### Install with Go

With Go 1.25.12 or newer:

```bash
go install github.com/dirloom/dirloom/cmd/dirloom@latest
```

### Build from source

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

# Copy Markdown ready to paste into GitHub
dirloom --format markdown --copy

# Start from a documented built-in preset
dirloom --preset compact

# Produce the versioned machine contract
dirloom --format json

# Emit a README-ready Mermaid structure graph
dirloom --format mermaid --output docs/structure.mmd

# Write safely to a file (stdout remains empty)
dirloom --format markdown --output structure.md

# Generate a shell completion script
dirloom completion bash
```

PowerShell composition still works, but `--copy` is the native clipboard path:

```powershell
dirloom --format markdown --copy
dirloom --style ascii > structure.txt
```

See [Clipboard and shell completions](docs/clipboard-and-completions.md) and [Practical use cases and examples](docs/use-cases.md) for filtering recipes, documentation and AI workflows, CI artifacts, JSON processing, ecosystem-specific commands and current product limitations.

## Persistent configuration

Dirloom can load shared project settings from `.dirloom.yaml` and personal defaults from your operating system's configuration directory.

<!-- dirloom-config-example:readme -->
```yaml
schemaVersion: 1

defaults:
  depth: 4
  format: text

ignore:
  - generated/**
```

Explicit CLI options take priority over the project file, which takes priority over the user file and built-in defaults. Inspect the effective values and their sources with:

```bash
dirloom config explain
```

Keep CI independent of personal settings while retaining the committed project configuration:

```bash
dirloom . --no-user-config --format json --output structure.json
```

See [Persistent configuration](docs/configuration.md) for discovery rules, the complete schema, monorepo and CI recipes, diagnostics, security boundaries and troubleshooting.

## Built-in presets

Dirloom includes deterministic presets for common workflows:

```bash
dirloom --preset docs       # Markdown for documentation and reviews
dirloom --preset compact    # Shallow directory-only overview
dirloom --preset monorepo   # Workspace topology without dist/build noise
dirloom --preset ai         # Markdown source structure for AI workflows
```

Inspect the exact built-in definition without scanning a directory:

```bash
dirloom preset explain ai
dirloom preset explain ai --as json
```

Explicit options override individual preset values. Use `--preset none` to neutralize a preset inherited from configuration. See [Built-in presets](docs/presets.md) for exact definitions, precedence, YAML activation, diagnostics, security boundaries and recipes.

## Terminal presentation

Interactive text uses automatic color and keeps icons disabled until requested. The independent `vivid` theme uses a two-tone neon system: structural roles color text while technical kinds color glyphs:

```bash
dirloom --theme vivid
dirloom --theme vivid --icons nerd
dirloom theme classify README.md --theme vivid
```

Pipes, redirects, CI, and `--output` stay neutral in automatic mode. Fenced Markdown, semantic Markdown, and JSON never contain ANSI or presentation icons. Reproduce canonical historical text explicitly with:

```bash
dirloom --color never --icons never
```

Dirloom respects `NO_COLOR`; only explicit CLI `--color always` overrides it. See [Terminal colors, icons, and themes](docs/themes.md) for the public theme schema and [Semantic catalog](docs/catalog.md) for the 256 matchers, 96 kinds, 16 roles, and classification diagnostics.

## CLI reference

```text
dirloom [directory] [flags]
```

`directory` defaults to the current working directory.

| Option | Meaning |
| --- | --- |
| `-d, --depth N\|unlimited` | Maximum depth. `0` prints only the root; `unlimited` removes an inherited limit. |
| `--dirs-only` | Include directories only. |
| `--hidden` | Include hidden entries that survive all other filters. |
| `--ignore PATTERN` | Add an exclusion. Repeat the option to add more rules. |
| `--no-default-ignore` | Disable built-in directory exclusions. |
| `--no-gitignore` | Do not load `.gitignore` files. |
| `--format text\|markdown\|markdown-tree\|json\|mermaid\|graphviz\|d2` | Select the public output contract. Default: `text`. `--format dot` is an alias for `graphviz`. |
| `--style unicode\|ascii` | Select the drawing style for text and fenced Markdown. Default: `unicode`. |
| `--diagram-view structure` | Select the diagram projection. Default: `structure`. Active only for diagram formats. |
| `--diagram-direction top-down\|left-right` | Select diagram flow. Default: `top-down`. |
| `--diagram-max-nodes N\|unlimited` | Fail if an explicit diagram node budget is exceeded. Default: unlimited. |
| `--config FILE` | Use an explicit project configuration file instead of automatic discovery. |
| `--no-user-config` | Skip personal configuration while retaining project configuration. |
| `--no-config` | Disable user and project configuration files. |
| `--preset docs\|compact\|monorepo\|ai\|none` | Select a built-in preset or neutralize an inherited preset. |
| `--color never\|always\|auto` | Control ANSI color for text output. Default: `auto`. |
| `--icons never\|unicode\|nerd\|auto` | Control presentation icons for text output. Default: `never`. |
| `--theme NAME\|PATH` | Select `default`, `midnight`, `daylight`, `vivid`, or a local YAML theme. |
| `-o, --output FILE` | Transactionally write to a file instead of stdout. |
| `--copy` | Copy the rendered tree to the clipboard instead of stdout. Mutually exclusive with `--output`. |
| `-h, --help` | Show integrated help. |
| `-v, --version` | Show the version. |

`--style` is intentionally rejected when explicitly combined with `--format json`, `--format markdown-tree`, `--format mermaid`, `--format graphviz` or `--format d2`; those contracts have no drawing style.

`dirloom config explain [directory]` reports source status, the active preset, effective values and provenance. Add `--as json` for the versioned machine-readable diagnostic.

`dirloom theme classify <path>` performs one bounded `Lstat` and explains the real entry type, semantic kind, roles, winning matcher, and resolved theme style without reading file content or scanning recursively.

`dirloom completion bash|zsh|fish|powershell` writes a deterministic completion script to stdout and does not modify your shell profile. See [Clipboard and shell completions](docs/clipboard-and-completions.md).

## Filtering

Dirloom evaluates every descendant in this fixed order. The first exclusion is final:

1. the `--output` destination;
2. built-in directory exclusions;
3. preset and explicit ignore rules merged from user, project and CLI layers;
4. scoped `.gitignore` rules;
5. hidden-entry visibility.

The explicit root itself is always retained. For example, `dirloom node_modules` still inspects that selected root.

### Built-in exclusions

The v0.1 list is deliberately conservative:

```text
.git  node_modules  .next  .nuxt  coverage  .cache  .turbo
```

`dist` and `build` are not excluded automatically. Use `--no-default-ignore` to disable the list.

### Explicit ignore rules

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
- There is no `!` re-inclusion syntax for explicit rules.

Configuration ignore lists are additive: user rules come first, followed by project and CLI rules. Exact duplicates are removed without changing the first rule's position or source.

### `.gitignore`

Dirloom reads `.gitignore` at the inspected root and in every traversed subdirectory, even outside a Git repository. Nested files are scoped to their own directory. Ordering, anchoring, wildcards, directory rules and `!` negations use Git-compatible last-match-wins semantics.

Dirloom intentionally does not read `.git/info/exclude`, files above the selected root, or the user's global Git excludes file. Use `--no-gitignore` to disable this layer.

As with Git matching, a malformed pattern is a non-match and does not make the directory scan fail.
Dirloom also follows Git's safety behavior by not following a symlink used as a working-tree `.gitignore` file.

## Output contracts

All formats are UTF-8 without BOM, contain LF line endings on every platform, and end with exactly one LF. Stdout, `--output` and `--copy` share that invariant.

### Text

Unicode tree drawing remains the default. Terminal presentation is automatic only on a usable interactive TTY; non-interactive output preserves the historical neutral bytes.

```bash
dirloom --style unicode
dirloom --style ascii
dirloom --color never --icons never
```

### Markdown

Markdown wraps the selected text drawing in a fenced `text` block:

```bash
dirloom --format markdown --style ascii
```

### Semantic Markdown

Use a native nested list when the destination should understand the tree as Markdown structure:

```bash
dirloom --format markdown-tree
```

```markdown
- `my-project/`
  - `src/`
    - `index.ts`
  - `README.md`
```

This format is deterministic, contains no ANSI or terminal icons, and does not use `--style`. See the [semantic Markdown guide](docs/markdown-tree.md) for its escaping, symlink and compatibility contracts.

### Mermaid, Graphviz and D2

Canonical structure graphs are available as DSL sources. Dirloom does not render images:

```bash
dirloom --format mermaid
dirloom --format graphviz
dirloom --format d2
```

See the [graphical export guide](docs/graph-exports.md) for the `structure` contract, direction, node budget, alias `dot`, and escaping rules.

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
- `--copy` is an exclusive destination: it never writes a file and never prints the tree on stdout.
- Existing output remains intact if safe replacement fails. Missing parent directories are never created.
- A symlink output destination is refused.

## Determinism

Directories are listed before terminal entries. Each group is sorted case-insensitively with deterministic Unicode casing, then by the original UTF-8 name and normalized relative path. Filesystem enumeration order, locale and platform do not control the output.

Canonical output preserves filesystem names exactly and does not silently normalize Unicode between NFC and NFD. When terminal presentation is active, dangerous controls are escaped in the displayed projection without changing the tree model.

## Architecture

```text
CLI arguments
    → configuration discovery and resolution
    → built-in preset expansion
    → presentation resolution and theme validation
    → application service
    → filter-aware filesystem scanner
    → renderer-independent tree model
    → deterministic sort
    → canonical text / fenced Markdown / semantic Markdown / JSON renderer
    → optional semantic catalog classification and terminal-only text projection
    → stdout or transactional file output
```

The configuration resolver, headless application service and model are independent from Cobra and from renderers, keeping future `browse`, snapshot and diff interfaces able to reuse the same core.

See [docs/architecture.md](docs/architecture.md) for package boundaries and [docs/dependencies.md](docs/dependencies.md) for dependency decisions.

## Development

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/dirloom
```

CI repeats formatting, `go mod tidy` with a clean diff, `go mod verify`, vet, tests and builds on Windows, Linux and macOS. It also runs the race detector, `golangci-lint`, `govulncheck`, official Mermaid/Graphviz/D2 parser checks, completion-script syntax, GoReleaser `check`, and a snapshot that verifies the 13-artifact inventory.

See [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a change.

## Release

Dirloom uses a protected `release/vX.Y.Z` branch for release composition.
`main` tracks the last published tag until the release candidate merges.
See [Release workflow](docs/release-workflow.md).

Tags matching `v*` invoke GoReleaser and produce a GitHub Release **draft**. Maintainers verify the 13 artifacts (6 archives, 6 SBOMs, `checksums.txt`), attestations, and checksums, then publish. Package-manager pull requests open only after publication.

The six official archives remain:

```text
dirloom_Windows_x86_64.zip
dirloom_Windows_arm64.zip
dirloom_Linux_x86_64.tar.gz
dirloom_Linux_arm64.tar.gz
dirloom_Darwin_x86_64.tar.gz
dirloom_Darwin_arm64.tar.gz
```

See [Distribution](docs/distribution.md) and [Release workflow](docs/release-workflow.md).

## Roadmap

The voted product sequence builds from the deterministic v0.1 foundation:

```text
v0.1 CORE → v0.2 ACCESSIBILITY → v0.3 PRESENTATION → v0.4 INTELLIGENCE → v0.5 CHANGE
```

- v0.2: install, `--copy`, completions, trusted GitHub releases (Release Done is independent of Winget merge);
- v0.3: visual richness — catalog, themes, semantic files, colors and styles, closing the gap with eza;
- later: fingerprints, snapshots, structural diff, then scaffold and Architecture Packs.

`dirloom browse` is scheduled after the v0.3 presentation increment. See the [product documentation](docs/product/README.md) for the vision, product principles, functional specification, glossary and the [voted strategic roadmap](docs/product/roadmap.md).

## License

Dirloom is available under the [MIT License](LICENSE). Third-party notices are listed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
