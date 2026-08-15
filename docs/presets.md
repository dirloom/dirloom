# Built-in presets

Dirloom presets are named, deterministic combinations of existing inspection options. They provide useful defaults for common workflows without hiding the effective configuration or limiting explicit command-line control.

Dirloom includes four presets: `docs`, `compact`, `monorepo`, and `ai`. They are compiled into the binary, perform no additional filesystem or network access, and never select an output file.

## Quick start

<!-- dirloom-preset-command:quick-start -->
```bash
dirloom --preset docs
dirloom preset explain docs
dirloom config explain . --preset docs
```

Use a preset as a starting point, then override individual values when needed:

```bash
dirloom --preset compact --dirs-only=false --depth 6
```

## Preset catalog

| Preset | Best for | Depth | Entries | Format | Additional exclusions |
| --- | --- | ---: | --- | --- | --- |
| `docs` | Documentation and architecture reviews | `4` | Files and directories | Markdown | None |
| `compact` | A short structural overview | `3` | Directories only | Text | None |
| `monorepo` | Workspace and package topology | `4` | Directories only | Text | `**/dist`, `**/build` |
| `ai` | Structural context for AI-assisted work | `4` | Files and directories | Markdown | `**/dist`, `**/build`, `*.map` |

Every preset also sets:

- hidden entries excluded;
- Unicode drawing style;
- built-in directory exclusions enabled;
- scoped `.gitignore` processing enabled.

Preset ignore rules are additive. They do not remove exclusions inherited from user or project configuration.

### `docs`

`docs` produces a bounded Markdown tree that can be pasted into a README, architecture note, pull request, or review document.

<!-- dirloom-preset-command:docs -->
```bash
dirloom . --preset docs
dirloom . --preset docs --output structure.md
```

Equivalent explicit options when file-backed configuration is disabled:

```bash
dirloom . --no-config --depth 4 --dirs-only=false --hidden=false \
  --format markdown --style unicode \
  --no-default-ignore=false --no-gitignore=false
```

The preset does not choose `--output`; writing remains an explicit caller decision.

### `compact`

`compact` shows a shallow directory-only map. Use it to understand the main shape of an unfamiliar project without listing terminal files.

<!-- dirloom-preset-command:compact -->
```bash
dirloom . --preset compact
dirloom ./src --preset compact --depth 5
```

Equivalent explicit options without file-backed configuration:

```bash
dirloom . --no-config --depth 3 --dirs-only --hidden=false \
  --format text --style unicode \
  --no-default-ignore=false --no-gitignore=false
```

### `monorepo`

`monorepo` focuses on workspace topology and removes repeated `dist` and `build` directories at any depth. It does not assume a particular package manager or workspace layout.

<!-- dirloom-preset-command:monorepo -->
```bash
dirloom . --preset monorepo
dirloom packages --preset monorepo --depth 5
```

Equivalent explicit options without file-backed configuration:

```bash
dirloom . --no-config --depth 4 --dirs-only --hidden=false \
  --format text --style unicode \
  --no-default-ignore=false --no-gitignore=false \
  --ignore "**/dist" --ignore "**/build"
```

The preset does not merge multiple project configuration files. Normal project discovery still loads only the nearest `.dirloom.yaml` inside the Git boundary.

### `ai`

`ai` produces bounded Markdown with source filenames while excluding common build directories and source maps. It is useful as structural context for a coding conversation or local agent workflow.

<!-- dirloom-preset-command:ai -->
```bash
dirloom . --preset ai
dirloom ./src --preset ai --depth 6
```

Equivalent explicit options without file-backed configuration:

```bash
dirloom . --no-config --depth 4 --dirs-only=false --hidden=false \
  --format markdown --style unicode \
  --no-default-ignore=false --no-gitignore=false \
  --ignore "**/dist" --ignore "**/build" --ignore "*.map"
```

The preset does not read file contents, calculate token budgets or size statistics, compress repeated subtrees, or call an LLM.

## Precedence and overrides

Preset selection follows the same precedence as other scalar configuration:

```text
explicit --preset
  > project preset
  > user preset
  > no preset
```

Only one preset is active. Dirloom expands it in its source layer before applying explicit values from that layer. Explicit values in the same or a higher layer therefore win.

```bash
# compact normally hides files; this invocation includes them
dirloom . --preset compact --dirs-only=false

# docs normally uses Markdown; this invocation uses tree JSON
dirloom . --preset docs --format json
```

Preset exclusions are inserted before explicit exclusions from the same layer. All exclusion layers remain additive and exact duplicates retain their first origin.

## Use a preset from configuration

Project and user configuration accept one optional top-level `preset` field.

<!-- dirloom-preset-config-example:project -->
```yaml
schemaVersion: 1
preset: docs

defaults:
  depth: 6

ignore:
  - generated/**
```

This project uses `docs`, overrides its depth with `6`, and adds a project-specific exclusion.

Use `null` to disable a preset inherited from user configuration without disabling the remaining user values:

<!-- dirloom-preset-config-example:reset -->
```yaml
schemaVersion: 1
preset: null
```

For one invocation, use the CLI reset value:

```bash
dirloom . --preset none
```

`none` is not a preset name. It is accepted by `--preset` only, so `dirloom preset explain none` fails with exit code `2`.

`--no-config` disables file-backed configuration, not explicit CLI presets:

```bash
dirloom . --no-config --preset ai
```

See [Persistent configuration](configuration.md) for source discovery, the complete YAML schema, precedence, diagnostics, and security boundaries.

## Inspect a preset

Inspect the compiled definition without loading configuration or scanning a directory:

<!-- dirloom-preset-command:explain -->
```bash
dirloom preset explain ai
dirloom preset explain ai --as json
```

The text output is optimized for reading. The JSON output is a separate versioned contract:

<!-- dirloom-preset-json-example:ai -->
```json
{
  "schemaVersion": 1,
  "name": "ai",
  "description": "Produce a concise Markdown structure for AI-assisted workflows.",
  "defaults": {
    "depth": 4,
    "dirsOnly": false,
    "hidden": false,
    "format": "markdown",
    "style": "unicode"
  },
  "filters": {
    "useDefaultIgnores": true,
    "useGitignore": true
  },
  "ignore": [
    "**/dist",
    "**/build",
    "*.map"
  ]
}
```

The `ignore` array is always present, including as `[]` for presets without additional exclusions.

Configuration-source flags are deliberately rejected by preset inspection because they would have no effect:

```text
--config
--no-config
--no-user-config
```

## Inspect the effective configuration

`preset explain` describes an intrinsic definition. `config explain` resolves the selected directory, configuration layers, active preset, explicit overrides, and final provenance:

```bash
dirloom config explain .
dirloom config explain . --preset ai --depth 6
dirloom config explain . --as json
```

A value supplied by a preset is identified as, for example, `project preset ai`. An explicit override from the same project or from the CLI is reported with its direct source instead.

Use `config explain` when output differs from the intrinsic preset definition because user settings, project settings, or CLI options may have replaced individual values or added exclusions.

## Common recipes

### Keep documentation settings in the repository

Commit this project configuration and keep the destination explicit in scripts:

```yaml
schemaVersion: 1
preset: docs
```

```bash
dirloom . --output structure.md
```

### Use a personal compact default

Place this in the operating system's Dirloom user configuration:

```yaml
schemaVersion: 1
preset: compact
```

Repositories can override or reset it without modifying the user file.

### Create a reproducible CI artifact

Ignore personal configuration, select the preset explicitly, and keep the machine format explicit:

```bash
dirloom . --no-user-config --preset monorepo \
  --format json --output structure.json
```

### Prepare AI context without repository settings

```bash
dirloom . --no-config --preset ai
```

Dirloom emits structure only. Add the specific code or documentation required by the task separately.

## Determinism and security

Presets do not weaken the existing execution model:

- definitions are compiled into the Dirloom binary;
- names and values are validated before scanning;
- no preset executes commands, expands variables, evaluates templates, or loads another file;
- no preset performs network access;
- no preset selects a directory or output destination;
- scanning, filtering, sorting, rendering, and transactional output retain their existing contracts.

Treat a project-selected preset as repository input: it can change which names appear in an inspection. Use `--preset none` to ignore only that selection, or `--no-config` to disable both file-backed layers.

## Troubleshooting

### A preset name is rejected

Names are lowercase and case-sensitive: `docs`, `compact`, `monorepo`, and `ai`. `none` is accepted only by `--preset` as a reset value.

### An explicit option appears to be ignored

Run `dirloom config explain <directory>` and check both the active preset and each value's origin. An explicit CLI option wins; an omitted option does not.

### Additional files are missing

Preset exclusions are additive with user, project, and CLI exclusions. `.gitignore` and hidden-entry filtering also remain active. Use `config explain` to inspect every effective rule and its origin.

### `preset explain` rejects a configuration flag

The command reports a compiled definition and intentionally does not resolve files. Remove `--config`, `--no-config`, or `--no-user-config`. Use `config explain` to inspect a preset in a real configuration context.

### Exit codes

| Code | Meaning |
| ---: | --- |
| `0` | Preset selected or explained successfully. |
| `1` | Filesystem, rendering, or output error. |
| `2` | Invalid name, value, argument, configuration, or flag combination. |

Errors occur before rendering. Stdout stays empty, and an existing `--output` file remains unchanged.

## Compatibility and evolution

The four names and their documented definitions are public v0.2 behavior. Changes must be evaluated under SemVer and recorded in the changelog.

The optional YAML `preset` field remains part of configuration `schemaVersion: 1`. Older strict binaries that predate presets reject the field rather than guessing its meaning.

Preset explanation JSON has its own `schemaVersion: 1`. Configuration diagnostic JSON remains version `1` with additive preset and provenance fields. Tree JSON is unchanged.

Future custom presets, inheritance, composition, themes, AI statistics, and context compression are separate capabilities and are not accepted by the current CLI or configuration schema.
