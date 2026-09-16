# Clipboard and shell completions

Dirloom can copy a rendered tree to the operating system clipboard and generate on-demand completion scripts for Bash, Zsh, Fish and PowerShell.

These features are execution-time conveniences. They do not scan extra files, do not change the inspected project, and are not stored in `.dirloom.yaml`.

## Copy a render

`--copy` replaces stdout. The clipboard receives the same UTF-8 text that would have been written to the terminal or to `--output`: same characters, same line breaks, and the same final newline. Dirloom does not trim the payload, does not wrap it, and does not print `Copied`.

<!-- dirloom-clipboard-command:markdown -->
```bash
dirloom --format markdown --copy
```

Paste into GitHub, GitLab, or any Markdown editor.

<!-- dirloom-clipboard-command:json -->
```bash
dirloom --format json --copy
```

Paste into an editor, an API client, or an LLM prompt.

<!-- dirloom-clipboard-command:text -->
```bash
dirloom --copy
```

Paste into a terminal, a chat, or a ticket. With the current defaults (`icons: never` and `color: auto`) this is the same tree as `dirloom`, without ANSI color.

`--copy` and `--output` are mutually exclusive. The conflict is reported with exit code `2` before configuration is loaded and before the directory is scanned. Neither `output` nor `copy` can be set in YAML.

### Presentation

The clipboard is not treated as a pipe or CI destination.

| Channel | `--color auto` | `--icons auto` |
| --- | --- | --- |
| Interactive TTY | ANSI | Unicode |
| `--output`, pipe, CI | no ANSI | `never` |
| `--copy` | no ANSI | Unicode, like interactive text |

ANSI sequences are usually unwanted in paste targets, so automatic color is off. Unicode and Nerd glyphs often paste as-is into GitHub, chat tools, and tickets, so icons follow the renderer, the format, and the preset.

Explicit `--color always|never` and `--icons never|unicode|nerd` still apply to text output. Markdown, JSON and diagram sources stay canonical: no ANSI and no presentation glyphs.

`dirloom --icons unicode` and `dirloom --icons unicode --copy` therefore produce the same tree except for ANSI color.

Direct Markdown copy is the explicit path `--format markdown --copy`, including the existing documentation presets.

### Native backends

The injectable `Clipboard.Write([]byte)` boundary is byte-identical to the renderer. Operating systems may recode that UTF-8 for storage:

- Windows uses `CF_UNICODETEXT` (UTF-16). Dirloom retries a busy clipboard for a few seconds, then fails.
- macOS writes stdin to `/usr/bin/pbcopy` with no shell.
- Linux prefers `wl-copy` when `WAYLAND_DISPLAY` is set, then `xclip -selection clipboard -in`, then `xsel --clipboard --input`. WSL `clip.exe` is used only when WSL interop is detected (`WSL_INTEROP` or `/proc/sys/fs/binfmt_misc/WSLInterop`).

Linux clipboard tools are optional system packages. Install `wl-clipboard`, `xclip`, or `xsel` for your session. Commands are executed directly; Dirloom never interpolates a shell.

### Exit codes and silence

| Result | Exit code | stdout | stderr |
| --- | --- | --- | --- |
| Copied | `0` | empty | empty |
| Invalid usage (`--copy` with `--output`, bad flags) | `2` | empty | error |
| Clipboard backend missing or failed | `1` | empty | actionable error |

A clipboard failure never reprints the tree on stdout. `--copy` is ignored for `--help`, `--version`, and subcommands that do not render a tree.

### Privacy

The clipboard is a shared operating-system surface. Anything Dirloom copies can be read by other processes and by cloud clipboard sync. Do not copy trees that reveal internal names you would not paste into a ticket. Dirloom still does not read file contents.

### Troubleshooting

- **Linux: `clipboard copy is unavailable`** — install `wl-copy` (Wayland) or `xclip` / `xsel` (X11), or enable WSL interop if you intended `clip.exe`.
- **Windows: `open clipboard`** — another application is holding the clipboard; retry after a few seconds.
- **macOS: `run /usr/bin/pbcopy`** — the system clipboard helper failed; check Screen Recording / clipboard permissions only if a security tool intercepts stdin helpers.
- **Paste looks like mojibake** — the destination is not interpreting Unicode text. The payload Dirloom handed to the OS is UTF-8 at the renderer boundary; Windows stores UTF-16 text.

## Completions

<!-- dirloom-clipboard-command:completion-bash -->
```bash
dirloom completion bash
dirloom completion zsh
dirloom completion fish
dirloom completion powershell
```

Scripts are written to stdout, are deterministic, and end with a newline. Dirloom does not modify your shell profile. An invalid shell name or argument count returns `2`. A write failure returns `1`.

Add `--no-descriptions` to omit completion descriptions.

Generated scripts are not versioned in the repository. Homebrew generates Bash, Zsh, Fish and PowerShell completions from the installed binary. Scoop, Winget and GitHub archives document the same `dirloom completion` commands.

### Install examples

Bash (Linux, typical):

```bash
dirloom completion bash | sudo tee /etc/bash_completion.d/dirloom >/dev/null
```

Zsh (site functions):

```bash
dirloom completion zsh > "${fpath[1]}/_dirloom"
```

Fish:

```bash
dirloom completion fish > ~/.config/fish/completions/dirloom.fish
```

PowerShell (current user profile):

```powershell
dirloom completion powershell | Out-File -Encoding utf8 $PROFILE -Append
```

Prefer a dedicated file under your PowerShell profile directory if you do not want the script appended to `$PROFILE`.

### Semantic values

Completions cover public enumerated flags, including formats, presets, styles, themes, color and icon modes, diagram view and direction, and special values such as `unlimited`. The inspect root completes as directories. Custom theme paths still complete as files. `__complete` remains Cobra's internal protocol for tests and shell integration; it is not a user command.
