# Contextual help

Dirloom ships three local help levels. All of them are compiled into the binary: no network, no browser, and no configuration or filesystem scan.

## Three help levels

```bash
dirloom --help
dirloom <command> --help
dirloom help <topic>
```

| Level | Command | Answers |
| --- | --- | --- |
| Global discovery | `dirloom --help` | What Dirloom is and which commands exist |
| Command help | `dirloom theme --help` or `dirloom help theme` | How to use that command |
| Concept help | `dirloom help icons` | How a capability works |

`dirloom help <name>` resolves a public command first, then a compiled topic. Command names keep precedence over topic names.

## Available topics

```bash
dirloom help topics
```

| Topic | Subject |
| --- | --- |
| `colors` | Terminal color behavior |
| `concepts` | Logical index of topics |
| `configuration` | Configuration resolution and precedence |
| `diagrams` | Mermaid, Graphviz and D2 exports |
| `examples` | Common command recipes |
| `filters` | Depth, ignore rules and visibility |
| `formats` | Text, Markdown, JSON and diagram formats |
| `icons` | Unicode, Nerd Font and automatic icons |
| `output` | stdout, clipboard and transactional files |
| `presets` | Built-in project-tree presets |
| `themes` | Built-in and custom terminal themes |

```bash
dirloom help <topic>
```

## Optional-value flags

Only flags with a natural boolean-like form get an implicit value:

```text
--icons[=MODE]
--color[=MODE]
```

`--icons` without a value means `--icons=auto`. `--color` without a value means `--color=auto`. Color modes are `never`, `always`, and `auto`. Icon modes are `never`, `unicode`, `nerd`, and `auto`. The built-in default remains `icons: never` when the flag is omitted.

These forms stay equivalent:

```text
--icons
--icons auto
--icons=auto
--icons unicode
--icons=unicode
```

The same pattern applies to `--color`. A following flag is not consumed as a mode (`--icons --help`, `--icons --depth 3`). Normalization stops at `--`, so tokens after the terminator are never rewritten as Dirloom flags. An unrecognized separated token such as `--icons banana` remains an invalid icon mode rather than a directory named `banana`.

`--theme`, `--preset`, `--format`, `--style`, `--diagram-view`, `--diagram-direction`, and `--config` still require an explicit value.

## Error guidance

Invalid enumerated CLI values print the accepted set and a targeted help command:

```text
Error: invalid value "foobar" for --icons

Valid values:
  never
  unicode
  nerd
  auto

Run 'dirloom help icons' for details.
```

Unknown help targets may suggest a nearby command or topic. Usage errors keep exit code `2`. Runtime failures keep exit code `1`.

## help vs explain

`help` teaches how a capability works. `explain` reports a concrete resolved state or definition:

```bash
dirloom help configuration
dirloom config explain

dirloom help presets
dirloom preset explain ai

dirloom help themes
dirloom theme explain vivid
```

## Shell completion

`dirloom help <TAB>` completes public commands and the compiled topics on Bash, Zsh, Fish, and PowerShell. Generate scripts with `dirloom completion` as described in [Clipboard and shell completions](clipboard-and-completions.md).
