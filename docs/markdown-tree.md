# Semantic Markdown trees

## Overview

`markdown-tree` renders the inspected structure as a native nested Markdown list. It is designed for READMEs, pull requests, issue descriptions and documentation systems where a semantic list is easier to read and transform than an ASCII drawing.

The existing `markdown` format remains unchanged. It wraps Dirloom's text drawing in a fenced `text` block and is the better choice when you need the familiar tree connectors. `markdown-tree` is a separate output contract and never enables itself implicitly.

Both formats are deterministic projections of the same inspected tree. Neither reads file contents, changes traversal order or adds nodes.

## Quick start

<!-- dirloom-markdown-tree-command:quick-start -->
```bash
dirloom --format markdown-tree
```

Given a small project, the result follows this shape:

<!-- dirloom-markdown-tree-output:basic -->
```markdown
- `project/`
  - `empty/`
  - `src/`
    - `link` -> `../shared`
    - `index.ts`
  - `README.md`
```

Paste the output directly into a Markdown document or write it transactionally:

```bash
dirloom . --format markdown-tree --depth 4 --output docs/project-tree.md
```

## Output contract

Every node is one unordered-list item:

- two spaces represent one nesting level;
- directories end with `/`;
- files use their exact displayed name;
- a symlink with a recorded target uses `` `name` -> `target` ``;
- a symlink without a target uses `` `name` [symlink] ``;
- labels and targets use Markdown code spans;
- the document is UTF-8 without a BOM, uses LF on every platform and ends with exactly one LF.

Directory order, terminal-entry order, filtering and depth come from Dirloom's canonical tree model. The renderer does not sort or inspect the filesystem.

## Names and Markdown safety

Code spans prevent ordinary filename characters such as `*`, `_`, `[` and `#` from becoming Markdown syntax. Dirloom selects a longer backtick delimiter when a name itself contains backticks.

Characters that could break the document or make a label deceptive are represented deterministically:

- backslashes become `\\`;
- tab, line feed and carriage return become `\t`, `\n` and `\r`;
- other control, line-separator and bidirectional-control characters use `\u{XXXX}`;
- a name made entirely of spaces uses one `\x20` escape per space.

These escapes affect only this representation. They do not rename a node or alter the JSON and text contracts.

## Choose between Markdown formats

| Need | Recommended format |
| --- | --- |
| Preserve the traditional tree drawing | `markdown` |
| Produce a native nested list | `markdown-tree` |
| Feed a versioned model to automation | `json` |
| Display an interactive terminal tree | `text` |

Use `markdown-tree` when the destination benefits from real list semantics, including screen-reader navigation or downstream Markdown processing. Use `markdown` when connector alignment is more important than document structure.

## Filtering and depth

All existing inspection controls work before rendering:

```bash
dirloom . \
  --format markdown-tree \
  --dirs-only \
  --depth 4 \
  --ignore "**/generated"
```

`--dirs-only` removes terminal entries from the model. `--depth 0` produces a list containing only the root. Ignore rules and `.gitignore` behavior are identical across all formats.

## Persistent configuration

Set the format for a project or user configuration like any other scalar:

<!-- dirloom-markdown-tree-config:project -->
```yaml
schemaVersion: 1

defaults:
  depth: 4
  format: markdown-tree
```

The normal priority applies: explicit CLI option, project configuration, user configuration, then the built-in default. Use `dirloom config explain` to inspect the effective value and its source.

The `unicode` and `ascii` drawing styles do not apply to semantic lists. An inherited style remains valid but is reported as inactive. Explicitly combining `--style` with `--format markdown-tree` is rejected so that a command never appears to apply an option that has no effect.

## Presets

Presets and formats remain independent. The `docs` and `ai` presets continue to select the existing fenced `markdown` format. Override them explicitly when a semantic list is more useful:

```bash
dirloom . --preset docs --format markdown-tree
dirloom . --preset ai --format markdown-tree
```

The preset still controls depth, filtering and directory visibility. The explicit format changes only the final projection.

## Determinism and pipelines

`markdown-tree` performs no terminal detection, emits no ANSI sequences and does not depend on locale, theme or font. The same canonical model produces the same bytes on Windows, Linux and macOS.

For a durable generated document, prefer `--output` over shell redirection. Dirloom renders the full document before transactionally replacing the destination, and the destination is excluded from its own scan.

## Limitations

The initial contract intentionally provides no:

- automatic links to repository files;
- headings, tables or HTML spans;
- colors, terminal icons or Nerd Font glyphs;
- generated descriptions or file-content summaries;
- theme-specific rules;
- inference from an `.md` output extension.

These omissions keep the format portable and prevent a repository layout from being confused with generated documentation content.

## Troubleshooting

### The output still contains tree connectors

You selected `markdown`, which produces a fenced text drawing. Use `--format markdown-tree` for a nested list.

### `--style` is rejected

Semantic lists have no drawing style. Remove `--style`; filtering, depth and presets remain available.

### A name contains escape sequences

The filesystem name contains a backslash, control character or direction-changing character that is unsafe or ambiguous in portable Markdown. The escape is deterministic and represents the original structural value without injecting Markdown syntax.

### The generated file appears in a later scan

Only the active `--output` destination is automatically excluded. Ignore older generated files explicitly if they should not appear.

## Compatibility

`markdown-tree` is an additive v0.2 output contract. The meanings and bytes of `text`, `markdown` and JSON schema v1 do not change. Future changes that alter semantic-list bytes or escaping rules must be documented as public contract changes and evaluated under Semantic Versioning.
