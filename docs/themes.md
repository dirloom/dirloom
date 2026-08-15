# Terminal colors, icons, and themes

Dirloom can add color and file-type icons to interactive text output without changing the inspected tree. The presentation layer is terminal-only: Markdown, tree JSON, diagnostics, help, errors, preset inspection, and theme inspection stay undecorated.

The canonical historical text artifact is always available with:

```bash
dirloom --color never --icons never
```

## Quick start

On an interactive terminal, the defaults enable terminal colors and portable Unicode icons automatically:

<!-- dirloom-theme-command:quick-start -->
```bash
dirloom
dirloom --theme midnight
dirloom --theme daylight
```

Select capabilities explicitly when you know the destination:

```bash
dirloom --color always --icons unicode
dirloom --color always --icons nerd --theme midnight
```

Use `theme explain` before selecting a built-in theme:

```bash
dirloom theme list
dirloom theme explain midnight
dirloom theme explain midnight --as json
```

## Color modes and `NO_COLOR`

`--color` accepts three values:

| Mode | Behavior |
| --- | --- |
| `auto` | Emit ANSI only for interactive text output on a usable TTY. This is the default. |
| `always` | Emit ANSI for text even through a pipe or explicit `--output`. |
| `never` | Never emit ANSI. Use this for canonical text artifacts. |

Automatic color is disabled when stdout is not a TTY, `--output` is used, `CI` is non-empty, or `TERM=dumb`. Dirloom selects truecolor when supported, ANSI-256 when `TERM` advertises it, and ANSI-16 otherwise. Forced color through a pipe or file uses truecolor when no terminal profile is available.

Dirloom follows the intent of [`NO_COLOR`](https://no-color.org/). A non-empty `NO_COLOR` disables ANSI from defaults and configuration. Only an explicit CLI `--color always` overrides it:

<!-- dirloom-theme-command:no-color -->
```bash
NO_COLOR=1 dirloom
NO_COLOR=1 dirloom --color always
```

`NO_COLOR` does not disable icons. Control them independently with `--icons`.

## Icon modes and Nerd Font fallback

`--icons` accepts four values:

| Mode | Behavior |
| --- | --- |
| `auto` | Use portable Unicode icons on an interactive TTY; otherwise use no icons. This is the default. |
| `unicode` | Force portable Unicode icons for text output. |
| `nerd` | Force Nerd Font icons for text output. |
| `never` | Use no icons. |

Dirloom does not detect installed fonts. Select Nerd Font only when your terminal uses a compatible font:

<!-- dirloom-theme-command:nerd -->
```bash
dirloom --icons nerd --theme midnight
```

If a selected Nerd token is absent, Dirloom uses its Unicode token; if both are absent, it prints the name without an icon. Dirloom never assumes that a glyph occupies one terminal cell.

The built-in catalog covers directories, ordinary files, symlinks, Markdown/README files, Go, TypeScript, JavaScript, JSON, YAML/TOML, and Dockerfiles. Nerd glyphs are a documented Material Design Icons subset distributed by Nerd Fonts. Dirloom does not bundle a font.

## Built-in themes

Built-in themes are compiled into the binary and require no runtime files:

| Theme | Intended background | Edge | Directory | File | Symlink | Accent |
| --- | --- | --- | --- | --- | --- | --- |
| `default` | Universal | `default` | `ansi:blue` | `default` | `ansi:magenta` | `ansi:cyan` |
| `midnight` | Dark (`#1A1B26` reference) | `#9AA5CE` | `#7AA2F7` | `#C0CAF5` | `#BB9AF7` | `#7DCFFF` |
| `daylight` | Light (`#FFFFFF` reference) | `#4B5563` | `#1D4ED8` | `#111827` | `#6B21A8` | `#0369A1` |

`default` uses the terminal's ANSI palette to remain usable across light and dark backgrounds. Dirloom does not detect the current background; choose `midnight` or `daylight` explicitly.

List and inspect exact compiled definitions without scanning:

<!-- dirloom-theme-command:explain -->
```bash
dirloom theme list --as json
dirloom theme explain daylight --as json
```

## Use a custom theme

Pass an absolute path or a path relative to the current working directory:

```bash
dirloom --theme ./.dirloom/themes/team.yaml
dirloom theme explain ./.dirloom/themes/team.yaml
```

This complete example inherits omitted values from `default`:

<!-- dirloom-theme-example:team -->
```yaml
schemaVersion: 1
name: team
description: Team terminal theme
appearance: dark

palette:
  edge: "#9AA5CE"
  directory: "#7AA2F7"
  file: default
  symlink: "#BB9AF7"
  accent: ansi:cyan

tokens:
  tree.edge:
    color: edge
    styles: [dim]

  node.directory:
    color: directory
    styles: [bold]
    icons:
      unicode: "▸"
      nerd: "󰉋"

  node.file:
    color: file
    icons:
      unicode: "·"
      nerd: "󰈔"

  node.symlink:
    color: symlink
    icons:
      unicode: "↗"
      nerd: "󰌷"

rules:
  - match:
      name: README.md
    color: accent
    styles: [bold]
    icons:
      unicode: "¶"
      nerd: "󰍔"

  - match:
      extension: .go
    color: accent
    icons:
      nerd: "󰟓"

  - match:
      glob: "src/generated/**"
    styles: [dim]

icons:
  spacing: 1
```

Custom themes cannot extend or include another file. Their only fallback is the built-in `default` theme.

## Theme schema reference

Every custom theme is one UTF-8 YAML document with `schemaVersion: 1`.

| Field | Required | Contract |
| --- | --- | --- |
| `schemaVersion` | yes | Integer `1`. |
| `name` | yes | Non-empty theme name used by diagnostics. |
| `description` | no | Human explanation. |
| `appearance` | yes | `universal`, `light`, or `dark`. It does not trigger background detection. |
| `palette` | no | Up to 64 named literal colors. |
| `tokens` | no | Base presentation for active semantic tokens. |
| `rules` | no | Up to 512 ordered match rules. |
| `icons.spacing` | no | Integer from `0` to `4`; default `1`. |

Colors accept:

- `default`;
- `#RRGGBB` hexadecimal values;
- `ansi:black`, `ansi:red`, `ansi:green`, `ansi:yellow`, `ansi:blue`, `ansi:magenta`, `ansi:cyan`, `ansi:white`;
- the same eight names prefixed with `ansi:bright-`.

Token and rule colors may reference a palette name. Palette values themselves are literal colors, not references.

Styles accept `bold`, `dim`, `italic`, and `underline`. The active schema-v1 tokens are:

```text
tree.edge
node.directory
node.file
node.symlink
```

A future token is ignored during inspection and reported as a non-blocking warning by `theme validate`.

Each rule defines exactly one matcher:

| Matcher | Meaning |
| --- | --- |
| `path` | Exact `/`-separated path relative to the inspected root. |
| `name` | Exact basename. |
| `glob` | Relative Dirloom glob; `**` crosses path segments. |
| `extension` | Exact extension including its leading dot, such as `.go`. |
| `type` | `directory`, `file`, or `symlink`. |

Names, paths, and extensions are case-sensitive on every platform. Duplicate matcher/value pairs are rejected.

Icons must contain valid UTF-8, occupy at most 64 bytes and four Unicode runes, and contain no terminal control, ANSI, line-break, or bidirectional-formatting character.

## Rule precedence

Dirloom selects at most one matching rule using this order:

```text
exact path
  > exact name
  > glob
  > extension
  > node type
  > base token
```

Within one priority, the first declared matching rule wins. A matching rule replaces only the fields it defines; remaining presentation comes from the base token and built-in fallback.

Path and glob matching uses normalized `/` separators relative to the inspected root. Rules affect presentation only: they cannot include, exclude, rename, reorder, or add a tree node.

## Persistent configuration

Visual preferences use the normal precedence: explicit CLI option, project configuration, user configuration, then built-in default.

<!-- dirloom-theme-config-example:presentation -->
```yaml
schemaVersion: 1

presentation:
  color: auto
  icons: unicode
  theme: midnight
```

Use a project-local custom theme:

```yaml
schemaVersion: 1

presentation:
  theme: .dirloom/themes/team.yaml
```

A theme path in configuration is relative to that configuration file. After resolving symlinks, the selected file must remain inside the configuration directory. This boundary prevents a repository file from reading an arbitrary external theme. CLI paths may be absolute and are resolved from the current working directory.

Use `theme: null` to neutralize an inherited theme and return to `default`. The selected theme is the only theme file Dirloom reads; masked paths are syntax-checked but not opened. Presets do not select color, icons, or a theme.

See [Persistent configuration](configuration.md) for source discovery and all schema fields.

## Inspect and validate themes

Theme commands never load Dirloom configuration and never scan a directory:

<!-- dirloom-theme-command:validate -->
```bash
dirloom theme list
dirloom theme explain midnight
dirloom theme explain ./.dirloom/themes/team.yaml --as json
dirloom theme validate ./.dirloom/themes/team.yaml
dirloom theme validate ./.dirloom/themes/team.yaml --as json
```

`theme list` returns only built-ins in lexical order. `theme explain` returns a normalized definition and source. `theme validate` returns success with warnings for inactive future tokens; schema or security violations fail.

The JSON contracts for list, explanation, and validation have their own `schemaVersion: 1`, stable ordering, and non-null arrays. `--as` accepts `text` or `json`. Configuration-source flags are rejected because they would have no effect.

Use `config explain` to inspect the selected presentation values and their provenance:

```bash
dirloom config explain
dirloom config explain --as json
```

The report describes requested `auto` modes rather than depending on the diagnostic command's current TTY. For Markdown and JSON tree formats, `color`, `icons`, and `theme` appear as inactive.

## Terminal and pipeline behavior

For reproducible pipeline output, select a canonical format explicitly:

<!-- dirloom-theme-command:pipeline -->
```bash
dirloom --color never --icons never > structure.txt
dirloom --format markdown --output structure.md
dirloom --format json --output structure.json
```

Inherited presentation settings are inactive for Markdown and JSON, so team or user preferences cannot contaminate machine artifacts. Explicit active visual options with those formats are rejected with exit code `2`; `--color never` and `--icons never` are accepted.

Forced presentation is available when a text file intentionally needs ANSI or icons:

```bash
dirloom --color always --icons unicode --output colored-tree.txt
```

The default auto modes emit the historical neutral bytes through pipes, redirections, CI, and `--output`.

## Security and trust

Theme files are declarative data. Dirloom does not execute commands, interpolate variables, evaluate templates, include another file, access the network, select a directory, or choose an output destination.

The loader rejects unknown fields, duplicate keys, multiple YAML documents, anchors, aliases, merge keys, and custom tags. Files are limited to 1 MiB. Palette and rule limits bound compilation work. Dirloom generates all ANSI sequences itself and escapes dangerous controls in names only when presentation is active.

All theme and mode errors occur before filesystem scanning. Because output is rendered in memory and file output remains transactional, an invalid theme leaves stdout and an existing `--output` destination untouched.

## Troubleshooting

### Icons render as boxes

Your terminal font does not contain the requested Nerd glyphs. Use `--icons unicode` or `--icons never`, or configure a Nerd Font outside Dirloom. Dirloom cannot detect the installed font.

### Colors are absent

Run `dirloom config explain` to inspect the selected mode. Check whether output is piped, `--output` is active, `CI` is set, `TERM=dumb`, or `NO_COLOR` is non-empty. Use `--color always` only when ANSI is intentional.

### A project theme path is rejected

Keep the YAML file under the directory that contains the selected configuration file. Symlinks that resolve outside that directory are rejected. Use an explicit CLI path only when you intentionally trust an external file.

### An option is rejected with Markdown or JSON

Presentation is text-only. Remove the active option or use `--color never --icons never`. Inherited values are already inactive and do not need an override.

### Validation reports an unknown token

The theme targets a semantic token not active in schema v1. Inspection uses the built-in fallback and validation succeeds with a warning. Remove the token for a warning-free theme or use a Dirloom version that supports it.

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Theme operation or rendering succeeded, including validation with warnings. |
| `1` | Filesystem permission, read, terminal setup, or output-write failure. |
| `2` | Invalid option, theme path, YAML, schema, value, or flag combination. |

Errors go to stderr. Usage and validation errors leave stdout empty.

## Compatibility and evolution

Theme YAML, theme command JSON, and configuration diagnostic JSON each declare an independent `schemaVersion: 1`. The tree JSON schema remains unchanged.

Built-in names, palettes, base icon mapping, and rule behavior are public v0.2 contracts. Future changes are recorded in the changelog and evaluated under Semantic Versioning. A future schema may activate more semantic tokens, but a schema-v1 theme cannot modify tree membership or canonical output.

Dirloom does not currently download themes or fonts, discover custom themes by name, detect Nerd Fonts or terminal backgrounds, compose themes, emit OSC-8 hyperlinks, or use `LS_COLORS`, `CLICOLOR`, or `FORCE_COLOR`.
