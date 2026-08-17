# Terminal colors, icons, and themes

Dirloom can decorate interactive text without changing the inspected tree. Colors, icons, and themes are a terminal-only projection: fenced Markdown, semantic Markdown, JSON trees, diagnostics, help, and errors remain canonical and undecorated.

The built-in presentation defaults are:

```text
color: auto
icons: never
theme: default
```

Use this command whenever historical neutral text bytes are required:

```bash
dirloom --color never --icons never
```

## Quick start

Color is automatic on an eligible terminal. Icons are opt-in:

<!-- dirloom-theme-command:quick-start -->
```bash
dirloom
dirloom --icons unicode
dirloom --theme vivid --icons nerd
```

Inspect a built-in definition without scanning a directory:

```bash
dirloom theme list
dirloom theme explain vivid
dirloom theme explain vivid --as json
```

## Color modes and `NO_COLOR`

| Mode | Behavior |
| --- | --- |
| `auto` | Emit ANSI only for text written to an eligible interactive TTY. This is the default. |
| `always` | Force ANSI for text, including a pipe or explicit `--output`. |
| `never` | Never emit ANSI. |

Automatic color is disabled for a pipe, redirection, `--output`, non-empty `CI`, or `TERM=dumb`. Dirloom selects truecolor when advertised, ANSI-256 for a `256color` terminal, and ANSI-16 otherwise. Forced color through a non-terminal destination uses truecolor when no profile can be detected.

A non-empty [`NO_COLOR`](https://no-color.org/) disables ANSI selected by defaults or configuration. Only an explicit CLI `--color always` overrides it:

<!-- dirloom-theme-command:no-color -->
```bash
NO_COLOR=1 dirloom
NO_COLOR=1 dirloom --color always
```

`NO_COLOR` does not affect icons.

## Icon modes and fallback

| Mode | Behavior |
| --- | --- |
| `never` | Emit no presentation icons. This is the default. |
| `unicode` | Force portable Unicode glyphs for text. |
| `nerd` | Force Nerd Font glyphs for text. |
| `auto` | Use Unicode on an eligible TTY; otherwise emit no icons. |

Dirloom never assumes that a Nerd Font is installed and never selects Nerd glyphs automatically.

<!-- dirloom-theme-command:nerd -->
```bash
dirloom --icons nerd --theme vivid
```

For each semantic kind, Nerd mode falls back to its Unicode glyph, then to no glyph. Dirloom does not assume a fixed display width. The semantic catalog and glyph provenance are documented in [Semantic catalog](catalog.md).

## Built-in themes

The four built-ins use the same semantic catalog. Switching themes changes only presentation bindings.

| Theme | Intended background | Edge | Directory | File | Symlink | Accent |
| --- | --- | --- | --- | --- | --- | --- |
| `default` | Universal | `default` | `ansi:blue` | `default` | `ansi:magenta` | `ansi:cyan` |
| `midnight` | Dark (`#1A1B26`) | `#9AA5CE` | `#7AA2F7` | `#C0CAF5` | `#BB9AF7` | `#7DCFFF` |
| `daylight` | Light (`#FFFFFF`) | `#4B5563` | `#1D4ED8` | `#111827` | `#6B21A8` | `#0369A1` |
| `vivid` | Dark (`#10131A`) | `#7A869E` | `#44D7FF` | `#F1F5F9` | `#F38BFF` | `#8B7CFF` |

`default` delegates to the terminal ANSI palette. Dirloom does not detect the terminal background; select `midnight`, `daylight`, or `vivid` explicitly.

<!-- dirloom-theme-command:explain -->
```bash
dirloom theme list --as json
dirloom theme explain daylight --as json
```

### Vivid palette

`vivid` is an independent two-tone neon theme for dark terminals. Structural roles control text while technical kinds control icon color. This separation makes a Go test, generated source, and ordinary Go source immediately distinguishable without changing their technical glyph.

Base presentation:

| Binding | Color | Binding | Color |
| --- | --- | --- | --- |
| `edge` | `#7A869E` | `file` | `#F1F5F9` |
| `directory` | `#44D7FF` | `symlink` | `#F38BFF` |
| `accent` | `#8B7CFF` | Reference background | `#10131A` |

Role-driven text colors:

| Role | Color | Role | Color |
| --- | --- | --- | --- |
| `security` | `#FF5C7C` | `generated` | `#A1AAC0` |
| `vendor` | `#8793AA` | `test` | `#B6F36B` |
| `contract` | `#FFE066` | `lock` | `#FFB86B` |
| `infra` | `#FF7A5C` | `config` | `#FFD166` |
| `executable` | `#5CFFA9` | `archive` | `#F4B860` |
| `media` | `#FF75D8` | `data` | `#45E0FF` |
| `source` | `#66F0C0` | `document` | `#C9A7FF` |
| `tooling` | `#AAB8D0` | `generic` | `#DEE6F2` |

Kind-driven icon colors:

| Kind family | Color | Kind family | Color |
| --- | --- | --- | --- |
| `directory` | `#00D7FF` | `symlink` | `#FF6BEE` |
| `source` | `#00FFD1` | `manifest` | `#FFB000` |
| `data` | `#00D4FF` | `document` | `#A78BFA` |
| `media` | `#FF4FB8` | `archive` | `#FF9F43` |
| `binary` | `#2EF2A1` |  |  |

`security` and `contract` are bold and underlined. `test`, `infra`, and `executable` are bold; `generated` and `vendor` are dimmed. Other roles keep their base text style. Icon spans remain free of text styles.

Every built-in `vivid` color reaches at least 4.5:1 contrast against `#10131A`, including decorative icon colors. Ratios are tested from the sRGB values. The theme does not set a background and still requires explicit `--icons unicode`, `--icons nerd`, or `--icons auto` to display glyphs.

## Use a custom theme

A CLI reference may be absolute or relative to the current working directory:

```bash
dirloom --theme ./.dirloom/themes/team.yaml
dirloom theme explain ./.dirloom/themes/team.yaml
```

This example uses the public theme schema v1. Omitted values inherit from `default`.

<!-- dirloom-theme-example:team -->
```yaml
schemaVersion: 1
catalogVersion: 1
name: team
description: Team terminal theme
appearance: dark

palette:
  edge: "#7A869E"
  file: "#F1F5F9"
  source: "#66F0C0"
  generated: "#A1AAC0"
  image: "#FF75D8"

tokens:
  tree.edge:
    color: edge
    styles: [dim]
  node.directory:
    color: file
    styles: [bold]
  node.file:
    color: file
  node.symlink:
    color: file

kinds:
  source:
    iconColor: source
  media.image:
    iconColor: image

roles:
  source:
    color: source
  generated:
    color: generated
    styles: [dim]
  contract:
    color: source
    styles: [bold, underline]

rules:
  - match:
      glob: "internal/generated/**"
    role: generated

  - match:
      path: "tools/codegen.go"
    kind: source.go
    role: tooling
    color: source
    iconColor: source
    styles: [bold]

  - match:
      name: README.md
    role: contract

icons:
  spacing: 1
```

Custom themes cannot add catalog entries, extend another theme, or include another file.

## Public theme schema v1

`schemaVersion: 1` is the first public theme contract shipped with Dirloom v0.2. It replaces the pre-release prototype, also labeled `1`, without a compatibility loader. `catalogVersion: 1` is mandatory and distinguishes the public format. A prototype file without it fails with an actionable usage error.

Theme YAML and `.dirloom.yaml` both use a `schemaVersion`, but they are independent contracts.

| Field | Required | Contract |
| --- | --- | --- |
| `schemaVersion` | yes | Integer `1`. |
| `catalogVersion` | yes | Integer `1`. |
| `name` | yes | Non-empty diagnostic name. |
| `description` | no | Human explanation. |
| `appearance` | yes | `universal`, `light`, or `dark`; it does not detect or set a background. |
| `palette` | no | Up to 128 named literal colors. |
| `tokens` | no | Bind the four base renderer tokens. |
| `kinds` | no | Up to 256 bindings for catalog kinds or parent families. |
| `roles` | no | Up to 64 structural-role bindings. |
| `rules` | no | Up to 512 ordered path/name/glob/extension/type rules. |
| `icons.spacing` | no | Integer `0` through `4`; default `1`. |

Files are UTF-8, contain exactly one YAML document, and are limited to 1 MiB.

### Colors, styles, and icons

A color is `default`, `#RRGGBB`, one of the 16 ANSI names below, or a palette reference. Palette entries themselves must be literals.

```text
ansi:black          ansi:bright-black
ansi:red            ansi:bright-red
ansi:green          ansi:bright-green
ansi:yellow         ansi:bright-yellow
ansi:blue           ansi:bright-blue
ansi:magenta        ansi:bright-magenta
ansi:cyan           ansi:bright-cyan
ansi:white          ansi:bright-white
```

Styles are `bold`, `dim`, `italic`, and `underline`.

`color`, `iconColor`, `styles`, and `icons` are resolved property by property:

- an absent property inherits;
- `styles: []` clears inherited text styles;
- `iconColor: null` makes the icon follow the effective text color;
- `icons.unicode: null` or `icons.nerd: null` removes the inherited glyph in that channel at the binding where it is declared;
- icon spans receive color only; bold, dim, italic, and underline apply to the text span.

Glyphs must be valid UTF-8, at most 64 bytes and four runes, and contain no ANSI, control, line-break, or bidirectional-formatting character.

### Tokens, kinds, and roles

The base tokens are:

```text
tree.edge
node.directory
node.file
node.symlink
```

Kinds describe technical identity, such as `source.go`, `data.json`, or `document.markdown`. Parent bindings apply before more specific kind bindings. Roles describe structural function, such as `test`, `generated`, or `contract`. The first classified role with a theme binding becomes the visual role.

Unknown token, kind-binding, or role-binding keys are ignored with stable validation warnings. Unknown `kind:` or `role:` actions in a rule are errors because an explicit action must never become silently inactive.

See [Semantic catalog](catalog.md) for all role names, matching precedence, and examples.

### Rules and complete precedence

A rule defines exactly one matcher:

| Matcher | Contract |
| --- | --- |
| `path` | Exact `/`-separated path relative to the inspected root. |
| `name` | Exact basename. |
| `glob` | Relative Dirloom glob; `**` crosses path segments. |
| `extension` | Exact extension including its leading dot. |
| `type` | `directory`, `file`, or `symlink`. |

User rules are case-sensitive on every platform. Exact path wins over exact name, then glob, extension, and node type. At the same priority, the first declared matching rule wins. Duplicate matcher/value pairs are rejected.

Resolution for the winning classification is:

```text
catalog classification
  → winning rule selects any replacement kind or role
  → base node token and catalog kind glyph
  → theme bindings from parent kind to specific kind
  → effective structural-role binding
  → direct rule color, iconColor, styles, and glyph overrides
```

A rule may set `kind:` and `role:` together. Replacing the kind recalculates its parent chain; replacing the role selects one explicit visual role. Rules affect presentation only and cannot include, exclude, rename, reorder, retype, or add a canonical node.

## Persistent configuration

Presentation values use the normal precedence: explicit CLI option, project configuration, user configuration, then built-in default.

<!-- dirloom-theme-config-example:presentation -->
```yaml
schemaVersion: 1

presentation:
  color: auto
  icons: unicode
  theme: vivid
```

A project-local custom theme uses an explicit relative path:

```yaml
schemaVersion: 1

presentation:
  theme: .dirloom/themes/team.yaml
```

A theme path from configuration is relative to that configuration file. After symlink resolution, it must remain inside the configuration directory. CLI theme paths may be absolute. Only the winning theme is read; masked references are syntax-checked but not opened.

`theme: null` resets an inherited theme to `default` without changing color or icon mode. Presets never set presentation values. See [Persistent configuration](configuration.md).

Inspect the effective values and their provenance with:

```bash
dirloom config explain
dirloom config explain --as json
```

The diagnostic reports the requested `auto` modes rather than depending on its current TTY. For fenced Markdown, semantic Markdown, and JSON tree formats, it reports `color`, `icons`, and `theme` as inactive; semantic Markdown also reports `style` as inactive.

## Inspect and validate themes

Theme commands do not load project or user configuration.

<!-- dirloom-theme-command:validate -->
```bash
dirloom theme list
dirloom theme explain vivid
dirloom theme explain ./.dirloom/themes/team.yaml --as json
dirloom theme validate ./.dirloom/themes/team.yaml
dirloom theme validate ./.dirloom/themes/team.yaml --as json
```

`theme list`, `theme explain`, and `theme validate` have independent JSON schema v1 contracts. Theme explanation identifies `themeSchemaVersion: 1` and reports catalog version and counts without dumping all 256 matchers. Validation returns stable warnings such as `unknown-token`, `unknown-kind-binding`, and `unknown-role-binding`.

Configuration-source flags are rejected because these commands intentionally ignore configuration.

## Inspect a real classification

`theme classify` performs one bounded filesystem metadata lookup. It loads and compiles the selected theme first, resolves the target within `--root`, calls `Lstat`, and does not follow the final symlink, traverse children, or read file content.

<!-- dirloom-theme-command:classify -->
```bash
dirloom theme classify README.md
dirloom theme classify src/main.go --theme vivid
dirloom theme classify ./src --root . --theme vivid --as json
```

The default options are `--root .`, `--theme default`, and `--as text`. A relative target is resolved below the root; an absolute target must remain inside it. Parent symlink escapes are rejected, while the final symlink itself is classified as `symlink` even if its target is missing or outside the root.

Text output is undecorated and reports the relative path, real type, kind, roles, winning matcher, theme, effective text/icon styles, and origins. JSON uses an independent `schemaVersion: 1`, never exposes an absolute target or custom-theme path, and emits arrays rather than `null`.

A missing target, traversal attempt, unsupported filesystem type, invalid theme, or invalid flag is a usage error (`2`). Permission and I/O failures are runtime errors (`1`). Theme validation happens before target access.

## Terminal and pipeline behavior

<!-- dirloom-theme-command:pipeline -->
```bash
dirloom --color never --icons never > structure.txt
dirloom --format markdown --output structure.md
dirloom --format markdown-tree --output project-tree.md
dirloom --format json --output structure.json
```

Fenced Markdown, semantic Markdown, and JSON never contain ANSI or presentation icons. Inherited presentation settings are inactive. Explicit active visual flags with those formats are rejected; neutral `--color never` and `--icons never` remain valid.

Use forced presentation only for a text destination that intentionally accepts it:

```bash
dirloom --color always --icons unicode --output colored-tree.txt
```

## Security and trust

Themes are declarative local data. They cannot execute commands, interpolate variables, evaluate templates, include another file, access the network, select a scan root, or choose an output destination.

The strict loader rejects unknown fields, duplicate keys, multiple documents, anchors, aliases, merge keys, custom tags, invalid palette references, unsafe glyphs, duplicate rules, and exceeded limits. ANSI sequences are generated by Dirloom. Dangerous terminal controls in displayed names are escaped only in the decorated projection.

All presentation errors occur before the project scan. Rendering finishes in memory and `--output` remains transactional, so an error leaves stdout and an existing destination untouched.

## Troubleshooting

### Icons are absent

The default is `icons: never`. Use `--icons unicode`, `--icons nerd`, or `--icons auto` on an eligible TTY. A theme never enables icons by itself.

### Nerd glyphs render as boxes

Use `--icons unicode` or `--icons never`, or configure a compatible Nerd Font outside Dirloom. Dirloom cannot detect the installed font.

### Colors are absent

Run `dirloom config explain`. Check for a pipe, `--output`, non-empty `CI`, `TERM=dumb`, or `NO_COLOR`. Use `--color always` only when ANSI is intentional.

### A project theme path is rejected

Keep the theme under the directory containing its configuration file. Symlinks resolving outside that boundary are rejected. Use an explicit CLI path only when you intentionally trust an external file.

### A visual option is rejected with Markdown or JSON

Presentation is text-only. Remove the active option or select `--color never --icons never`. Inherited values are already inactive and require no override.

### A binding produces a warning

Unknown token, kind-binding, and role-binding keys are future-facing warnings. Remove the key for a warning-free v1 theme. An unknown kind or role used as a rule action is an error.

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Rendering or theme operation succeeded, including validation with warnings. |
| `1` | Permission, read, terminal-setup, or output-write failure. |
| `2` | Invalid option, target, path, YAML, schema, value, or flag combination. |

Usage and validation errors write only to stderr.

## Compatibility and evolution

The theme file, semantic catalog, theme list, theme explanation, theme validation, theme classification, configuration diagnostic, and tree JSON each own an independent version constant. They currently all expose version `1` where applicable; one contract can evolve without silently changing another.

The public theme schema v1 and catalog v1 ship together in Dirloom v0.2. After release, renaming or removing a kind, changing a catalog matcher, or changing a built-in definition requires a changelog entry and Semantic Versioning review.

Dirloom does not currently download themes, catalogs, or fonts; discover custom themes by name; detect Nerd Fonts or terminal backgrounds; compose themes; inspect file contents for classification; emit OSC-8 hyperlinks; or consume `LS_COLORS`, `CLICOLOR`, or `FORCE_COLOR`.