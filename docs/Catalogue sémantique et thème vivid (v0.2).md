---
name: Catalogue sémantique et thème vivid v0.2
overview: >-
  Remplacer le mini-catalogue visuel par un classifieur Kind + rôles,
  un catalogue immuable de 256 matchers, le schéma public de thème v1,
  le thème vivid, iconColor et un diagnostic filesystem inspectable,
  sans compatibilité avec le prototype pré-release qu’il remplace.
todos:
  - id: catalog-contract
    content: Verrouiller Kind, rôles, manifest v1 de 256 matchers, index et fixtures indépendantes
    status: completed
  - id: theme-schema-v1
    content: Séparer les versions de contrats et implémenter le schéma public de thème v1 sans loader du prototype
    status: completed
  - id: deterministic-resolver
    content: Implémenter la résolution Kind + rôles + règles + overrides et séparer icône/texte
    status: completed
  - id: builtins-vivid
    content: Brancher le catalogue sur les quatre thèmes et livrer la palette vivid vérifiée
    status: completed
  - id: classify-cli
    content: Ajouter theme classify avec inspection Lstat, root explicite et diagnostics v1
    status: completed
  - id: tests-docs-gitops
    content: Livrer tests, documentation publique, notices, draft PR et CI 6/6
    status: in_progress
isProject: false
---

# Catalogue sémantique et thème vivid (v0.2)

## 1. Résultat attendu et décisions verrouillées

eza décrit des entrées de répertoire. Dirloom décrit la structure d’un projet. Le moteur terminal actuel dans [`internal/presentation`](../internal/presentation) est sûr et déterministe, mais son catalogue reste limité à 12 règles dans [`catalog.go`](../internal/presentation/catalog.go), avec seulement quatre tokens et une couleur commune à l’icône et au nom.

Ce chantier doit être intégré avant le tag `v0.2.0` et livre ensemble :

- un catalogue sémantique immuable, compilé dans le binaire et partagé par tous les thèmes ;
- une classification à deux axes : `Kind` technique pour l’icône, rôles structurels pour la couleur et les styles ;
- exactement 256 matchers dans le catalogue v1, couvrant les projets modernes sans chercher l’exhaustivité artificielle ;
- le schéma YAML public de thème v1, qui remplace le prototype pré-release sans le prendre en charge ;
- le champ `iconColor` et une décoration ANSI séparant icône et texte ;
- les thèmes `default`, `midnight`, `daylight` et le nouveau thème `vivid` ;
- `dirloom theme classify`, qui inspecte réellement une entrée du filesystem sans scanner récursivement un arbre ;
- les tests, la documentation publique, les notices de licence et le changelog correspondants.

Décisions produit :

- `.dirloom.yaml` et les fichiers de thème utilisent chacun `schemaVersion: 1`, mais constituent des contrats indépendants, définis et versionnés dans des packages distincts ;
- le nouveau format de thème remplace le prototype pré-release sans changer son numéro : `schemaVersion: 1` désigne désormais le premier contrat public livré avec v0.2 ;
- aucun fichier conforme uniquement au prototype n’est accepté ou converti ; `catalogVersion: 1` est obligatoire et sert notamment de discriminant explicite ;
- les versions des fichiers thème, diagnostics thème, diagnostics de classification, configuration et arbres JSON sont indépendantes ;
- le catalogue est actif dans les quatre thèmes ; changer de thème ne change pas la classification ;
- le thème sélectionné par défaut reste `default`, mais la préférence intégrée d’icônes devient `never` ;
- `--icons auto`, `--icons unicode` ou `--icons nerd` active explicitement les glyphes du catalogue ; un thème ne peut jamais activer les icônes à lui seul ;
- `--color never --icons never` reste le rendu texte canonique historique ; Markdown, Markdown Tree et JSON restent strictement canoniques ;
- permissions, taille, dates, owner, suivi des symlinks pendant l’inspection d’arbre et modèle JSON restent hors périmètre.

Showcase :

```bash
dirloom --theme vivid --icons nerd
```

## 2. Definition of Done

La fonctionnalité est terminée lorsque :

- les 256 matchers, 96 kinds et 16 rôles du catalogue v1 sont centralisés, immuables et inspectables ;
- chaque matcher possède un cas de fixture indépendant et chaque kind possède un fallback Unicode sûr ;
- tous les thèmes consomment le même catalogue sans dupliquer ses entrées dans `Theme.Rules` ;
- le comportement sans option visuelle explicite utilise `theme=default`, `color=auto` et `icons=never` ;
- `--icons auto` active Unicode uniquement sur un TTY éligible, tandis que `--icons nerd` reste toujours explicite ;
- un thème conforme uniquement au prototype pré-release échoue avec le code `2`, stdout vide et une erreur actionnable indiquant que `catalogVersion` est requis par le schéma public v1 ;
- les diagnostics JSON possèdent leurs propres versions et ne changent pas lors d’une modification du schéma YAML ;
- `theme classify` détermine le type réel par `Lstat`, ne suit pas les symlinks et ne parcourt aucun descendant ;
- les règles utilisateur, le catalogue, les bindings de kind, les rôles et les overrides directs suivent une priorité testée propriété par propriété ;
- les goldens texte neutre, Markdown, Markdown Tree et JSON restent byte-for-byte identiques ;
- les sorties décorées ne peuvent ni renommer, masquer, ajouter, réordonner ou retyper un nœud canonique ;
- les exemples publics YAML et CLI passent par les vrais parseurs et commandes ;
- les checks Windows, Linux et macOS, le race detector, le lint, le scan de vulnérabilités et le snapshot GoReleaser sont verts.

## 3. Modèle produit : séparer identité technique et rôle

La couleur doit exprimer la fonction d’une entrée dans le projet. L’icône doit exprimer son type technique. Un fichier `_test.go` doit donc conserver l’icône Go tout en recevant le rôle `test`; un fichier `.pb.go` conserve l’icône Go avec le rôle `generated`.

```go
type Classification struct {
    Kind       Kind
    Roles      []Role
    Source     MatchSource
    MatcherKey string
}
```

- `Kind` décrit l’identité technique : `source.go`, `data.json`, `document.markdown`, `media.image.png`, `manifest.node`, `directory` ou `symlink` ;
- `Roles` décrit les fonctions structurelles détectées, dans un ordre canonique ;
- `Source` vaut `node-type`, `filename`, `directory`, `suffix`, `extension` ou `fallback` ;
- `MatcherKey` expose la clé exacte ayant gagné, sans chemin absolu ni donnée sensible.

Les rôles publics du catalogue v1 sont :

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

Cet ordre est aussi l’ordre de priorité. Plusieurs rôles peuvent être conservés pour le diagnostic, mais le premier rôle possédant un binding dans le thème devient le rôle visuel effectif. Une règle utilisateur avec `role:` remplace cette sélection par un rôle explicite unique.

Exemples :

| Entrée | Kind | Rôles ordonnés | Match |
| --- | --- | --- | --- |
| `README.md` | `document.markdown` | `contract`, `document` | filename |
| `internal/api/user_test.go` | `source.go` | `test`, `source` | suffix `_test.go` |
| `proto/user.pb.go` | `source.go` | `generated`, `source` | suffix `.pb.go` |
| `package-lock.json` | `data.json` | `lock`, `config`, `data` | filename |
| `node_modules/` | `directory` | `vendor` | directory |
| `logo.png` | `media.image.png` | `media` | extension |

Cette séparation évite les combinaisons artificielles telles que `test.go`, `generated.go`, `test.ts` et `generated.ts`.

## 4. Pipeline et architecture interne

```mermaid
flowchart TD
  fs["Métadonnées réelles: type, nom, chemin relatif"] --> catalog["Catalogue v1: classification Kind + rôles"]
  catalog --> rule["Règle utilisateur gagnante"]
  rule --> effective["Classification effective"]
  effective --> token["Token de type de nœud"]
  token --> kind["Bindings du Kind et de ses parents"]
  kind --> role["Binding du rôle visuel"]
  role --> overrides["Overrides directs de la règle"]
  overrides --> style["NodeStyle: textColor, iconColor, styles, glyphs"]
  style --> decorator["Decorator: segments icône et texte séparés"]
```

Le catalogue reste pur : aucune lecture filesystem, aucune dépendance à Cobra, YAML, ANSI ou terminal. L’adaptateur CLI collecte les métadonnées puis appelle le classifieur.

```text
internal/presentation/
  catalog/
    types.go              # Kind, Role, MatchSource, Classification
    registry.go           # 96 kinds, parents, glyphes et invariants
    manifest.go           # 256 matchers, seule source de vérité
    index.go              # maps exactes + trie inversé des suffixes
    classify.go           # fonction pure Classify
    normalize.go          # case folding ASCII et chemins slash
    validate.go           # collisions, cycles, références, glyphes
    testdata/
      classification-v1.yaml  # 256 attentes indépendantes
  theme_default.go
  theme_midnight.go
  theme_daylight.go
  theme_vivid.go
  compile.go
  decorator.go
  loader.go
  diagnostic.go
  types.go

internal/cli/
  theme.go
  theme_classify.go       # Lstat, root, classification, sortie transactionnelle
```

Le package `catalog` peut dépendre du type canonique minimal de nœud dans `internal/tree`, mais il ne doit jamais importer `presentation`. `presentation` adapte les glyphes et bindings du catalogue vers le rendu terminal.

L’actuel [`catalog.go`](../internal/presentation/catalog.go) est supprimé. `baseIconRules` ne doit plus apparaître dans `Theme.Rules`; le quota de règles reste réservé aux règles YAML utilisateur.

## 5. Contrat du catalogue sémantique v1

### 5.1 Taille et gouvernance

Le catalogue livré contient exactement :

| Classe de matcher | Nombre | Usage |
| --- | ---: | --- |
| Noms de fichiers exacts | 64 | contrats, manifests, lockfiles, outils, CI et infrastructure |
| Noms de dossiers exacts | 40 | source, tests, docs, vendor, build, VCS et tooling |
| Suffixes composés | 32 | tests, génération, déclarations, stories, snapshots et fichiers minifiés |
| Extensions | 120 | langages, web, data, documents, médias, archives, fonts et binaires |
| **Total** | **256** | contrat du catalogue v1 |

Le registre contient exactement 96 kinds et les 16 rôles définis plus haut. Une modification de ces nombres exige une modification explicite du plan, des tests de contrat et du changelog avant merge.

Le manifest Go est la seule autorité runtime. La fixture YAML est indépendante : elle énumère des chemins d’exemple et leurs résultats attendus, mais elle n’est jamais utilisée pour générer le manifest ou le binaire. Cela évite un test tautologique qui recopierait les valeurs testées depuis la source elle-même.

Chaque entrée du manifest contient :

```go
type Entry struct {
    Matcher Matcher
    Kind    Kind
    Roles   []Role
}
```

Chaque définition de kind contient :

```go
type KindDefinition struct {
    Parent  Kind
    Unicode string
    Nerd    string
}
```

Les glyphes appartiennent aux kinds, pas aux matchers. Plusieurs extensions peuvent ainsi partager une identité sans dupliquer les codepoints.

### 5.2 Couverture fonctionnelle obligatoire

Le manifest doit couvrir, au minimum, les groupes suivants dans son inventaire exact de 256 matchers :

- langages système et compilés : C, C++, Objective-C, Swift, Go, Rust, Zig, Java, Kotlin, Scala, C#, F#, Dart, Solidity, VHDL et assembleur ;
- langages dynamiques et scripting : Python, Ruby, PHP, Lua, Perl, R, Julia, Elixir, Erlang, Clojure, Groovy et shells POSIX/PowerShell/Windows ;
- web : JavaScript, TypeScript, JSX, TSX, Vue, Svelte, Astro, HTML, CSS, Sass, Less et Stylus ;
- contrats et manifests : README, LICENSE/LICENCE, COPYING, NOTICE, CHANGELOG, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml` et équivalents ;
- tests et génération : `_test.go`, `.test.*`, `.spec.*`, `.stories.*`, `.snap`, `.pb.go`, `.gen.*`, `.generated.*`, `.d.ts`, `.d.mts`, `.d.cts`, `.g.dart` et `.freezed.dart` ;
- infrastructure : Dockerfile, Containerfile, Compose, Kubernetes, Helm, Terraform, Ansible, CI GitHub/GitLab, Make, CMake, Bazel, Task et Just ;
- data et configuration : JSON/JSONC, YAML, TOML, XML, INI, ENV, CSV, TSV, SQL, GraphQL, Protobuf, Avro et Parquet ;
- documents : Markdown/MDX, reStructuredText, AsciiDoc, TeX, texte, PDF et formats bureautiques courants ;
- médias : images raster/vectorielles, audio et vidéo courants ;
- distribution : archives, packages Linux, JAR/WAR, wheel, WebAssembly, bibliothèques et exécutables ;
- dossiers : source, commandes, packages, tests, fixtures, docs, exemples, assets, migrations, scripts, caches, builds, vendor, VCS et IDE.

L’objectif n’est pas de battre eza par un compteur brut, mais par une meilleure information structurelle : type technique conservé, rôles cumulables, suffixes composés, diagnostic explicable et complexité bornée.

### 5.3 Hiérarchie des kinds

Les kinds forment un arbre acyclique, validé au démarrage des tests :

```text
file
  source
    source.go
    source.rust
    source.python
    source.typescript
    source.javascript
    source.web
  manifest
    manifest.node
    manifest.go
    manifest.rust
    manifest.python
  data
    data.json
    data.yaml
    data.toml
    data.sql
  document
    document.markdown
    document.pdf
    document.office
  media
    media.image
      media.image.png
      media.image.svg
    media.audio
    media.video
  archive
  font
  binary
directory
symlink
```

Contraintes :

- profondeur maximale : 4 ;
- un parent doit exister ;
- aucun cycle ni kind orphelin ;
- identifiants en minuscules ASCII, segments séparés par `.` ;
- aucune création dynamique de kind au runtime ;
- `CatalogVersion = 1` est indépendant du schéma YAML de thème.

### 5.4 Algorithme de classification

`Classify(name, relativePath, nodeType) Classification` applique :

1. `symlink` si `nodeType` est un symlink, sans examiner le nom comme un fichier cible ;
2. matcher de dossier exact si `nodeType` est un dossier ;
3. matcher de nom de fichier exact ;
4. plus long suffixe composé via un trie inversé ;
5. extension simple ;
6. fallback `file` ou `directory`.

Normalisation :

- les clés intégrées sont ASCII et normalisées en minuscules ;
- noms remarquables, dossiers, suffixes et extensions intégrés sont insensibles à la casse sur toutes les plateformes ;
- les chemins relatifs utilisent `/` ;
- aucune normalisation Unicode implicite n’est appliquée ; un nom Unicode non reconnu retombe proprement sur son type générique ;
- les règles YAML utilisateur restent case-sensitive et conservent leur contrat actuel.

Complexité annoncée :

- noms, dossiers et extensions : O(1) moyen via maps immuables ;
- suffixes : O(L) via trie inversé, où L est la longueur du nom ;
- sélection de la règle utilisateur : O(R), avec `R ≤ 512` ;
- résolution des parents : O(D), avec `D ≤ 4`.

Le plan ne revendique donc pas un resolver global O(1). Il supprime le scan linéaire des centaines de règles intégrées et garde le coût utilisateur explicitement borné.

## 6. Schéma public de thème v1, sans compatibilité avec le prototype

### 6.1 Versions indépendantes

Remplacer la constante globale actuelle par des constantes séparées :

```go
const (
    ThemeFileSchemaVersion       = 1
    SemanticCatalogVersion       = 1
    ThemeListSchemaVersion       = 1
    ThemeExplainSchemaVersion    = 1
    ThemeValidateSchemaVersion   = 1
    ThemeClassifySchemaVersion   = 1
)
```

Les versions existantes de `.dirloom.yaml`, `config explain` et du JSON d’arbre restent définies dans leurs packages respectifs. Aucune constante de présentation ne doit les piloter.

Un fichier dont `schemaVersion` est absent ou différent de `1` est rejeté avant toute inspection. Un ancien fichier prototype portant déjà `schemaVersion: 1` est rejeté structurellement s’il ne satisfait pas le nouveau contrat, notamment si `catalogVersion` est absent. Dans les deux cas :

- code de sortie `2` ;
- stdout vide ;
- fichier `--output` intact ;
- message précis, par exemple `catalogVersion is required for theme schemaVersion 1` pour un prototype reconnaissable ;
- aucune détection heuristique, conversion, fallback ou lecture du prototype.

Ce remplacement ne nécessite pas de modifier la politique générale de versioning : le prototype de thème n’a jamais été publié dans une release Dirloom. Le changelog doit néanmoins rendre visible sa substitution par le schéma public v1 définitif. Après le tag `v0.2.0`, toute évolution de ce contrat sera gouvernée normalement par la politique de versioning.

### 6.2 Document YAML

```yaml
schemaVersion: 1
catalogVersion: 1
name: team
description: Team terminal theme
appearance: dark

palette:
  edge: "#96A0B5"
  file: "#E5E9F0"
  source: "#65D6BA"
  generated: "#9AA4B6"
  image: "#FF8FC1"

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

icons:
  spacing: 1
```

Contrat :

- `schemaVersion: 1` et `catalogVersion: 1` sont obligatoires ;
- `name` et `appearance` restent obligatoires ;
- `iconColor` est accepté sur tokens, kinds, rôles et règles ; absent, il hérite ; explicitement `null`, il suit la couleur effective du texte ;
- `icons.unicode` et `icons.nerd` acceptent une chaîne valide ou `null` pour supprimer explicitement le glyphe hérité de ce canal ;
- `styles: []` efface les styles hérités ; une propriété absente hérite ;
- `kinds` applique des bindings aux kinds ou familles de kinds ;
- `roles` applique des bindings aux rôles structurels ;
- une règle peut remplacer `kind`, remplacer le rôle visuel avec `role`, puis appliquer des champs visuels directs ;
- une règle doit définir exactement un matcher et au moins une action ;
- `kind` et `role` inconnus dans une règle sont des erreurs, car une action demandée ne doit jamais devenir silencieusement inactive ;
- une clé inconnue dans `kinds` ou `roles` produit un warning stable et est ignorée, afin de permettre l’inspection de thèmes destinés à un catalogue futur ;
- les champs YAML inconnus restent rejetés par `KnownFields` ;
- les custom themes héritent des tokens, bindings de kinds et bindings de rôles de `default`, puis appliquent leurs overlays ;
- aucune entrée du catalogue ne peut être ajoutée ou remplacée par YAML dans cette version.

Limites :

| Élément | Limite |
| --- | ---: |
| Fichier YAML | 1 Mio |
| Palette | 128 couleurs |
| Bindings de kinds | 256 |
| Bindings de rôles | 64 |
| Règles utilisateur | 512 |
| Profondeur d’un kind | 4 |
| Icône | 64 octets et 4 runes |

Les restrictions existantes restent applicables : document YAML unique, UTF-8, clés dupliquées rejetées, ancres/alias/merge keys/tags personnalisés interdits, pas d’include, template, interpolation, réseau ou exécution.

## 7. Résolution déterministe

### 7.1 Sélection de la règle

Une seule règle utilisateur gagne :

```text
path exact
  > name exact
  > glob
  > extension
  > node type
```

À priorité égale, la première déclaration gagne. Les matchers strictement dupliqués restent invalides. Les règles de différents niveaux ne sont pas fusionnées entre elles.

### 7.2 Fusion propriété par propriété

Pour chaque nœud :

1. partir du token `node.directory`, `node.file` ou `node.symlink` ;
2. appliquer la chaîne de parents du kind, du plus générique au plus spécifique ;
3. parcourir les rôles détectés dans l’ordre canonique et appliquer le premier binding présent ;
4. sélectionner la règle utilisateur gagnante ;
5. si la règle définit `kind`, remplacer le kind détecté et recalculer toute sa chaîne de bindings ;
6. si la règle définit `role`, remplacer le rôle visuel et appliquer son binding ;
7. appliquer les champs directs de la règle : `color`, `iconColor`, `styles`, `icons` ;
8. si `iconColor` reste absent ou est explicitement réinitialisé, utiliser la couleur finale du texte ;
9. choisir le glyphe selon le mode : Nerd → Unicode → aucun glyphe, ou Unicode → aucun glyphe.

Les champs directs gagnent toujours sur les bindings induits par `kind` et `role`. Une liste de styles explicite remplace la liste héritée au lieu de la concaténer. Les diagnostics exposent l’origine de chaque propriété finale.

`NodeStyle` devient :

```go
type NodeStyle struct {
    textColor colorSpec
    iconColor colorSpec
    styles    []string
    icons     IconPair
}
```

## 8. Décorateur terminal

Le [`decorator.go`](../internal/presentation/decorator.go) actuel peint l’icône, les espaces et le nom dans une même séquence ANSI. La nouvelle projection doit :

1. échapper le nom affiché ;
2. choisir le glyphe avec son fallback ;
3. peindre l’icône avec `iconColor`, sans `bold`, `italic`, `dim` ni `underline` ;
4. fermer le span ANSI de l’icône ;
5. ajouter l’espacement non stylé ;
6. peindre le nom avec `textColor` et `styles` ;
7. fermer le span ANSI du nom.

Contraintes :

- chaque span possède son reset ;
- aucun style ne fuit vers un connecteur ou la ligne suivante ;
- aucune largeur de cellule n’est supposée ;
- les contrôles, ANSI injecté et caractères bidirectionnels invisibles restent échappés ;
- le décorateur neutre et tous les formats canoniques ignorent intégralement ces données.

## 9. Thèmes intégrés

### 9.1 Activation commune du catalogue

Les quatre thèmes utilisent le même catalogue et les mêmes glyphes. Ils diffèrent uniquement par leurs tokens, bindings de kinds, bindings de rôles, couleurs et styles.

| Thème | Intention | Catalogue |
| --- | --- | --- |
| `default` | universel, sobre, palette ANSI du terminal | actif ; bindings colorés minimaux |
| `midnight` | fond sombre de référence `#1A1B26` | actif ; familles structurelles complètes |
| `daylight` | fond clair de référence `#FFFFFF` | actif ; familles structurelles complètes |
| `vivid` | identité sombre expressive, vitrine Nerd Font | actif ; 16 rôles et accents de kinds |

La préférence intégrée globale devient :

```text
color: auto
icons: never
theme: default
```

Conséquences :

- une invocation sans option conserve un arbre sans icônes ;
- `--theme vivid` ne suffit pas à afficher des glyphes ;
- `--icons auto` active Unicode uniquement sur un vrai TTY éligible ;
- `--icons unicode` ou `--icons nerd` force explicitement le jeu demandé pour le texte ;
- `--icons nerd` ne prétend toujours pas détecter la police installée ;
- presets, thèmes et configuration d’inspection ne modifient jamais implicitement le mode d’icônes.

### 9.2 Palette exacte de `vivid`

`vivid` utilise le fond sombre de référence `#10131A`, sans l’émettre ni modifier le terminal. Sa palette publique v0.2 est :

| Token/rôle | Couleur |
| --- | --- |
| `edge` | `#96A0B5` |
| `file` | `#E5E9F0` |
| `directory` | `#7EB6FF` |
| `symlink` | `#C6A0FF` |
| `accent` | `#6ED6FF` |
| `security` | `#FF7C91` |
| `generated` | `#9AA4B6` |
| `vendor` | `#8F99AB` |
| `test` | `#A8E063` |
| `contract` | `#FFD166` |
| `lock` | `#E8A66A` |
| `infra` | `#FF927E` |
| `config` | `#F2C879` |
| `executable` | `#77DDB0` |
| `archive` | `#DDB07A` |
| `media` | `#FF8FC1` |
| `data` | `#70D0F6` |
| `source` | `#65D6BA` |
| `document` | `#B8ACFF` |
| `tooling` | `#B2BDCF` |
| `generic` | `#C8D0DD` |

Tous les coloris utilisés pour le texte doivent atteindre WCAG 2.1 AA `≥ 4.5:1` contre le fond de référence. Une `iconColor` décorative doit atteindre `≥ 3:1`. Les tests calculent le contraste à partir des valeurs sRGB, sans accepter une validation visuelle subjective comme seule preuve.

Bindings `vivid` :

- `contract` : bold + underline ;
- `test` : couleur test, sans style additionnel ;
- `generated` et `vendor` : dim ;
- `security` : bold ;
- `source`, `infra`, `config`, `lock`, `data`, `document`, `media`, `archive`, `executable`, `tooling` et `generic` : couleur dédiée ;
- kinds de médias, langages et manifests : `iconColor` de famille ou accent spécifique, sans modifier la couleur du texte donnée par le rôle.

La palette est originale et ne reprend pas la combinaison violette/jaune/rose d’eza. Aucune police n’est embarquée.

## 10. Inspection d’une classification réelle

### 10.1 Commande

```bash
dirloom theme classify README.md
dirloom theme classify src/main.go --theme vivid
dirloom theme classify ./src --root . --theme vivid --as json
```

La commande inspecte exactement l’entrée demandée :

1. valider les arguments et charger/compiler le thème demandé ; toute erreur de thème survient avant l’accès à la cible ;
2. résoudre `--root` depuis le répertoire courant ; sa valeur par défaut est `.` ;
3. résoudre le chemin cible relativement à cette racine, ou accepter un chemin absolu contenu dans la racine ;
4. vérifier, après nettoyage et résolution des parents symlinkés, que la cible reste sous la racine ;
5. appeler `os.Lstat` sur la cible ;
6. classifier `file`, `directory` ou `symlink` sans suivre la cible d’un symlink ;
7. normaliser le chemin relatif avec `/` pour les règles de thème et produire le diagnostic ;
8. ne parcourir aucun enfant, ne lire aucun contenu de fichier et n’effectuer aucun accès réseau.

Un socket, pipe, device ou type filesystem non pris en charge produit une erreur d’usage. Une cible absente produit le code `2`; une permission refusée ou autre erreur d’I/O produit le code `1`.

Options :

| Option | Valeurs | Défaut |
| --- | --- | --- |
| `--root` | dossier existant | `.` |
| `--theme` | built-in ou chemin YAML v1 public | `default` |
| `--as` | `text`, `json` | `text` |

Les flags `--config`, `--no-config`, `--no-user-config`, `--preset`, `--color`, `--icons`, `--format`, `--style`, `--depth`, `--ignore` et `--output` sont rejetés plutôt qu’ignorés. La commande n’utilise ni configuration projet/utilisateur ni preset.

### 10.2 Sortie texte

```text
Path: src/main.go
Type: file
Kind: source.go
Roles: source
Matched by: extension (.go)
Theme: vivid (built-in)
Text: color=#65D6BA styles=none
Icon: unicode="•" nerd="󰟓" color=#65D6BA
```

La sortie reste non décorée pour pouvoir être copiée dans un rapport ou une issue.

### 10.3 Contrat JSON

```json
{
  "schemaVersion": 1,
  "path": "src/main.go",
  "type": "file",
  "classification": {
    "kind": "source.go",
    "roles": ["source"],
    "matchedBy": "extension",
    "matcherKey": ".go"
  },
  "theme": {
    "name": "vivid",
    "source": { "kind": "built-in" }
  },
  "style": {
    "textColor": "#65D6BA",
    "iconColor": "#65D6BA",
    "styles": [],
    "icons": {
      "unicode": "•",
      "nerd": "󰟓"
    }
  },
  "origins": {
    "kind": "catalog",
    "role": "catalog",
    "textColor": "theme-role",
    "iconColor": "theme-kind",
    "icons": "catalog-kind"
  }
}
```

Règles : tableaux jamais `null`, clés et rôles dans un ordre stable, chemin absolu absent du JSON, stdout vide en erreur, écriture transactionnelle en mémoire et `schemaVersion: 1` du diagnostic indépendant du `schemaVersion: 1` du thème YAML.

## 11. Diagnostics des thèmes

`ThemeNames()` inclut `vivid`. `theme list`, `theme explain` et `theme validate` conservent chacun leur enveloppe JSON v1.

`theme explain` n’imprime pas 256 matchers. Il expose :

- les tokens ;
- la palette ;
- les bindings de kinds ;
- les bindings de rôles ;
- les règles utilisateur d’un thème fichier ;
- le catalogue lié et ses compteurs.

Exemple d’enveloppe :

```json
{
  "schemaVersion": 1,
  "themeSchemaVersion": 1,
  "catalog": {
    "version": 1,
    "entryCount": 256,
    "kindCount": 96,
    "roleCount": 16
  },
  "theme": {
    "name": "vivid",
    "appearance": "dark"
  }
}
```

`theme validate` retourne également `themeSchemaVersion: 1` et `catalogVersion: 1`. Les warnings possèdent des codes stables, notamment `unknown-token`, `unknown-kind-binding` et `unknown-role-binding`.

## 12. Sécurité et compatibilité

Le catalogue et les thèmes intégrés sont compilés dans le binaire :

- aucun téléchargement de catalogue, thème ou police ;
- aucun codepoint ANSI fourni directement par YAML ;
- aucun include, template, interpolation ou commande ;
- aucune lecture de contenu par `theme classify` ;
- aucune traversée récursive par une commande de diagnostic ;
- aucune modification d’un fichier inspecté ou d’un thème ;
- aucune influence sur le modèle canonique, l’ordre, les exclusions ou la profondeur ;
- aucun changement du schéma JSON des arbres.

Le catalogue v1 et le schéma public de thème v1 font partie du contrat de v0.2. Après publication, renommer/supprimer un kind, changer la classification d’un matcher ou modifier une définition de thème exige une analyse SemVer et une entrée de changelog. Le prototype pré-release antérieur n’est pas supporté et aucun code de compatibilité n’est conservé.

## 13. Tests

### 13.1 Catalogue pur

- exactement 256 matchers, 96 kinds et 16 rôles ;
- fixture indépendante avec un cas positif par matcher ;
- unicité des matchers après normalisation et absence de collisions case-insensitive ;
- toutes les références de kind et rôle valides ;
- arbre de kinds sans cycle, profondeur ≤ 4 et parents présents ;
- suffixe le plus long gagnant pour `.d.ts`, `.d.mts`, `.spec.ts`, `.pb.go` et cas imbriqués ;
- distinction filename/directory et priorité du symlink ;
- extensions et noms remarquables case-insensitive sur les trois OS ;
- chemins et règles utilisateur toujours case-sensitive ;
- glyphes Unicode/Nerd valides, fallback Unicode présent, aucun contrôle/ANSI/bidi ;
- copies défensives et impossibilité de muter les registres retournés ;
- fuzz tests sur noms, chemins slash et entrées Unicode : aucun panic et résultat déterministe ;
- benchmarks séparant exact lookup, suffix trie et fallback ; aucune promesse de temps absolu fragile en CI.

### 13.2 Schéma public de thème v1

- v1 minimal, complet et avec valeurs explicitement réinitialisées ;
- `schemaVersion` absent, `2`, inconnu ou mal typé ;
- `catalogVersion` absent ou inconnu ;
- fixtures de l’ancien prototype `schemaVersion: 1` sans `catalogVersion` ou avec sa structure obsolète : rejet avant inspection, sans conversion ;
- `iconColor` sur token, kind, rôle et règle ;
- `null` explicite pour `iconColor` et glyphes ;
- `styles: []` effaçant l’héritage ;
- kind/rôle inconnu dans une règle : erreur ;
- kind/rôle inconnu dans un bloc de bindings : warning stable ;
- règles sans action, plusieurs matchers ou matchers dupliqués ;
- limites palette/kinds/rôles/règles et fichier > 1 Mio ;
- champs inconnus, clés dupliquées, ancres, alias, merge keys, tags et documents multiples ;
- versions de diagnostic toujours égales à `1` lorsque le fichier de thème vaut lui aussi `1`, avec des constantes et tests de contrat distincts.

### 13.3 Résolution et rendu

- token → parents du kind → rôle → règle `kind`/`role` → overrides directs ;
- `_test.go` : icône Go + couleur test ;
- `.pb.go` : icône Go + couleur generated ;
- `README.md` : icône Markdown + rôle contract ;
- `package-lock.json` : icône JSON/manifest + rôle lock ;
- règle `kind:` recalculant la chaîne de parents ;
- règle `role:` remplaçant le rôle visuel ;
- `iconColor` distinct de `textColor` ;
- styles d’icône toujours vides ;
- spans ANSI séparés et resets indépendants ;
- fallback Nerd → Unicode → aucun glyphe ;
- les quatre thèmes utilisent le catalogue ;
- `icons=never` par défaut, même sur TTY ;
- `--icons auto` Unicode sur TTY et neutre dans pipe/CI ;
- `--theme vivid` seul n’active aucune icône ;
- `--theme vivid --icons nerd` utilise les glyphes Nerd ;
- `NO_COLOR`, profils truecolor/256/16 et VTP Windows inchangés ;
- texte neutre, Markdown, Markdown Tree et JSON : zéro diff des goldens historiques.

### 13.4 CLI `theme classify`

- fichier, dossier et symlink réels via `Lstat` ;
- symlink non suivi, y compris cible absente ou extérieure à la racine ;
- chemin relatif et absolu contenu dans `--root` ;
- racine relative/absolue, espaces et Unicode ;
- traversal `..`, cible hors root et parent symlink sortant rejetés ;
- entrée absente, permission refusée et type spécial ;
- thèmes built-in et fichier YAML conforme au schéma public v1 ;
- prototype pré-release rejeté avant diagnostic ;
- texte et JSON conformes aux contrats ;
- flags d’inspection interdits ;
- argument absent/surnuméraire et `--as` invalide ;
- stdout vide sur erreur et write failure classifiée code `1` ;
- aucune lecture du contenu, aucun parcours récursif et aucun accès réseau.

### 13.5 Documentation et couverture

- exemples YAML marqués dans [`themes.md`](themes.md) chargés par le vrai loader du schéma public v1 ;
- commandes `vivid` et `theme classify` exercées par les tests CLI ;
- compteurs documentés comparés au catalogue compilé ;
- liens relatifs du README et des guides vérifiés ;
- flags documentés comparés à l’aide Cobra ;
- fixture catalogue distincte du manifest runtime ;
- couverture `internal/presentation/catalog` ≥ 95 % ;
- couverture `internal/presentation` maintenue au moins à son niveau de référence (~82 %) ;
- aucun seuil n’est satisfait en excluant artificiellement du code métier.

## 14. Documentation publique

Créer `docs/catalog.md` comme page canonique anglophone :

1. Overview.
2. How classification works.
3. Technical kinds and structural roles.
4. Match precedence and case handling.
5. Built-in coverage.
6. Inspect a real entry.
7. Use catalog bindings in a custom theme.
8. Determinism, security and performance.
9. Compatibility and catalog evolution.

Mettre à jour dans la même PR :

- [`themes.md`](themes.md) : schéma public v1, `catalogVersion`, kinds, rôles, `iconColor`, `null`, `vivid`, `theme classify` et quotas ;
- [`configuration.md`](configuration.md) : défaut `icons: never` et activation explicite ;
- [`presets.md`](presets.md) : les presets ne sélectionnent jamais un mode d’icônes ;
- [`use-cases.md`](use-cases.md) : revue de projet, diagnostic de classification, TTY sobre et showcase Nerd ;
- [`architecture.md`](architecture.md) : pipeline Kind + rôles avant projection ;
- [`README.md`](../README.md) : exemple court `vivid`, garantie canonique et liens ;
- [`functional-specification.md`](product/functional-specification.md) : schéma public de thème v1, catalogue v1 et priorité complète ;
- [`roadmap.md`](product/roadmap.md) : catalogue/vivid livrés en v0.2 ;
- [`CHANGELOG.md`](../CHANGELOG.md) : `Added` pour catalogue/vivid/classify, `Changed` pour le remplacement du prototype par le schéma public de thème v1 et les icônes désactivées par défaut ;
- [`THIRD_PARTY_NOTICES.md`](../THIRD_PARTY_NOTICES.md) : provenance MDI/Nerd Fonts, codepoints utilisés et absence de police embarquée ;
- [`dependencies.md`](dependencies.md) si la documentation de provenance doit référencer une version de Nerd Fonts ;
- ne pas modifier [`SPEC-v0.1.md`](../SPEC-v0.1.md), qui reste l’archive contractuelle v0.1.

Les guides publics restent en anglais. La roadmap et la spécification stratégique conservent leur langue actuelle. Les commentaires de code restent en anglais.

## 15. Gates de validation

```bash
gofmt -l cmd internal
go mod tidy
go mod verify
go vet ./...
go test ./...
go test -race ./...
go build -trimpath ./cmd/dirloom
golangci-lint run
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
goreleaser release --snapshot --clean
```

Le snapshot doit produire les six archives Windows/Linux/macOS amd64/arm64, rester `CGO_ENABLED=0`, inclure la documentation et les notices mises à jour, et ne dépendre d’aucun fichier de catalogue ou thème au runtime.

## 16. GitOps et livraison

Sources : [hub Release workflow & Git-Ops](https://knowledge.floxio.ai/doc/guide-release-workflow-git-ops-hub-6ERj1DbE2s), [politique de branching](https://knowledge.floxio.ai/doc/policy-strategie-de-branching-git-workflow-RRY4kPPKjc), [runbook Pull Request](https://knowledge.floxio.ai/doc/runbook-workflow-developpeur-git-pull-request-jtJ3vCs2d3) et [politique de versioning](https://knowledge.floxio.ai/doc/policy-politique-de-versioning-release-wXOdmECJfM).

Ce chantier est une demande de changement, pas une release. La PR sémantique précédente est déjà intégrée dans `origin/main`. Avant tout code :

```bash
git status
git switch main
git pull --ff-only origin main
git switch -c feat/semantic-catalog
```

Ne pas poursuivre le travail sur `feat/semantic-markdown`. Vérifier que le document de plan est conservé lors du changement de branche avant tout staging.

### PR draft

Titre :

```text
feat(theme): add semantic catalog and vivid identity
```

Commits recommandés :

```text
feat(catalog): add kind and role classification engine
feat(theme)!: replace pre-release theme schema with public v1 catalog bindings
feat(theme): add vivid built-in theme
feat(cli): inspect filesystem semantic classifications
test(theme): cover catalog and canonical contracts
docs(theme): publish semantic catalog and vivid guide
```

La PR explique explicitement :

- le remplacement du prototype pré-release par le schéma public de thème v1 définitif, sans loader de compatibilité ;
- l’indépendance des versions YAML, diagnostics et arbres JSON ;
- le passage du défaut `icons:auto` à `icons:never` ;
- l’activation du catalogue dans les quatre thèmes ;
- la séparation Kind/rôles et la priorité complète ;
- le scan non récursif et sans suivi de symlink de `theme classify` ;
- les 256 matchers, 96 kinds et 16 rôles ;
- les résultats de tests, contrastes, couverture et snapshot ;
- le risque `medium-high`, principalement lié au nouveau contrat public de thème v1 et au rendu TTY explicite ;
- le rollback avant release : revert du squash merge ;
- l’absence de migration runtime, v1 n’ayant jamais été publiée.

Une seule PR reste acceptable parce que le schéma, le catalogue, les thèmes et leurs diagnostics forment un contrat atomique. La revue doit néanmoins suivre les commits logiques et distinguer le manifest mécanique du moteur. Si le diff non mécanique devient difficile à relire, la PR est arrêtée et découpée avant passage en ready.

Conditions de passage ready :

- code, tests, documentation, changelog et notices présents ;
- diff relu, sans fichier local ni secret ;
- checks frais 6/6 sur la dernière révision ;
- catalogue et fixture revus indépendamment ;
- revue indépendante ;
- conversations résolues ;
- PR non draft seulement après satisfaction de toutes les portes.

Merge : squash commit `feat(theme): add semantic catalog and vivid identity`, puis suppression de la branche ordinaire après confirmation du merge.

Interdits dans ce chantier : tag `v0.2.0`, GitHub Release, GoReleaser non-snapshot ou publication d’artefacts officiels. Le tag reste régi par le [runbook Release workflow](https://knowledge.floxio.ai/doc/runbook-release-workflow-B3YkeIOyjw) après freeze du scope v0.2.

## 17. Ordre d’implémentation

1. Verrouiller le manifest de 256 matchers, les 96 kinds, les 16 rôles et la fixture indépendante.
2. Implémenter le package pur `catalog`, ses validateurs, son index et ses tests.
3. Séparer toutes les constantes de version et introduire le schéma public de thème v1 strict.
4. Implémenter bindings de kinds/rôles, `iconColor`, resets explicites et résolution propriété par propriété.
5. Supprimer `baseIconRules` et brancher `default`, `midnight` et `daylight` sur le catalogue.
6. Passer le défaut intégré d’icônes à `never` et verrouiller les régressions TTY/canoniques.
7. Ajouter `vivid`, valider mathématiquement les contrastes et enregistrer la provenance des glyphes.
8. Ajouter `theme classify` avec `Lstat`, confinement `--root` et diagnostics texte/JSON.
9. Mettre à jour les documents publics, la spécification, la roadmap, le changelog et les notices.
10. Exécuter la matrice locale, ouvrir la PR draft, obtenir CI 6/6 et une revue indépendante.

## 18. Hors périmètre

- compatibilité ou migration automatique des fichiers issus du prototype pré-release de thème ;
- catalogue utilisateur ou overlay YAML `catalog:` ;
- téléchargement de thèmes, catalogues ou polices ;
- détection ou installation d’une Nerd Font ;
- `LS_COLORS`, `CLICOLOR`, `FORCE_COLOR` ou hyperlinks OSC-8 ;
- héritage ou composition entre thèmes ;
- classification basée sur le contenu, shebang, MIME ou analyse syntaxique ;
- scan récursif dans `theme classify` ;
- permissions, taille, dates, owner ou métadonnées POSIX ;
- états Git, diff, conformité ou sévérité non produits par l’arbre actuel ;
- modification des formats Markdown, Markdown Tree ou JSON d’arbre ;
- TUI, Desktop ou télémétrie ;
- publication immédiate de `v0.2.0`.

Extensions futures préparées mais non réservées dans le contrat v1 : catalogue utilisateur signé, rôles `state.*`/`status.*`, détection par shebang et commandes de recherche du catalogue. Elles devront obtenir leur propre décision de schéma au lieu d’être acceptées silencieusement par le moteur actuel.
