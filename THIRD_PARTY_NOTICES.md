# Third-party notices

Dirloom includes the following direct runtime dependencies:

- `github.com/spf13/cobra` v1.10.2 — Apache License 2.0.
- `github.com/git-pkgs/gitignore` v1.2.0 — MIT License.
- `golang.org/x/sys` v0.47.0 — BSD 3-Clause License.
- `golang.org/x/term` v0.45.0 — BSD 3-Clause License.
- `go.yaml.in/yaml/v3` v3.0.4 — MIT and Apache License 2.0.

Transitive module metadata is pinned in `go.sum`. Full corresponding license texts are distributed in `LICENSES` and every release archive. The Go project licenses for `x/sys` and `x/term` are recorded separately as `LICENSES/BSD-3-Clause-x-sys.txt` and `LICENSES/BSD-3-Clause-x-term.txt`.

## Nerd Font and Material Design Icons glyph metadata

Dirloom's optional Nerd Font strings use code points assigned to [Material Design Icons](https://github.com/Templarian/MaterialDesign) through the [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) mapping. Material Design Icons are distributed under Apache License 2.0; Nerd Fonts records that provenance in its glyph catalog and license audit. The Apache 2.0 text is included as `LICENSES/Apache-2.0.txt`.

Catalog v1 embeds only these glyph strings and their semantic mapping:

| Use | Glyph | Code point |
| --- | --- | --- |
| file | `󰈔` | `U+F0214` |
| source family | `󰅩` | `U+F0169` |
| manifest / JSON | `󰘦` | `U+F0626` |
| data family | `󰆼` | `U+F01BC` |
| document / YAML / TOML | `󰈙` | `U+F0219` |
| media family | `󰉏` | `U+F024F` |
| archive family | `󰀼` | `U+F003C` |
| font family | `󰛖` | `U+F06D6` |
| binary family | `󰆍` | `U+F018D` |
| directory | `󰉋` | `U+F024B` |
| symlink | `󰌷` | `U+F0337` |
| Go | `󰟓` | `U+F07D3` |
| Rust | `󱘗` | `U+F1617` |
| Python | `󰌠` | `U+F0320` |
| JavaScript | `󰌞` | `U+F031E` |
| TypeScript | `󰛦` | `U+F06E6` |
| HTML | `󰌝` | `U+F031D` |
| CSS | `󰌜` | `U+F031C` |
| Markdown | `󰍔` | `U+F0354` |
| PDF | `󰈦` | `U+F0226` |
| PNG | `󰸭` | `U+F0E2D` |
| package archive | `󰏗` | `U+F03D7` |
| Dockerfile / Containerfile | `󰡨` | `U+F0868` |

No font, font binary, SVG, image, or network-delivered asset is bundled. Users must install a compatible font independently before choosing `--icons nerd`; Dirloom otherwise supports Unicode fallback or no icon.