# Third-party notices

Dirloom includes the following direct runtime dependencies:

- `github.com/spf13/cobra` v1.10.2 — Apache License 2.0.
- `github.com/git-pkgs/gitignore` v1.2.0 — MIT License.
- `golang.org/x/sys` v0.47.0 — BSD 3-Clause License.
- `golang.org/x/term` v0.45.0 — BSD 3-Clause License.
- `go.yaml.in/yaml/v3` v3.0.4 — MIT and Apache License 2.0.

Dirloom's optional Nerd Font icon strings use code points assigned to the [Material Design Icons](https://github.com/Templarian/MaterialDesign) collection by [Nerd Fonts](https://github.com/ryanoasis/nerd-fonts). Material Design Icons are distributed under the Apache License 2.0; Nerd Fonts records that provenance in its glyph catalog and license audit. Dirloom embeds only the selected Unicode code points and their semantic mapping—no font, font binary, SVG, or network-delivered asset. The Apache 2.0 text is included in `LICENSES/Apache-2.0.txt`.

Their transitive module metadata is pinned in `go.sum`. The corresponding full license texts are distributed in the `LICENSES` directory and in every release archive.

The Go project license copies for `x/sys` and `x/term` are recorded separately as `LICENSES/BSD-3-Clause-x-sys.txt` and `LICENSES/BSD-3-Clause-x-term.txt`.
