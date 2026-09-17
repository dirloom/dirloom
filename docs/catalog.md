# Semantic catalog

Dirloom classifies project entries before applying terminal presentation. The catalog keeps technical identity and structural purpose separate: a `_test.go` file stays Go while receiving the `test` role; a `.pb.go` file stays Go while receiving `generated`.

Catalog v1 is compiled into the binary. v0.3 extends it additively; `catalogVersion` remains `1`.

| Contract | v0.2 | v0.3 |
| --- | ---: | ---: |
| Exact filenames | 64 | 179 |
| Exact directory names | 40 | 74 |
| Compound suffixes | 32 | 58 |
| Extensions | 120 | 195 |
| **Matchers** | **256** | **506** |
| Technical kinds | 96 | 119 |
| Structural roles | 16 | 16 |

Classification affects terminal color, icon, and style only. It never changes tree membership, node type, name, order, canonical Markdown, or JSON.

## How classification works

Each result contains:

```text
Kind        technical identity, such as source.go or data.json
Roles       ordered structural purposes, such as test and source
Matched by  node-type, filename, directory, suffix, extension, or fallback
Matcher key exact built-in key that won
```

Examples:

| Entry | Kind | Roles | Match |
| --- | --- | --- | --- |
| `README.md` | `document.markdown` | `contract`, `document` | filename |
| `requirements.txt` | `manifest.python` | `config` | filename |
| `Chart.yaml` | `manifest.helm` | `contract`, `infra`, `config` | filename |
| `internal/api/user_test.go` | `source.go` | `test`, `source` | suffix `_test.go` |
| `proto/user.pb.go` | `source.go` | `generated`, `source` | suffix `.pb.go` |
| `package-lock.json` | `data.json` | `lock`, `config`, `data` | filename |
| `node_modules/` | `directory` | `vendor` | directory |
| `main.tf` | `manifest.terraform` | `infra`, `source` | extension `.tf` |
| `flake.nix` | `manifest.nix` | `contract`, `config` | filename |
| `ca.pem` | `data.certificate` | `security` | extension `.pem` |
| `logo.png` | `media.image.png` | `media` | extension |

## Technical kinds

Kinds form a bounded hierarchy. A theme may bind a parent family once and refine a specific child:

```text
file
├── source
│   ├── source.go
│   ├── source.rust
│   ├── source.python
│   ├── source.javascript
│   ├── source.typescript
│   ├── source.haskell
│   └── source.nix
├── manifest
│   ├── manifest.node
│   ├── manifest.go
│   ├── manifest.rust
│   ├── manifest.terraform
│   ├── manifest.helm
│   ├── manifest.nix
│   └── manifest.container
├── data
│   ├── data.json
│   ├── data.yaml
│   ├── data.toml
│   ├── data.sql
│   ├── data.certificate
│   └── data.key
├── document
│   ├── document.markdown
│   ├── document.pdf
│   └── document.office
├── media
│   ├── media.image
│   │   ├── media.image.png
│   │   └── media.image.svg
│   ├── media.audio
│   └── media.video
├── archive
├── font
└── binary

directory
symlink
```

The complete v1 registry has 119 kinds, a maximum depth of four, no runtime-created identifiers, and a safe Unicode fallback for every kind. Glyphs belong to kinds, not individual matchers. v0.3 added language, manifest, and security identities without renaming or removing any v0.2 kind. Terraform/OpenTofu sources use `manifest.terraform`; Nix flakes and well-known Nix files use `manifest.nix`; generic `.hcl` and `.nix` stay on their source kinds.

## Structural roles

Roles use this public priority order:

```text
security
generated
vendor
test
contract
lock
infra
config
executable
archive
media
data
source
document
tooling
generic
```

A classification may retain several roles for diagnostics. The first role with a binding in the active theme becomes the visual role. A theme rule with `role:` replaces that choice with one explicit role.

This order makes high-signal purposes win predictably. For example, `SECURITY.md` is both security-related and contractual; security is visualized first. A generated Go file keeps its Go kind and glyph while the generated role controls text color and dimming.

## Match precedence and case handling

Built-in classification uses this fixed order:

```text
symlink node type
  → exact directory name for a directory
  → exact filename
  → longest compound suffix
  → simple extension
  → file or directory fallback
```

The final symlink is classified before its name, so a link named `README.md` remains `symlink`. Compound suffixes beat extensions: `.d.mts`, `.spec.ts`, `_test.go`, `.blade.php`, and `.pb.go` are resolved before `.mts`, `.ts`, `.go`, or `.php`.

v0.3 may **promote** a real-world path that previously lost to a generic v0.2 extension or fallback. That does not rewrite the 256 frozen v0.2 matcher identities. Those matchers still return the same `Kind`, `Roles`, `MatchSource`, and `MatcherKey`. Promotions are new, more-specific matchers and are changelogued as intentional classification changes:

| Path | v0.2 result | v0.3 result | Why |
| --- | --- | --- | --- |
| `requirements.txt` | `document.text` via `.txt` | `manifest.python` via filename | Python dependency manifest |
| `Chart.yaml` | `data.yaml` via `.yaml` | `manifest.helm` via filename | Helm chart identity |
| `go.work` | `file` fallback | `manifest.go` via filename | Go workspace contract |
| `main.tf` | `file` fallback | `manifest.terraform` via `.tf` | Terraform/OpenTofu source |
| `build.gradle.kts` | `source.kotlin` via `.kts` | `manifest.java` via filename | Gradle Kotlin DSL project file |

Built-in filename, directory, suffix, and extension keys are ASCII and case-insensitive on every platform. User theme rules remain case-sensitive. Relative paths use `/`; Dirloom does not silently normalize Unicode names.

## Built-in coverage

Catalog v1 covers representative modern project structures without content inspection.

- Systems and compiled languages: C/C++, Objective-C, Swift, Go, Rust, Zig, Java, Kotlin, Scala, C#, F#, Dart, D, Fortran, V, Solidity, VHDL, and assembly.
- Functional and additional source ecosystems: Haskell, OCaml, Nim, Gleam, Scheme, Racket, Elm, Crystal, Nix, HCL, CUE, and Jsonnet.
- Dynamic languages and scripts: Python, Ruby, PHP, Lua, Perl, R, Julia, Elixir, Erlang, Clojure, Groovy, POSIX shells, PowerShell, and Windows batch.
- Web: JavaScript, TypeScript, JSX/TSX, Vue, Svelte, Astro, HTML, CSS, Sass, Less, and WebAssembly.
- Contracts and manifests: README variants, license/notice files, changelogs, contribution/security documents, package manifests, lockfiles, Helm charts, Terraform files, and schema files.
- Tests and generation: `_test.go`, `.test.*`, `.spec.*`, `.cy.*`, `.e2e.*`, `.stories.*`, snapshots, `.pb.go`, `.gen.*`, `.generated.*`, TypeScript declarations, `.g.dart`, `.freezed.dart`, and `.blade.php`.
- Infrastructure and tooling: Dockerfile/Containerfile, Compose, Terraform/OpenTofu-related HCL, Helm, Kubernetes directories, Ansible, GitHub/GitLab/CircleCI, Make, CMake, Bazel, Gradle, Task, Just, editors, runtime version pins, and dependency automation.
- Data and configuration: JSON/JSONC, YAML, TOML, XML, INI, properties, plist, localization strings, environment files including `.env.local`, CSV/TSV, SQL, GraphQL, Protobuf, Avro, Parquet, databases, and notebooks.
- Security-shaped names: certificate and key extensions such as `.pem`, `.crt`, `.key`, and `.pub` receive the `security` role as a structural signal. Dirloom does not read content or assert that a file contains a secret.
- Documents and media: Markdown/MDX, reStructuredText, AsciiDoc, TeX, PDF, office documents, common image/audio/video/design formats, and fonts.
- Distribution: archives including 7z/xz/zst, Linux packages, JAR/WAR, wheels, mobile/desktop packages, executables, and shared libraries.
- Directories: source, commands, packages, tests, fixtures, documentation, examples, assets, migrations, scripts, caches, builds, vendor, virtualenvs, VCS, CI, IDE metadata, and generated app output such as `.next` or `.dart_tool`.

Unknown names fall back to `file`/`generic` or `directory`/`generic`. The catalog does not infer from file content, MIME type, shebang, Git state, permissions, size, owner, or timestamps. Some extensions are intrinsically ambiguous: `.v` is classified as V source and `.d` as D source, not Verilog or compiler dependency files.

## Inspect a real entry

Use `theme classify` to see the real filesystem type, catalog result, active theme binding, and origin of each visual property.

<!-- dirloom-catalog-command:classify -->
```bash
dirloom theme classify README.md
dirloom theme classify src/main.go --theme vivid
dirloom theme classify ./src --root . --theme vivid --as json
dirloom theme classify requirements.txt
dirloom theme classify Chart.yaml --as json
```

The command loads the selected theme before target access, resolves the target inside `--root`, and performs exactly one `Lstat`. It does not read file content, follow the final symlink, or recurse into a directory.

Example text contract:

```text
Path: src/main.go
Type: file
Kind: source.go
Roles: source
Visual role: source
Matched by: extension (.go)
Theme: vivid (built-in)
Text: color=#66F0C0 styles=none
Icon: unicode="•" nerd="󰟓" color=#00FFD1
```

With `vivid`, this two-tone result is intentional: the `source` role selects the text color while the `source.go` kind inherits the `source` icon color.

The JSON diagnostic owns an independent `schemaVersion: 1`. It contains no absolute target path or custom-theme path, uses stable role ordering, and emits empty arrays rather than `null`.

## Bind the catalog in a custom theme

A theme can bind a family, a specific kind, and roles without copying catalog matchers.

<!-- dirloom-catalog-theme-example:bindings -->
```yaml
schemaVersion: 1
catalogVersion: 1
name: catalog-bindings
appearance: dark

palette:
  source: "#66F0C0"
  test: "#B6F36B"
  generated: "#A1AAC0"

kinds:
  source:
    iconColor: source
  source.go:
    icons:
      unicode: "•"
      nerd: "󰟓"

roles:
  source:
    color: source
  test:
    color: test
  generated:
    color: generated
    styles: [dim]

rules:
  - match:
      path: "tools/codegen.go"
    kind: source.go
    role: generated
```

The result is resolved property by property:

```text
catalog classification
  → winning user rule selects any replacement kind or role
  → base node token and catalog glyph
  → parent-to-child kind bindings
  → effective role binding
  → direct visual fields from the winning rule
```

An unknown key under `kinds` or `roles` is ignored with a stable validation warning so a future-targeted theme remains inspectable. An unknown `kind:` or `role:` action in a rule is an error.

See [Terminal colors, icons, and themes](themes.md) for the complete schema, nullable resets, quotas, and validation commands.

## Determinism, safety, and performance

The catalog is immutable, local, and compiled into Dirloom:

- no network lookup, plugin, user catalog, or runtime file;
- no file-content read or recursive scan for classification;
- average O(1) exact filename, directory, and extension lookup;
- O(L) longest-suffix matching through a reverse trie, where L is the name length;
- O(R) user-rule selection with `R ≤ 512`;
- O(D) kind inheritance with `D ≤ 4`.

Catalog output cannot alter the canonical tree. `--color never --icons never`, fenced Markdown, semantic Markdown, and JSON retain their established bytes and schemas.

A synthetic corpus lives in `testdata/showcase/`. It contains well-known names only. Generated or hidden directories such as `.next` and `node_modules` remain classified; listing them in a tree requires `--no-default-ignore` and `--hidden` because they are still default-excluded from scanning.

```bash
dirloom testdata/showcase/go-service --no-config --color never --icons never
dirloom testdata/showcase/infra-terraform-k8s --no-config --color always --icons unicode --theme midnight
dirloom testdata/showcase/typescript-next --no-config --no-default-ignore --hidden --color always --icons nerd --theme vivid
```

## Compatibility and evolution

Catalog v1 remains the public contract. v0.3 grows the compiled matcher set and kind registry additively:

- `catalogVersion` stays `1`;
- the 16 roles and their order are unchanged;
- matcher precedence is unchanged;
- the 256 v0.2 **matcher identities** keep the same `Kind`, `Roles`, `MatchSource`, and `MatcherKey`;
- new exact names or longer suffixes may promote paths that previously hit a generic extension or fallback. Those path-level promotions are intentional SemVer-reviewed classification changes, not a rewrite of the frozen matcher table.

Unicode glyphs for existing kinds stay stable. Nerd glyphs may become more specific; Unicode remains the portable fallback. Dirloom never bundles a font and never auto-detects a Nerd Font. The default icon mode remains `never`.

Renaming or removing a kind or role, changing a historical matcher classification, or modifying a documented built-in palette identity requires a changelog entry and Semantic Versioning review. Promoting a path with a new more-specific matcher is the supported v0.3 evolution path and must be listed in the changelog.

Future content-based classifiers, signed user catalogs, Git-state roles, and architecture/compliance states require explicit new contracts. Unknown identifiers are not accepted silently as runtime kinds.