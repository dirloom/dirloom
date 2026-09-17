# Plan v0.3.0 — Presentation Richness, Semantic Catalog & Visual Quality

> **Projet :** Dirloom  
> **Version cible :** `v0.3.0`  
> **Statut :** Implementation plan — prêt pour exécution par agent de code  
> **Baseline auditée :** `dirloom/dirloom@33e63b62eb52d266e960ef842f9ab52bf1fe09db` (`main`, 17 septembre 2026)  
> **Source stratégique :** roadmap Dirloom — v0.3 sanctuarisée comme release de richesse visuelle  
> **Principe directeur :** enrichir fortement la présentation sans modifier l’artefact canonique, sans réécrire le moteur v0.2 et sans absorber les chantiers TUI / Structural Version Control / Architecture Packs.

---

## 0. Executive decision

La v0.3.0 n’est **pas** une refonte du moteur de présentation.

La v0.2 a déjà livré les fondations structurantes nécessaires :

- catalogue sémantique compilé ;
- séparation `Kind` / `Role` ;
- classification déterministe ;
- ordre de précédence stable ;
- reverse trie pour les suffixes composés ;
- quatre thèmes intégrés ;
- thèmes YAML confinés et inspectables ;
- couleurs terminal sûres ;
- icônes Unicode et Nerd ;
- diagnostics `theme list|explain|validate|classify` ;
- isolation stricte entre présentation terminal et artefacts canoniques ;
- tests contractuels, fuzzing, benchmarks, tri-OS CI et documentation exécutable.

La v0.3 doit donc être une **extension additive et maîtrisée** de cette architecture.

### Objectif produit

> Refermer l’écart visuel avec les outils de listing modernes, notamment eza, en enrichissant fortement la reconnaissance des fichiers, dossiers, extensions, suffixes, manifests, outils et écosystèmes logiciels — sans transformer Dirloom en clone de `ls`.

### Signature de sortie

> **Un rendu Dirloom reconnaît visuellement les structures d’un projet moderne avec une richesse nettement supérieure, tout en conservant les contrats canoniques et la sécurité de v0.2.**

---

# 1. Périmètre v0.3.0

## 1.1 IN

La v0.3 couvre :

1. extension du catalogue sémantique ;
2. extensions de fichiers supplémentaires ;
3. fichiers « well-known » supplémentaires ;
4. dossiers spéciaux et project-centric supplémentaires ;
5. suffixes composés supplémentaires ;
6. nouveaux technical kinds lorsque nécessaire ;
7. enrichissement des glyphes Nerd existants ;
8. glyphes Unicode portables pour les nouveaux kinds ;
9. meilleure couverture des manifests, outils, frameworks et infrastructures ;
10. validation explicite des fallbacks ;
11. enrichissement prudent des thèmes intégrés ;
12. corpus de showcase visuel ;
13. tests de compatibilité v0.2 → v0.3 ;
14. documentation publique et release notes ;
15. preuves de non-régression des formats canoniques.

## 1.2 OUT

La v0.3 ne doit pas absorber :

- `dirloom browse` / TUI ;
- navigation interactive ;
- snapshot ;
- fingerprint ;
- diff ;
- history ;
- watch ;
- Architecture Contracts ;
- scaffold ;
- Architecture Packs ;
- query engine ;
- drift ;
- impact ;
- MCP ;
- Desktop ;
- analyse du contenu des fichiers ;
- shebang / MIME / Git state / permissions / owner / timestamps ;
- catalogue distant ;
- plugin runtime ;
- registre de thèmes ;
- nouveau format de sortie ;
- changement de scanner ;
- changement de modèle `tree.Node` ;
- nouveau chantier d’infrastructure de release.

Toute proposition relevant de ces sujets doit être reportée au milestone prévu.

---

# 2. Baseline auditée — ce qui existe réellement

## 2.1 Semantic catalog v1

Le moteur courant est dans :

```text
internal/presentation/catalog/
├── types.go
├── manifest.go
├── registry.go
├── classify.go
├── validate.go
├── catalog_contract_test.go
├── catalog_internal_test.go
└── testdata/
    └── classification-v1.yaml
```

Contrat actuel :

```text
catalog.Version = 1

64   exact filenames
40   exact directories
32   compound suffixes
120  extensions
-------------------------
256  matchers

96   technical kinds
16   structural roles
```

Ordre public des rôles :

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

## 2.2 Précédence de classification existante

L’ordre actuel est déjà correct et doit rester inchangé :

```text
symlink node type
  → exact directory name
  → exact filename
  → longest compound suffix
  → simple extension
  → file/directory fallback
```

Propriétés existantes :

- matchers built-in case-insensitive ;
- règles de thème utilisateur case-sensitive ;
- longest suffix via reverse trie ;
- fallback déterministe ;
- aucune lecture de contenu ;
- aucune dépendance au réseau ;
- aucune création de kind dynamique ;
- complexité moyenne O(1) pour filename/directory/extension ;
- O(L) pour suffix matching ;
- hiérarchie de kinds bornée à quatre niveaux.

## 2.3 Theme engine

Le thème public est déjà un contrat v1 :

```text
ThemeFileSchemaVersion     = 1
ThemeListSchemaVersion     = 1
ThemeExplainSchemaVersion  = 1
ThemeValidateSchemaVersion = 1
ThemeClassifySchemaVersion = 1
```

Built-ins existants :

```text
default
midnight
daylight
vivid
```

Modes :

```text
--color never|always|auto
--icons never|unicode|nerd|auto
--theme default|midnight|daylight|vivid|<path>
```

Défauts actuels :

```text
color: auto
icons: never
theme: default
```

Ces valeurs sont **compatibility-frozen pour le scope initial v0.3**. Un changement de `icons: never` vers `auto` ne doit pas être glissé dans une PR catalogue ; voir Decision Gate D1.

## 2.4 Isolation canonical / presentation

Aujourd’hui, seul `text` a `UsesPresentation: true`.

Les formats suivants doivent rester dépourvus d’ANSI et de glyphes de présentation :

```text
markdown
markdown-tree
json
mermaid
graphviz
d2
```

Le scanner, `app.Inspect`, le tree model et les encoders canoniques ne doivent jamais importer ou connaître :

- ANSI ;
- glyphes ;
- thèmes ;
- TTY ;
- capacités terminal.

Cette frontière est non négociable.

## 2.5 Built-in theme behavior

Le moteur résout déjà :

```text
catalog classification
  → optional user rule kind/role replacement
  → base node token
  → catalog glyph
  → parent-to-child kind bindings
  → effective role binding
  → direct user-rule overrides
```

Les rôles pilotent actuellement principalement la couleur/les styles du texte.

Les kinds pilotent principalement l’identité technique et la couleur du glyphe.

Le thème `vivid` possède déjà une identité two-tone indépendante.

## 2.6 Contrats de sécurité et validation déjà présents

Les glyphes sont contrôlés :

- UTF-8 valide ;
- maximum 64 octets ;
- maximum 4 runes ;
- pas de caractères de contrôle ;
- pas d’ANSI ;
- pas de caractères bidi dangereux.

Les thèmes custom ont déjà des quotas :

```text
palette <= 128
kind bindings <= 256
role bindings <= 64
rules <= 512
theme file <= 1 MiB
```

Ne pas affaiblir ces limites dans v0.3.

---

# 3. Compatibility policy v0.3

La v0.3 étend un contrat public v0.2. Le mot-clé est **additif**.

## 3.1 Versions publiques

Sauf découverte bloquante :

```text
catalogVersion: 1
theme schemaVersion: 1
config schemaVersion: 1
JSON tree schemaVersion: 1
```

La v0.3 ne doit pas introduire `catalogVersion: 2` simplement parce que le catalogue grandit.

## 3.2 Éléments gelés

Ne pas :

- renommer un kind existant ;
- supprimer un kind existant ;
- changer le parent d’un kind existant sans décision explicite ;
- renommer un rôle ;
- supprimer un rôle ;
- changer l’ordre des 16 rôles ;
- supprimer un matcher v0.2 ;
- changer `Kind` ou `Roles` d’un matcher v0.2 ;
- changer la précédence de classification ;
- ajouter un nouveau `MatchSource` ;
- rendre le classifier path-aware ;
- rendre le classifier content-aware ;
- modifier les défauts de présentation dans une PR de catalogue.

## 3.3 Promotions sémantiques autorisées

L’ajout d’un matcher plus précis peut volontairement changer le résultat d’un fichier qui tombait auparavant dans une règle plus générique.

Exemple conceptuel :

```text
avant:
requirements.txt
  → extension .txt
  → document.text

après:
requirements.txt
  → exact filename
  → manifest.python / config
```

C’est acceptable si :

1. le matcher n’existait pas dans v0.2 ;
2. le nouveau sens est meilleur et documenté ;
3. un test dédié verrouille la promotion ;
4. le changelog la mentionne si elle est suffisamment visible.

Même règle pour les nouveaux suffixes composés qui surclassent une extension générique.

## 3.4 Glyphes existants

Politique recommandée :

- **Unicode existant : stable** sauf bug évident ;
- **Nerd glyph existant : améliorable** pour rendre un kind plus spécifique ;
- chaque modification de Nerd glyph existant est explicitement testée ;
- aucune police n’est embarquée ;
- Dirloom continue de ne jamais auto-détecter ni supposer une Nerd Font.

## 3.5 Palettes existantes

Les couleurs publiques déjà documentées des thèmes intégrés doivent rester stables par défaut.

Autorisé :

- ajouter de nouveaux tokens de palette ;
- ajouter des bindings plus spécifiques ;
- ajouter des mappings icon-color ;
- raffiner le rendu des nouveaux kinds.

À éviter sans décision séparée :

- changer les couleurs fondamentales documentées ;
- changer l’identité `vivid` ;
- introduire un cinquième thème built-in sans justification produit.

---

# 4. Decision gates

## D1 — Default icon mode

**Décision initiale : conserver `icons: never`.**

Ne pas modifier :

```go
Icons: presentation.IconsNever
```

dans `defaultResolution()` pendant les PRs catalogue.

À la fin du milestone, un PR séparé peut être proposé pour :

```text
icons: never → icons: auto
```

uniquement si :

- le showcase démontre un gain net ;
- les implications de compatibilité sont acceptées ;
- les tests CLI/non-TTY/clipboard sont ajustés ;
- README/docs/changelog sont mis à jour ;
- la décision est explicitement approuvée.

Sans approbation, D1 reste fermé.

## D2 — Catalog v2

**Décision initiale : NON.**

Une simple extension additive ne justifie pas un `catalogVersion: 2`.

Ouvrir D2 seulement si l’implémentation exige :

- nouveau type de matcher ;
- changement de précédence ;
- reclassification massive non additive ;
- sémantique incompatible d’un kind ou rôle existant.

## D3 — New structural roles

**Décision initiale : NON.**

Les 16 rôles actuels sont suffisants pour v0.3.

Les nouveaux fichiers doivent être projetés sur :

```text
security/generated/vendor/test/contract/lock/infra/config/
executable/archive/media/data/source/document/tooling/generic
```

Un nouveau rôle modifie l’ordre sémantique public et ne doit pas être ajouté pour simplement obtenir une couleur supplémentaire.

## D4 — Runtime / external catalog

**Décision : NON pour v0.3.**

Le catalogue reste :

- local ;
- compilé dans le binaire ;
- déterministe ;
- sans téléchargement ;
- sans plugin ;
- sans fichier de données runtime.

---

# 5. Benchmark eza — règle d’utilisation

La baseline de comparaison examinée est `eza-community/eza`, notamment son catalogue d’icônes actuel.

eza possède une couverture beaucoup plus large :

- langages supplémentaires ;
- outils spécifiques ;
- frameworks ;
- manifests ;
- dossiers spéciaux ;
- formats médias ;
- formats système ;
- applications desktop ;
- fichiers utilisateur.

Dirloom **ne doit pas chercher la parité brute**.

## 5.1 Ce qu’on emprunte comme benchmark

Comparer :

- densité de reconnaissance ;
- diversité des technical identities ;
- qualité des fallbacks ;
- couverture des écosystèmes modernes ;
- différenciation des fichiers connus ;
- différenciation des dossiers de projet.

## 5.2 Ce qu’on ne copie pas

Ne pas importer aveuglément :

- dossiers personnels (`Desktop`, `Contacts`, etc.) ;
- logique de file manager ;
- métadonnées Unix non pertinentes ;
- mapping OS générique sans rapport avec les projets ;
- code ou tables eza verbatim.

eza est une **source de gap analysis**, pas une dépendance.

### Licence

Ne pas copier directement des tables ou code d’eza.

Si une donnée ou un mapping externe est adapté :

1. vérifier sa licence ;
2. documenter la provenance ;
3. mettre à jour `THIRD_PARTY_NOTICES.md` / `third_party/licenses/` si nécessaire.

Les Nerd glyphs doivent être sélectionnés depuis une source appropriée sans embarquer les fichiers de police.

---

# 6. Target coverage v0.3

Le succès ne doit pas être mesuré uniquement par un nombre de matchers.

La cible est une couverture project-centric forte.

## 6.1 Languages / source ecosystems

Étendre prioritairement les langages manquants ou sous-représentés :

```text
Haskell
OCaml
Nim
D
Fortran
Gleam
Scheme
Racket
Elm
V
Crystal
Nix
HCL
CUE
Jsonnet
```

À vérifier pendant l’implémentation contre les extensions canoniques réelles.

Ne pas créer un kind pour une extension exotique sans usage logiciel plausible.

## 6.2 JavaScript / TypeScript / Node ecosystem

Couvrir les fichiers et dossiers courants autour de :

```text
npm
pnpm
Yarn
Bun
Deno
Next.js
Nuxt
Vite
Vitest
Jest
Playwright
Cypress
Storybook
Nx
Turborepo
Biome
ESLint
Prettier
Changesets
Husky
```

Exemples à auditer/ajouter lorsque absents :

```text
bun.lock
deno.json
deno.jsonc
biome.json
turbo.json
nx.json
lerna.json
playwright.config.*
cypress.config.*
.storybook/
.changeset/
.husky/
.next/
.nuxt/
```

Utiliser exact filenames/suffixes/directories existants plutôt que d’inventer un matcher générique.

## 6.3 Go

Compléter notamment :

```text
go.work
go.work.sum
.golangci.yml
.golangci.yaml
.goreleaser.yml
.goreleaser.yaml
```

Conserver `source.go` et les rôles existants.

## 6.4 Dart / Flutter

Couvrir :

```text
pubspec.lock
analysis_options.yaml
build.yaml
.dart_tool/
.flutter-plugins
.flutter-plugins-dependencies
```

et les suffixes generated courants qui restent déterministes.

## 6.5 Python

Couvrir notamment :

```text
requirements.txt
requirements-dev.txt
Pipfile
Pipfile.lock
uv.lock
tox.ini
pytest.ini
ruff.toml
mypy.ini
.python-version
.venv/
venv/
__pycache__/
.pytest_cache/
.mypy_cache/
.ruff_cache/
```

Les credentials ou virtualenv doivent conserver des rôles explicables (`config`, `vendor`, `generated`, etc.).

## 6.6 Rust

Compléter :

```text
rust-toolchain
rust-toolchain.toml
.cargo/
```

sans dupliquer les semantics de `Cargo.toml` / `Cargo.lock`.

## 6.7 JVM / Gradle / Maven

Couvrir :

```text
build.gradle.kts
settings.gradle
settings.gradle.kts
gradle.properties
gradlew
gradlew.bat
.gradle/
```

et les fichiers Maven/JVM courants réellement utiles.

## 6.8 .NET

Ajouter une couverture project-centric :

```text
.sln
.csproj
.fsproj
.vbproj
.vcxproj
Directory.Build.props
Directory.Build.targets
global.json
NuGet.config
packages.lock.json
bin/
obj/
```

Réutiliser les kinds `manifest.dotnet`, `source.csharp`, `source.fsharp` lorsque possible.

## 6.9 PHP / Ruby / Elixir / Erlang

Exemples :

```text
composer.lock
phpunit.xml
Gemfile
Gemfile.lock
Rakefile
mix.exs
mix.lock
rebar.config
```

## 6.10 Infrastructure / platform

### Terraform / OpenTofu

Couvrir en priorité :

```text
.tf
.tfvars
.hcl
.terraform/
terraform.tfstate
terraform.tfstate.backup
tofu-related well-known files when stable
```

Les states doivent être différenciés des simples configs si un kind existant le permet sans complexité excessive.

### Kubernetes / Helm / GitOps

Couvrir :

```text
Chart.yaml
Chart.lock
values.yaml
kustomization.yaml
helmfile.yaml
skaffold.yaml
charts/
helm/
k8s/
kubernetes/
manifests/
deploy/
deployments/
```

### Containers

Compléter :

```text
.dockerignore
compose.yml
compose.yaml
docker-compose.yml
docker-compose.yaml
devcontainer.json
.devcontainer/
```

Ne pas ajouter de matcher prefix juste pour gérer `Dockerfile.foo` dans v0.3.

### CI/CD

Couvrir les noms courants supplémentaires :

```text
action.yml
action.yaml
azure-pipelines.yml
Jenkinsfile
.gitlab-ci.yml
cloudbuild.yaml
```

et dossiers :

```text
.github/
.gitlab/
.circleci/
```

## 6.11 Editors / developer tooling

Couvrir raisonnablement :

```text
.editorconfig
.vscode/
.idea/
.devcontainer/
.tool-versions
.nvmrc
.node-version
.python-version
```

Ne pas dériver vers les fichiers personnels de l’utilisateur.

## 6.12 Documentation / governance

Compléter les variantes well-known :

```text
README
README.txt
README.rst
LICENSE.md
LICENCE.md
CHANGELOG
CHANGELOG.txt
CONTRIBUTORS
AUTHORS
SECURITY.md
CODEOWNERS
```

Toute promotion doit être cohérente avec `contract` / `document` / `security`.

## 6.13 Security / certificates / keys

Ajouter une reconnaissance structurelle prudente des extensions usuelles :

```text
.pem
.crt
.cer
.key
.pub
.p12
.pfx
.asc
.sig
```

Contraintes :

- pas de lecture de contenu ;
- pas de message affirmant qu’un fichier contient réellement un secret ;
- utiliser le rôle `security` comme signal structurel, pas comme verdict ;
- ne jamais afficher le contenu.

## 6.14 Media / design / archives

Compléter les formats courants :

```text
.gif
.webp
.avif
.ico
.bmp
.tif
.tiff

.flac
.ogg
.m4a

.mov
.mkv
.avi

.7z
.bz2
.xz
.zst
.rar

.apk
.aab
.ipa
.msi
.nupkg
.gem
```

Réutiliser les parents `media.*`, `archive.*`, `binary.*` au maximum.

---

# 7. Technical-kind strategy

## 7.1 Réutiliser avant d’ajouter

Avant de créer un kind :

1. chercher un kind existant ;
2. chercher un parent existant ;
3. vérifier si le besoin est seulement un rôle ;
4. vérifier si le besoin est seulement un Nerd glyph plus spécifique.

Ne créer un nouveau kind que lorsqu’il représente une identité technique stable.

## 7.2 Kinds candidats

À confirmer par inventaire réel, par exemple :

```text
source.haskell
source.ocaml
source.nim
source.d
source.fortran
source.gleam
source.scheme
source.racket
source.elm
source.v
source.crystal
source.nix
source.hcl
source.cue
source.jsonnet

manifest.terraform
manifest.helm
manifest.nix

data.properties
data.plist
data.certificate
data.key
data.localization
```

Cette liste est une cible de conception, **pas une obligation de créer chaque kind**.

## 7.3 Glyph policy

Pour chaque nouveau kind :

- Unicode portable et sobre ;
- Nerd glyph spécifique si disponible ;
- pas d’emoji ;
- pas d’hypothèse sur la largeur d’affichage ;
- pas de font bundling ;
- validation existante conservée.

Pour les kinds existants :

- garder les Unicode stables ;
- enrichir les Nerd glyphs lorsque le kind peut réellement devenir plus identifiable.

Cibles prioritaires pour des Nerd glyphs spécifiques :

```text
source.*
manifest.node
manifest.go
manifest.rust
manifest.python
manifest.java
manifest.dotnet
manifest.dart
manifest.php
data.database
data.notebook
source.vue
source.svelte
source.astro
source.graphql
source.protobuf
```

---

# 8. Code architecture

## 8.1 Ne pas réécrire le classifier

`classify.go` est déjà adapté au problème.

La v0.3 ne doit pas remplacer :

- maps exactes ;
- reverse suffix trie ;
- `classificationFrom`;
- ordre de précédence.

Les changements dans `classify.go` devraient être nuls ou extrêmement limités à des garde-fous/tests.

## 8.2 Décomposer le manifeste avant croissance

`manifest.go` est aujourd’hui monolithique. Son expansion importante rendrait les reviews difficiles.

Refactor recommandé, **sans changement sémantique** :

```text
internal/presentation/catalog/
├── manifest.go
├── manifest_filenames.go
├── manifest_directories.go
├── manifest_suffixes.go
├── manifest_extensions.go
```

`manifest.go` conserve :

- orchestration `buildManifest()`;
- assemblage stable ;
- validation des comptes ;
- `Entries()` ;
- helpers communs.

Chaque fichier de données ne contient qu’une catégorie de matchers.

Le refactor doit être prouvé byte/behavior-neutral par la fixture v0.2.

## 8.3 Compteurs explicites

Remplacer les magic numbers dispersés par des compteurs nommés si cela améliore la lisibilité :

```go
const (
    FilenameEntryCount  = ...
    DirectoryEntryCount = ...
    SuffixEntryCount    = ...
    ExtensionEntryCount = ...

    EntryCount = FilenameEntryCount +
        DirectoryEntryCount +
        SuffixEntryCount +
        ExtensionEntryCount
)
```

Les nombres finaux de v0.3 ne doivent pas être choisis pour atteindre un quota arbitraire.

Ils sont le résultat du catalogue accepté, puis deviennent contractuels.

## 8.4 Roles

Conserver :

```go
RoleCount = 16
```

et l’ordre exact existant.

## 8.5 Registry

`registry.go` reste le registre central des kinds.

Si sa taille devient réellement problématique, autoriser une séparation purement déclarative :

```text
registry.go
registry_source.go
registry_manifest.go
registry_data.go
registry_document.go
registry_media.go
```

mais seulement si nécessaire.

Ne pas introduire :

- génération de code ;
- fichier YAML runtime ;
- JSON embarqué ;
- nouvelle dépendance.

## 8.6 Built-in themes

Le moteur actuel `builtIn()` + palettes doit être réutilisé.

L’objectif v0.3 est :

- confirmer l’héritage de tous les nouveaux kinds ;
- ajouter les bindings spécifiques utiles ;
- enrichir les icon colors seulement si nécessaire ;
- maintenir le contraste ;
- ne pas créer de rules built-in ad hoc pour compenser un mauvais catalogue.

Le catalogue doit porter l’identité ; les thèmes doivent l’interpréter.

---

# 9. Test architecture

La v0.3 doit augmenter le niveau de preuve, pas seulement les fixtures.

## 9.1 Frozen v0.2 compatibility fixture

Créer une baseline immutable avant toute expansion :

```text
internal/presentation/catalog/testdata/classification-v0.2.yaml
```

Cette fixture doit être une copie figée des 256 cas de v0.2.

Ajouter :

```text
catalog_v02_compat_test.go
```

Le test vérifie pour les 256 entrées historiques :

- même `Kind` ;
- mêmes `Roles` ;
- même `MatchSource` ;
- même `MatcherKey`.

Cette fixture ne doit **jamais** être mise à jour mécaniquement dans v0.3.

Si un cas historique change, le test doit casser et provoquer une décision humaine.

## 9.2 Exhaustive v0.3 matcher fixture

Conserver / étendre :

```text
testdata/classification-v1.yaml
```

Le document doit contenir exactement un cas par matcher v1 actuel.

Critères :

- ID unique ;
- matcher unique ;
- couverture 100 % des matchers ;
- groupe/count exact ;
- résultat exact ;
- stable order.

## 9.3 Promotion tests

Ajouter une table explicite pour les nouveaux matchers qui deviennent plus spécifiques qu’une ancienne extension/fallback.

Exemples conceptuels :

```text
requirements.txt
terraform.tf
Chart.yaml
go.work
build.gradle.kts
```

Le test doit montrer la nouvelle classification attendue.

## 9.4 Collision / precedence tests

Étendre la suite pour couvrir explicitement :

```text
symlink > everything
directory exact only for directory
filename > suffix
filename > extension
longest suffix > shorter suffix
suffix > extension
case-insensitive built-ins
unknown unicode basename → fallback
dotfile exact
extensionless exact
compound-extension edge cases
```

Ne pas changer la précédence pour satisfaire un nouveau cas : ajouter le matcher approprié.

## 9.5 Kind registry tests

Tester :

- `len(Kinds()) == KindCount`;
- aucun duplicate ;
- parent existant ;
- chaîne <= 4 ;
- Unicode valide ;
- Nerd valide ;
- aucun control/bidi/ANSI ;
- every kind has effective glyphs ;
- defensive copies ;
- known specific Nerd mappings.

## 9.6 Role contract tests

Verrouiller explicitement :

```text
RoleCount == 16
```

et l’ordre exact.

## 9.7 Fuzzing

Enrichir les seeds de `FuzzClassifyIsDeterministic` :

```text
README
.env.local
terraform.tf
foo.spec.tsx
foo.d.mts
foo.blade.php
Ω/名.go
emoji-like filenames
dotfiles
names with multiple dots
very long safe basename
```

Invariant :

```text
Classify(x) == Classify(x)
Kind != ""
Roles non-empty
UTF-8 safe
```

## 9.8 Benchmarks

Conserver :

```text
BenchmarkClassifyExact
BenchmarkClassifySuffix
BenchmarkClassifyFallback
```

Ajouter si utile :

```text
BenchmarkClassifyExtension
BenchmarkClassifyLargeCatalogExact
```

Avant PR1 expansion :

```bash
go test -run '^$' -bench 'Classify' -benchmem ./internal/presentation/catalog
```

Après expansion, enregistrer les résultats dans la description de PR.

Pas de seuil CI basé sur le temps absolu.

Gate de conception :

- aucune régression asymptotique ;
- pas de scan linéaire de tous les matchers ;
- exact lookups restent O(1) ;
- suffix remains O(L).

## 9.9 Theme tests

Pour les quatre thèmes :

- compile successful ;
- existing palette identity preserved ;
- every new kind resolves ;
- no missing effective glyph ;
- vivid remains two-tone ;
- existing contrast test >= 4.5:1 still passes ;
- new palette colors, if any, are included in contrast tests ;
- existing null/reset semantics unchanged.

## 9.10 Decorator tests

Ajouter des cas représentatifs :

```text
ordinary source
test source
generated source
manifest
security file
infra file
special directory
archive/media
symlink
unknown fallback
```

Vérifier séparément :

- Unicode icon;
- Nerd icon;
- Nerd → Unicode fallback;
- text color;
- icon color;
- styles;
- spacing;
- ANSI resets;
- escaped terminal text.

## 9.11 CLI tests

Ajouter/étendre les tests pour :

```bash
dirloom theme classify <new-known-file>
dirloom theme classify <new-known-dir>
dirloom theme classify <new-extension>
dirloom theme classify <new-compound-suffix>
```

Text et `--as json`.

Vérifier les exit codes existants.

## 9.12 Canonical non-regression tests

Obligation de prouver que v0.3 ne modifie pas :

```text
neutral unicode text
neutral ASCII text
fenced Markdown
markdown-tree
JSON tree schema/output
Mermaid
Graphviz
D2
```

Les goldens existants sous `internal/render/testdata/` ne doivent pas être « régénérés » pour faire passer une modification de présentation.

Si un golden canonique change, traiter cela comme un bug jusqu’à preuve contraire.

## 9.13 Capability tests

Conserver les contrats :

```text
color auto only eligible TTY
NO_COLOR respected
explicit --color always can override NO_COLOR
icons auto → Unicode, never Nerd
icons auto disabled in pipe/redirection
clipboard semantics unchanged
CI and TERM=dumb neutral
```

D1 fermé => default icons remains `never`.

---

# 10. Visual showcase corpus

Créer un corpus synthétique représentatif, sans dépendre de vrais repos externes.

## 10.1 Scenarios

Minimum :

```text
showcase/
├── go-service
├── typescript-next
├── node-monorepo
├── flutter-app
├── python-service
├── rust-cli
├── dotnet-service
├── jvm-service
├── infra-terraform-k8s
└── mixed-platform
```

Chaque scénario doit inclure uniquement des noms de fichiers/dossiers utiles à la classification.

Éviter de committer de grandes quantités de faux code.

Une définition déclarative dans les tests est acceptable si elle produit un fixture déterministe.

## 10.2 Required visual projections

Pour le corpus :

```bash
dirloom <fixture> --no-config --color always --icons never --theme default
dirloom <fixture> --no-config --color always --icons unicode --theme midnight
dirloom <fixture> --no-config --color always --icons nerd --theme vivid
```

Les outputs ANSI ne deviennent pas nécessairement des contrats publics byte-for-byte pour chaque entrée.

En revanche, les classifications et `StyleInspection` doivent être goldénées.

## 10.3 Showcase acceptance

Dans le corpus curated :

- aucun fichier marqué « known » ne doit finir en `file/generic` par accident ;
- aucun dossier marqué « known » ne doit finir en `directory/generic` ;
- les éléments deliberately generic restent fallback ;
- tous les grands écosystèmes ci-dessus ont une représentation identifiable.

---

# 11. Documentation plan

## 11.1 `docs/catalog.md`

Mettre à jour :

- counts finaux ;
- catégories couvertes ;
- nouveaux kinds ;
- nouveaux exemples ;
- fallback policy ;
- promotion policy ;
- compatibility v0.2 → v0.3 ;
- performance model, s’il reste inchangé ;
- limites explicites.

Les exemples de code/docs doivent rester testables.

## 11.2 `docs/themes.md`

Mettre à jour seulement ce qui change réellement :

- nouveaux exemples de glyphes ;
- héritage des nouveaux kinds ;
- éventuels nouveaux palette keys ;
- showcase vivid ;
- aucune modification mensongère de schema.

Si schema reste v1, dire explicitement que v0.3 enrichit le catalogue sans changer le format de thème.

## 11.3 `README.md`

Mettre à jour :

- section Terminal presentation ;
- counts du catalogue ;
- exemple v0.3 ;
- status release/distribution actuel au moment du merge ;
- ne pas conserver « v0.2.0 composing ».

La distribution Winget doit être vérifiée au moment de la modification ; ne pas hardcoder un état obsolète.

## 11.4 `CHANGELOG.md`

`[Unreleased]` doit recevoir les changements v0.3 au fil des PRs.

Séparer :

```text
Added
Changed
```

Inclure notamment :

- catalogue étendu ;
- nouveaux kinds ;
- nouvelles classifications well-known ;
- enrichissements Nerd glyph ;
- tout changement visuel built-in intentionnel.

## 11.5 `CONTRIBUTING.md`

Le document contient encore une consigne spécifique à `release/v0.2.0`.

La retirer/actualiser avant les travaux v0.3.

Nouvelle règle :

- feature/fix branches depuis latest `main`;
- release branch v0.3 créée uniquement au freeze ;
- PRs focalisées ;
- mêmes gates de qualité.

## 11.6 Roadmap produit

Mettre à jour le statut :

```text
v0.2 — Released
v0.3 — In progress
```

Sans réécrire la vision.

`dirloom browse` reste post-v0.3.

## 11.7 Plan dans le repo

Commit recommandé :

```text
plans/Plan v0.3.0 — Presentation Richness.md
```

Ce document devient la source d’exécution du milestone.

---

# 12. PR decomposition

Le milestone doit être exécuté par PRs indépendantes, reviewables et réversibles.

---

## PR0 — v0.3 development baseline

### Branch

```text
chore/v0.3-development-baseline
```

### Objectif

Nettoyer la documentation de cycle et poser les preuves de compatibilité avant tout changement fonctionnel.

### Code

Aucun comportement utilisateur.

Ajouter :

```text
internal/presentation/catalog/testdata/classification-v0.2.yaml
internal/presentation/catalog/catalog_v02_compat_test.go
```

### Docs

Mettre à jour :

```text
CONTRIBUTING.md
README.md release status
docs/product/roadmap.md status
CHANGELOG.md [Unreleased] if needed
```

### Acceptance

```bash
go test ./internal/presentation/...
go test ./...
```

Les 256 classifications historiques passent inchangées.

---

## PR1 — Catalog maintainability & precedence hardening

### Branch

```text
refactor/v0.3-catalog-layout
```

### Objectif

Préparer une croissance importante du catalogue sans modifier son comportement.

### Code

Refactor recommandé :

```text
manifest.go
manifest_filenames.go
manifest_directories.go
manifest_suffixes.go
manifest_extensions.go
```

Ajouter des compteurs explicites par famille.

Ne pas changer :

```text
EntryCount = 256
KindCount = 96
RoleCount = 16
```

dans cette PR.

### Tests

- v0.2 frozen fixture ;
- current `classification-v1.yaml` ;
- precedence collision tests ;
- fuzz ;
- benchmarks.

### Acceptance

Diff comportemental : **zéro**.

---

## PR2 — Extensions, technical kinds & Nerd glyph richness

### Branch

```text
feat/v0.3-catalog-extensions
```

### Objectif

Étendre la couverture par extension et enrichir les technical kinds/glyphes.

### Code

Toucher principalement :

```text
internal/presentation/catalog/manifest_extensions.go
internal/presentation/catalog/registry.go
```

Ajouter les extensions validées par la matrice de couverture.

Enrichir les Nerd glyphs des kinds existants lorsque pertinent.

Mettre à jour :

```text
KindCount
ExtensionEntryCount
EntryCount
```

### Tests

- 1 fixture par nouveau matcher ;
- registry/glyph safety ;
- classification extension ;
- fallback ;
- benchmark before/after ;
- theme inheritance.

### Docs

Mettre à jour les counts et catégories dans `docs/catalog.md`.

---

## PR3 — Well-known files, directories & compound suffixes

### Branch

```text
feat/v0.3-semantic-names
```

### Objectif

Ajouter la reconnaissance project-centric à forte valeur.

### Code

Toucher :

```text
manifest_filenames.go
manifest_directories.go
manifest_suffixes.go
```

Couvrir :

- manifests ;
- lockfiles ;
- configs ;
- CI/CD ;
- dev tooling ;
- generated/test suffixes ;
- special project directories ;
- infra directories ;
- documentation/governance ;
- security names.

### Tests

- exhaustive fixtures ;
- promotion cases ;
- precedence ;
- case folding ;
- dotfiles ;
- common multi-dot filenames ;
- exact dir vs file behavior.

### Docs

Ajouter des exemples réels à `docs/catalog.md`.

---

## PR4 — Theme richness & visual semantics

### Branch

```text
feat/v0.3-theme-richness
```

### Objectif

Faire exploiter le catalogue enrichi par les quatre thèmes sans casser le schema v1.

### Code

Toucher principalement :

```text
internal/presentation/catalog.go
internal/presentation/catalog_test.go
internal/presentation/compile.go   # seulement si nécessaire
```

Attentes :

- nouveaux kinds héritent correctement ;
- bindings spécifiques uniquement lorsqu’ils apportent une valeur visuelle réelle ;
- vivid garde son identité ;
- palettes existantes restent cohérentes ;
- pas de built-in rule hack pour compenser un mauvais matcher.

### Tests

- all built-ins compile ;
- contrast ;
- specific kind glyph/color;
- generated/test/security/infra semantics ;
- decorator outputs ;
- diagnostics origin.

### D1

Ne pas changer le default icon mode dans cette PR.

---

## PR5 — Showcase, docs & release readiness

### Branch

```text
docs/v0.3-presentation-showcase
```

### Objectif

Fournir la preuve produit et verrouiller la release.

### Code/tests

Ajouter le showcase corpus et ses tests.

### Docs

Finaliser :

```text
README.md
docs/catalog.md
docs/themes.md
docs/use-cases.md        # si pertinent
docs/product/roadmap.md
CHANGELOG.md
```

### Release evidence

Produire des exemples reproductibles pour :

```text
default + no icons
midnight + unicode
vivid + nerd
```

### Gate

Aucune feature nouvelle dans cette PR.

---

# 13. Branching / GitOps

Ne pas créer `release/v0.3.0` au début du chantier.

Chaque PR :

```powershell
git fetch origin
git switch main
git pull --ff-only origin main
git switch -c <branch>
```

Après merge de chaque PR :

```powershell
git switch main
git pull --ff-only origin main
```

Créer `release/v0.3.0` uniquement lorsque :

- PR0–PR4 sont mergées ;
- le scope est gelé ;
- le showcase passe ;
- aucune décision D1/D2/D3 ouverte ne bloque la release.

Ne jamais toucher :

```text
tag v0.2.0
GitHub Release v0.2.0
v0.2.0 release artifacts
checksums/SBOM/attestations v0.2.0
```

---

# 14. Required local validation per PR

Minimum :

```bash
gofmt -w ./cmd ./internal
go mod tidy
git diff --exit-code go.mod go.sum
go mod verify
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/dirloom
```

Lorsque disponibles :

```bash
golangci-lint run
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
goreleaser check
```

Pour une PR catalogue :

```bash
go test -run '^$' -bench 'Classify' -benchmem ./internal/presentation/catalog
```

Avant release candidate :

```bash
goreleaser release --snapshot --clean --skip=publish
go run ./cmd/release-artifacts prepare --dist dist --syft syft
go run ./cmd/release-artifacts verify --dist dist
```

---

# 15. CI gates

Toutes les PRs doivent passer la CI existante :

```text
Verify Ubuntu
Verify Windows
Verify macOS
Race detector
Lint
Vulnerability scan
Diagram syntax compatibility
Pin GitHub Actions and release tools
Release snapshot
```

Ne pas affaiblir une gate pour faire passer v0.3.

Ne pas désactiver :

- documentation contract tests ;
- catalog contract tests ;
- contrast tests ;
- fuzz tests existants ;
- GoReleaser snapshot inventory.

---

# 16. Canonical-output proof

Avant release, produire explicitement la preuve suivante.

## Text neutral

```bash
dirloom <fixture> --no-config --color never --icons never
```

Doit conserver le contrat de rendu historique.

## Markdown

```bash
dirloom <fixture> --no-config --format markdown
```

Aucun ANSI, aucun presentation glyph.

## Semantic Markdown

```bash
dirloom <fixture> --no-config --format markdown-tree
```

Aucun ANSI, aucun presentation glyph.

## JSON

```bash
dirloom <fixture> --no-config --format json
```

Schema tree inchangé.

## Diagram sources

```bash
dirloom <fixture> --no-config --format mermaid
dirloom <fixture> --no-config --format graphviz
dirloom <fixture> --no-config --format d2
```

Aucune sémantique de thème injectée.

---

# 17. Acceptance criteria

## Catalog

- [ ] v0.2 frozen compatibility fixture présente ;
- [ ] 256 matchers historiques inchangés ;
- [ ] catalogue v1 étendu de façon additive ;
- [ ] `catalogVersion` reste `1` ;
- [ ] `RoleCount` reste `16` ;
- [ ] role order inchangé ;
- [ ] matcher precedence inchangée ;
- [ ] tous les nouveaux matchers ont un fixture ;
- [ ] aucune duplicate key ;
- [ ] aucun unknown kind/role ;
- [ ] aucun runtime catalog.

## Kinds / glyphs

- [ ] chaque kind a des glyphes effectifs sûrs ;
- [ ] Unicode portable ;
- [ ] Nerd glyphs spécifiques lorsque utiles ;
- [ ] glyph validation intacte ;
- [ ] aucun emoji dépendant d’une largeur ambiguë ;
- [ ] aucune police embarquée.

## Themes

- [ ] quatre built-ins toujours disponibles ;
- [ ] schema theme v1 inchangé ;
- [ ] default palette stable ;
- [ ] midnight stable ;
- [ ] daylight stable ;
- [ ] vivid reste distinct ;
- [ ] contraste >= 4.5:1 pour toutes les couleurs de référence ;
- [ ] custom theme limits inchangés ;
- [ ] unknown bindings warnings inchangés ;
- [ ] unknown actions errors inchangés.

## Terminal behavior

- [ ] `NO_COLOR` intact ;
- [ ] color auto intact ;
- [ ] icons never par défaut ;
- [ ] icons auto n’active jamais Nerd ;
- [ ] non-TTY auto neutre ;
- [ ] CI neutral behavior intact ;
- [ ] clipboard contract intact ;
- [ ] Windows ANSI prep intact.

## Canonical formats

- [ ] neutral text intact ;
- [ ] Markdown intact ;
- [ ] markdown-tree intact ;
- [ ] JSON intact ;
- [ ] Mermaid intact ;
- [ ] Graphviz intact ;
- [ ] D2 intact.

## Performance

- [ ] exact filename lookup reste O(1) ;
- [ ] exact directory lookup reste O(1) ;
- [ ] extension lookup reste O(1) ;
- [ ] suffix lookup reste O(L) ;
- [ ] aucun scan linéaire global introduit ;
- [ ] benchmark evidence jointe aux PRs catalogue.

## Docs

- [ ] `docs/catalog.md` à jour ;
- [ ] `docs/themes.md` à jour ;
- [ ] README à jour ;
- [ ] CHANGELOG `[Unreleased]` à jour ;
- [ ] CONTRIBUTING ne parle plus d’une release v0.2 ouverte ;
- [ ] roadmap indique v0.3 in progress / done selon étape ;
- [ ] exemples marqués restent exécutables ;
- [ ] liens docs valides.

## CI / release readiness

- [ ] tri-OS green ;
- [ ] race green ;
- [ ] lint green ;
- [ ] govulncheck green ;
- [ ] diagram syntax green ;
- [ ] pin check green ;
- [ ] GoReleaser snapshot green ;
- [ ] 13-artifact inventory green.

---

# 18. Explicit non-goals / agent guardrails

L’agent ne doit pas, même s’il pense « améliorer » le produit :

- changer `icons: never` sans D1 ;
- créer `catalogVersion: 2` sans D2 ;
- créer un nouveau rôle sans D3 ;
- ajouter du TUI ;
- ajouter Bubble Tea ou autre lib interactive ;
- ajouter une base de données ;
- ajouter du réseau ;
- ajouter un registry externe ;
- lire le contenu des fichiers pour classifier ;
- analyser les imports ;
- lire Git state ;
- lire MIME/shebang ;
- ajouter du scaffold ;
- ajouter snapshot/diff ;
- déplacer la logique de présentation dans `tree`, `scanner` ou `app`;
- modifier les formats machine ;
- réécrire le moteur de thèmes ;
- changer la précédence actuelle ;
- copier le catalogue eza ;
- ajouter des fichiers binaires/font au repo ;
- modifier l’automatisation packaging sans lien direct.

Toute dérive doit être remontée comme proposition séparée.

---

# 19. Risk register

| Risque | Impact | Mitigation |
| --- | --- | --- |
| Reclassification involontaire d’un matcher v0.2 | Élevé | Frozen v0.2 fixture |
| Giant `manifest.go` difficile à reviewer | Moyen/Élevé | Split par matcher source |
| Raw-count race avec eza | Moyen | Coverage matrix project-centric |
| Trop de kinds | Moyen | Reuse-first policy |
| Trop de roles | Élevé | RoleCount frozen à 16 |
| Rupture de thèmes custom | Élevé | catalogVersion/theme schema restent v1 |
| Nerd Font incohérent | Moyen | explicit mode + Unicode fallback |
| Palette illisible | Élevé | contrast tests |
| ANSI leak dans machine formats | Critique | canonical regression suite |
| Performance dégradée | Moyen | map/trie preserved + benchmarks |
| Scope creep TUI/diff | Élevé | explicit OUT + PR boundaries |
| Tables externes copiées | Élevé | provenance/license review |
| README/roadmap stale | Faible/Moyen | PR0 documentation hygiene |

---

# 20. Definition of Done v0.3.0

La v0.3.0 est **code complete** lorsque :

1. le catalogue v0.3 est mergé ;
2. les 256 matchers v0.2 restent compatibles ;
3. tous les nouveaux matchers sont exhaustivement testés ;
4. les kinds/glyphes sont sûrs ;
5. les quatre thèmes exploitent correctement la nouvelle richesse ;
6. le showcase project-centric démontre le gain ;
7. tous les formats canoniques restent inchangés ;
8. toute la CI est verte ;
9. README/catalog/themes/changelog/roadmap sont cohérents ;
10. aucune feature hors scope n’a été introduite.

La v0.3.0 est **Release Ready** lorsque :

1. le scope est gelé ;
2. la branche `release/v0.3.0` est créée depuis un `main` vert ;
3. le snapshot GoReleaser passe ;
4. l’inventaire de 13 artefacts passe ;
5. smoke tests passent ;
6. release notes v0.3 sont prêtes ;
7. D1 est explicitement clos (`icons: never` conservé ou changement approuvé dans un PR indépendant).

---

# 21. Agent execution contract

Le texte ci-dessous peut être donné tel quel à l’agent de code.

```text
Tu implémentes Dirloom v0.3.0 à partir de latest main.

Avant toute modification :
1. lis entièrement `plans/Plan v0.3.0 — Presentation Richness.md`;
2. inspecte le code réel et vérifie que les hypothèses du plan restent vraies;
3. relève le HEAD de main et signale toute divergence significative;
4. ne touche jamais au tag/release/artifacts v0.2.0.

Règles :
- suis le découpage PR0 → PR5;
- une PR = un objectif focalisé;
- aucune écriture directe sur main;
- ne change pas catalogVersion/theme schema/config schema sans décision explicite;
- conserve les 16 rôles et leur ordre;
- conserve la précédence de classification;
- conserve `icons: never` tant que D1 n’est pas explicitement approuvé;
- ne rends pas le classifier content-aware ou path-aware;
- ne copie pas les tables eza;
- ne modifie ni scanner, tree model, app.Inspect ni formats canoniques pour satisfaire la présentation;
- ne régénère jamais un golden canonique pour masquer une fuite de présentation.

Pour chaque PR :
1. annonce les fichiers à toucher;
2. implémente;
3. ajoute les tests correspondants;
4. mets à jour les docs publiques affectées;
5. lance les gates locales;
6. résume résultats, risques et éventuels écarts au plan;
7. n’ouvre la PR que lorsque le diff est propre et ciblé.

La v0.3 vise la richesse du catalogue et de la présentation, pas de nouvelles catégories produit.
```

---

# 22. Suggested first action

Commencer par **PR0**, pas par ajouter des icônes.

Séquence immédiate :

```powershell
git fetch origin
git switch main
git pull --ff-only origin main

git switch -c chore/v0.3-development-baseline
```

Puis :

1. figer les 256 cas v0.2 ;
2. ajouter le test de compatibilité ;
3. corriger `CONTRIBUTING.md` ;
4. réconcilier README/roadmap avec l’état réel ;
5. exécuter la suite complète ;
6. ouvrir PR0.

Seulement après le merge de PR0 :

```text
PR1 catalog layout
→ PR2 extensions/kinds/glyphs
→ PR3 well-known files/dirs/suffixes
→ PR4 theme richness
→ PR5 showcase/docs/release readiness
```

---

# 23. Final product constraint

La v0.3 doit rendre Dirloom **plus riche visuellement**, pas plus magique.

Le résultat attendu est :

```text
same filesystem
same canonical tree
same ordering
same filters
same JSON
same Markdown
same diagrams

+ much better semantic recognition
+ much better icon identity
+ much better project-specific visual signal
+ stronger showcase quality
```

La fondation de confiance reste prioritaire sur le nombre d’icônes.

> **Presentation richness must remain a projection of structure — never a mutation of it.**
