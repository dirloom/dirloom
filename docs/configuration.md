# Persistent configuration

Dirloom can keep repeatable inspection settings in YAML while preserving explicit command-line control. Commit a project configuration when a team needs the same output, keep personal defaults in the operating system's configuration directory, and use CLI options for one-off changes.

Built-in presets can also provide a named starting point. See [Built-in presets](presets.md) for their exact definitions and usage recipes.

Configuration is optional. If no configuration file exists, Dirloom behaves exactly as it does with its built-in defaults.

## Quick start

Create `.dirloom.yaml` at the root of your repository:

<!-- dirloom-config-example:quick-start -->
```yaml
schemaVersion: 1

defaults:
  depth: 4
  format: text
  style: unicode

ignore:
  - generated/**
  - vendor/cache/**
```

Run Dirloom normally:

```bash
dirloom
```

Check which files and values Dirloom resolved:

```bash
dirloom config explain
```

Command-line options remain authoritative. This command uses depth `2` without changing the file:

```bash
dirloom --depth 2
dirloom --style ascii
```

## Configuration sources

Dirloom resolves four layers, from highest to lowest priority:

```text
explicit CLI option
  > project configuration
  > user configuration
  > built-in default
```

An omitted CLI option is not an override. Explicit Boolean forms such as `--hidden=false` and `--dirs-only=false` can turn off values inherited from a file.

### Project configuration

The project file is named `.dirloom.yaml`.

Inside a Git worktree, Dirloom finds the nearest file between the inspected directory and the worktree boundary. It recognizes both regular `.git` directories and the `.git` files used by worktrees and submodules. Only the nearest project file is loaded; parent project files are not merged.

Outside Git, Dirloom checks only `.dirloom.yaml` inside the inspected directory. This prevents unrelated files in parent directories from changing an inspection.

Use a specific file instead of automatic discovery when necessary:

```bash
dirloom . --config configs/docs.yaml
```

A relative `--config` path is resolved from the current working directory. The file must exist.

### User configuration

The user file is optional and follows the operating system's standard configuration location:

| Platform | Path |
| --- | --- |
| Windows | `%AppData%\dirloom\config.yaml` |
| Linux | `${XDG_CONFIG_HOME:-$HOME/.config}/dirloom/config.yaml` |
| macOS | `$HOME/Library/Application Support/dirloom/config.yaml` |

Use it for personal preferences that should apply across projects. Do not commit this file to individual repositories.

Skip the user layer while retaining project configuration:

```bash
dirloom . --no-user-config
```

Disable both file-backed layers:

```bash
dirloom . --no-config
```

`--no-config` cannot be combined with `--config` or `--no-user-config`.

## How values are combined

Scalar values use normal precedence: the highest layer that explicitly defines a value wins. This includes `false`, `0`, and an explicit unlimited depth.

`preset` is also a scalar. The highest user, project, or CLI selection wins, and Dirloom applies the selected preset before explicit values from that same layer. A higher explicit value can therefore override one preset setting without redefining the others.

Use `preset: null` in YAML or `--preset none` on the CLI to neutralize an inherited preset while retaining other inherited configuration values.

The `ignore` list is additive because personal, project, and one-off exclusions serve different purposes. Dirloom appends patterns in this order:

```text
user preset → user ignore
  → project preset → project ignore
  → CLI preset → repeated --ignore options
```

Only the winning preset contributes rules; masked presets are skipped. Preset rules are inserted before explicit exclusions from the same layer. Exact duplicates are removed while the first occurrence and its source are preserved. The initial schema does not support negation or resetting inherited explicit exclusions.

## Schema reference

The file must contain exactly one YAML document and declare `schemaVersion: 1`.

| Field | Type | Built-in default | Description |
| --- | --- | --- | --- |
| `schemaVersion` | integer | required | Configuration contract version. Only `1` is supported. |
| `preset` | `docs`, `compact`, `monorepo`, `ai`, or `null` | no preset | Select a built-in preset, or explicitly neutralize an inherited preset with `null`. |
| `defaults.depth` | integer or `null` | unlimited | Maximum depth. `0` prints only the root; `null` removes an inherited limit. |
| `defaults.dirsOnly` | Boolean | `false` | Include directories only. |
| `defaults.hidden` | Boolean | `false` | Include hidden entries that survive the other filters. |
| `defaults.format` | `text`, `markdown`, or `json` | `text` | Select the public output contract. |
| `defaults.style` | `unicode` or `ascii` | `unicode` | Select the drawing style for text and Markdown. It is inactive for JSON. |
| `filters.useDefaultIgnores` | Boolean | `true` | Apply Dirloom's built-in directory exclusions. |
| `filters.useGitignore` | Boolean | `true` | Apply scoped `.gitignore` files encountered during traversal. |
| `ignore` | sequence of strings | empty | Add explicit exclusions relative to the inspected root. |

`directory` and `output` are intentionally not configurable. A repository configuration cannot redirect an inspection or cause Dirloom to write a file.

### Complete example

<!-- dirloom-config-example:complete -->
```yaml
schemaVersion: 1
preset: docs

defaults:
  depth: 6
  dirsOnly: false
  hidden: false
  format: markdown
  style: ascii

filters:
  useDefaultIgnores: true
  useGitignore: true

ignore:
  - generated/**
  - vendor/cache/**
  - "*.log"
```

Configuration files are limited to 1 MiB. Unknown fields, duplicate keys, multiple YAML documents, aliases, anchors, merge keys, and custom tags are rejected. Dirloom does not expand environment variables and does not support includes or templates.

## Inspect effective configuration

Use `config explain` before troubleshooting output or committing a shared file:

```bash
dirloom config explain .
```

The text report includes:

- the absolute inspected root;
- user and project source paths;
- source status: `loaded`, `missing`, `disabled`, or `unavailable`;
- the active preset or explicit reset and its origin;
- every effective scalar and its origin;
- every effective ignore pattern and its origin;
- settings that are currently inactive.

Use JSON for automation:

```bash
dirloom config explain . --as json
```

The diagnostic document has its own `schemaVersion: 1`. It is separate from the tree JSON schema and may contain absolute configuration paths because it is an explicit local diagnostic, not a portable tree artifact.

You can apply temporary inspection options to the explanation itself:

```bash
dirloom config explain . --depth 3 --format markdown
```

Values supplied by a preset identify both their configuration layer and preset name. Values explicitly replaced in that layer report the direct layer as their origin.

## Common recipes

### Share team exclusions

Commit a project file with stable repository-specific noise:

<!-- dirloom-config-example:team -->
```yaml
schemaVersion: 1

ignore:
  - generated/**
  - reports/private/**
```

Every contributor and CI job gets the same exclusions unless an explicit CLI option changes a scalar setting.

### Keep personal defaults

Place personal presentation preferences in the user configuration file:

<!-- dirloom-config-example:user -->
```yaml
schemaVersion: 1

defaults:
  depth: 3
  style: ascii
```

A project can override either value without modifying the user file.

If the user file selects a preset, a project can neutralize only that selection:

```yaml
schemaVersion: 1
preset: null
```

The remaining explicit user values and exclusions continue to participate in normal resolution.

### Configure a monorepo

Place the shared file at the repository root. Dirloom finds it when a nested directory is inspected:

```bash
dirloom packages/payments/src
```

Add a closer `.dirloom.yaml` inside `packages/payments` only when that package needs a complete project-level replacement. Dirloom loads the nearest project file; it does not merge it with the repository-root file.

### Make CI independent of personal settings

Commit `.dirloom.yaml`, disable the user layer, and keep output explicit:

```bash
dirloom . --no-user-config --format json --output structure.json
```

For a fully explicit configuration path:

```bash
dirloom . --no-user-config --config .dirloom.yaml --format json --output structure.json
```

PowerShell uses the same Dirloom arguments:

```powershell
dirloom . --no-user-config --config .dirloom.yaml --format json --output structure.json
if ($LASTEXITCODE -ne 0) { throw "Dirloom failed" }
```

### Remove an inherited depth limit

In a project file:

<!-- dirloom-config-example:unlimited -->
```yaml
schemaVersion: 1

defaults:
  depth: null
```

For one invocation:

```bash
dirloom --depth unlimited
```

### Override inherited Booleans

Boolean flags accept explicit `false` values:

```bash
dirloom --dirs-only=false --hidden=false
dirloom --no-default-ignore=false --no-gitignore=false
```

The second command re-enables the corresponding filter when a configuration file disabled it.

## Security and trust

Treat project configuration as repository input. Dirloom validates it before scanning and provides `--no-config` for untrusted or diagnostic runs.

A configuration file can change which names appear in output, including hidden entries, but it cannot:

- execute commands;
- read file contents through an include or template;
- interpolate environment variables;
- select a different inspection root;
- choose an output destination;
- modify the project or the configuration file.

Preset definitions are compiled into Dirloom and have the same boundaries. They do not load additional files, perform network access, or select an output destination.

Secrets do not belong in Dirloom configuration. The schema has no secret-bearing fields.

## Troubleshooting

### The project file is not loaded

Run `dirloom config explain <directory>` and inspect the project source. Inside Git, confirm that `.dirloom.yaml` is between the inspected directory and its nearest worktree boundary. Outside Git, place it directly in the inspected directory or use `--config`.

### A field is reported as unknown

Check spelling and nesting against the schema reference. Dirloom rejects unknown fields instead of silently ignoring a likely typo. Fields planned in the product roadmap are not accepted until the corresponding capability ships.

### A value comes from the wrong source

Use `config explain` to inspect the active preset and provenance. Remember that an explicit CLI value wins, the nearest project file replaces any more distant project file, and ignore patterns accumulate across layers. Use `preset explain <name>` to compare the effective result with the intrinsic preset definition.

### An ignore pattern is rejected

Patterns must be relative to the inspected root. Absolute paths, drive-qualified paths, empty segments, and `..` segments are invalid. Use `/` as the normalized separator.

### Dirloom cannot read a file

An automatically discovered file that exists but cannot be read is an error. Fix its permissions or disable the relevant layer. Missing automatic files are normal and do not produce an error.

### Exit codes

| Code | Meaning for configuration |
| ---: | --- |
| `0` | Resolution succeeded; optional automatic files may be absent. |
| `1` | Filesystem or permission error. |
| `2` | Invalid YAML, schema, value, pattern, flag combination, or explicit path. |

Configuration errors are written to stderr before rendering begins. Stdout stays empty, and an existing `--output` file remains unchanged.

## Compatibility

Configuration is a public Dirloom contract. Schema version `1` keeps the documented meanings and types stable. A future optional field may be introduced with release notes, but an older strict binary will reject a field it does not recognize. A change to an existing field's meaning or type requires a new schema version.

The optional `preset` field was added before the first v0.2 release and remains part of schema version `1`. The preset explanation JSON and configuration diagnostic JSON are separate versioned contracts; the tree JSON schema is unchanged.

An older Dirloom binary fails clearly when it encounters an unsupported schema version or field. It never guesses how to interpret a newer contract. The CLI, configuration schema, diagnostic JSON, and tree JSON are versioned and tested independently.
