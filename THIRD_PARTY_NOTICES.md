# Third-party notices

Dirloom includes the following direct runtime dependencies:

- `github.com/spf13/cobra` v1.10.2 — Apache License 2.0.
- `github.com/git-pkgs/gitignore` v1.2.0 — MIT License.
- `golang.org/x/sys` v0.47.0 — BSD 3-Clause License.
- `golang.org/x/term` v0.45.0 — BSD 3-Clause License.
- `go.yaml.in/yaml/v3` v3.0.4 — MIT and Apache License 2.0.

Transitive module metadata is pinned in `go.sum`. Full corresponding license texts are distributed in `LICENSES` and every release archive. The Go project licenses for `x/sys` and `x/term` are recorded separately as `LICENSES/BSD-3-Clause-x-sys.txt` and `LICENSES/BSD-3-Clause-x-term.txt`.

## Nerd Font glyph metadata

Dirloom does not bundle a font, font binary, `.ttf`, `.otf`, SVG, image, or logo asset. Optional `--icons nerd` output embeds only Unicode characters/codepoints. Users must install a compatible [Nerd Font](https://github.com/ryanoasis/nerd-fonts) independently. The compiled catalog is the Nerd mapping; Dirloom never inspects the installed font.

Pinned Nerd Fonts registry:

```text
Nerd Fonts: v3.5.1
repository: ryanoasis/nerd-fonts
tag: v3.5.1
glyphnames.json retrieved: 2026-09-18
```

Nerd Fonts is the patched-font project that assigns those codepoints. The original icon drawings come from the upstream projects below. v0.3.3 is a governed multi-collection catalog; it does not claim that every glyph is Material Design Icons.

| Collection | Nerd Fonts prefix | Original project | License in this repository |
| --- | --- | --- | --- |
| Material Design Icons | `nf-md-*` | [Templarian/MaterialDesign](https://github.com/Templarian/MaterialDesign) | Apache-2.0 (`LICENSES/Apache-2.0.txt`) |
| Devicons | `nf-dev-*` | [devicons/devicon](https://github.com/devicons/devicon) | MIT (`LICENSES/MIT-devicons.txt`) |

Generic file, source, document, and media identities use Material Design Icons. Technology-specific logos are used only when the pinned Nerd Fonts v3.5.1 registry contains an approved Devicons or MDI identity with a documented license. If that logo is missing or the license/provenance is unclear, Dirloom uses a conservative semantic fallback such as `nf-md-code-braces`.

Catalog v1 embeds only glyph strings, official Nerd Fonts names, codepoints, collection, upstream, and license metadata. ASCII and Unicode channels stay frozen. Representative mappings:

| Use | Glyph | Official name | Code point | Collection |
| --- | --- | --- | --- | --- |
| file | `󰈔` | `nf-md-file` | `U+F0214` | Material Design Icons |
| source family | `󰅩` | `nf-md-code-braces` | `U+F0169` | Material Design Icons |
| Go | `󰟓` | `nf-md-language-go` | `U+F07D3` | Material Design Icons |
| JPEG | `󰈥` | `nf-md-file-jpg-box` | `U+F0225` | Material Design Icons |
| SVG | `󰜡` | `nf-md-svg` | `U+F0721` | Material Design Icons |
| LICENSE | `󰿃` | `nf-md-license` | `U+F0FC3` | Material Design Icons |
| CHANGELOG | `󰋚` | `nf-md-history` | `U+F02DA` | Material Design Icons |
| Svelte | `` | `nf-dev-svelte` | `U+E8B7` | Devicons |
| Helm | `` | `nf-dev-helm` | `U+E7FB` | Devicons |
| Dockerfile | `󰡨` | `nf-md-docker` | `U+F0868` | Material Design Icons |

The complete compiled contract lives in `internal/presentation/catalog/testdata/nerd-fonts-v3.5.1-contract.json`. No font, SVG, image, or network-delivered asset is bundled. `--icons nerd` asserts compatibility; otherwise use Unicode, ASCII, or no icons.
