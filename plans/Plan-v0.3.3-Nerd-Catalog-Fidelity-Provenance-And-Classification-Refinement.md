# Dirloom v0.3.3 — Nerd Catalog Fidelity, Provenance & Classification Refinement

> **Release cible :** `v0.3.3`
>
> **Nature :** raffinement rétrocompatible du jalon PRESENTATION.
>
> **Objet :**
>
> 1. corriger et professionnaliser la projection Nerd ;
> 2. introduire une gouvernance explicite des collections Nerd Fonts utilisées ;
> 3. supprimer les faux logos et métaphores ambiguës ;
> 4. corriger quelques classifications conventionnelles déjà présentes dans le catalogue ;
> 5. préserver tous les contrats structurels et canoniques de Dirloom.
>
> **Cette version doit être exécutée de bout en bout, dans son intégralité.**
>
> Aucun élément du présent plan ne doit être reporté silencieusement à une
> version ultérieure.

---

# 0. Mandat d'exécution

Ce document est **normatif** pour l'implémentation de `v0.3.3`.

L'agent doit réaliser l'intégralité du chantier :

- audit de confirmation de l'état réel du dépôt ;
- création des branches prévues ;
- modifications de code ;
- classification refinement ;
- refonte contrôlée du catalogue Nerd ;
- gouvernance et provenance des glyphes ;
- tests unitaires ;
- tests de contrat ;
- tests de non-régression ;
- fixtures et showcases nécessaires ;
- validation des licences ;
- documentation utilisateur ;
- documentation développeur ;
- documentation de provenance ;
- changelog ;
- statut de release ;
- lint ;
- vet ;
- race detector ;
- vulnérabilités ;
- build ;
- validation finale du diff.

Le travail n'est **pas considéré comme terminé** tant qu'il reste :

- un TODO ;
- un FIXME introduit par ce chantier ;
- un stub ;
- un test skipped ;
- un mapping Nerd non audité parmi ceux inclus dans le scope ;
- une provenance inconnue ;
- une licence manquante ;
- une fixture obsolète ;
- une documentation contradictoire ;
- un test rouge ;
- un changement de classification non documenté ;
- un comportement prévu par ce plan mais non implémenté.

Si l'état réel du dépôt diffère légèrement de ce plan, l'agent doit :

1. préserver les invariants et objectifs définis ici ;
2. adapter l'implémentation à l'architecture réellement présente ;
3. expliquer l'écart dans son rapport final ;
4. ne jamais réduire silencieusement le scope.

La création du tag `v0.3.3`, la GitHub Release, sa publication et les mises à
jour Scoop/Homebrew/Winget sont **hors scope de la branche d'implémentation**.

---

# 1. État audité au démarrage

État observé avant travaux :

```text
repository:
  dirloom/dirloom

main:
  95a3adcf74a522d33910b32a4cd6bf979e3c783e

latest published release:
  v0.3.2

catalog:
  catalogVersion: 1
  matchers: 506
  filenames: 179
  directories: 74
  suffixes: 58
  extensions: 195
  kinds: 119
  roles: 16
```

Architecture actuelle pertinente :

```text
internal/presentation/catalog/
├── classify.go
├── manifest.go
├── manifest_filenames.go
├── manifest_directories.go
├── manifest_suffixes.go
├── manifest_extensions.go
├── registry.go
├── types.go
├── validate.go
├── catalog_contract_test.go
├── catalog_internal_test.go
├── catalog_v02_compat_test.go
├── showcase_test.go
└── testdata/

internal/presentation/
├── catalog.go
├── decorator.go
├── capabilities.go
└── ...

docs/
├── catalog.md
├── themes.md
├── architecture.md
├── release-workflow.md
└── ...

THIRD_PARTY_NOTICES.md
LICENSES/
CHANGELOG.md
README.md
```

Classification actuelle :

```text
file
  → exact filename
  → suffix
  → extension
  → fallback
```

Cette priorité doit rester inchangée.

---

# 2. Positionnement de v0.3.3

La séquence produit doit rester :

```text
v0.3.0
PRESENTATION

    ↓

v0.3.1
Contextual Help & CLI Guidance

    ↓

v0.3.2
Icon Capability Contract & Portable ASCII Icons

    ↓

v0.3.3
Nerd Catalog Fidelity, Provenance
& Classification Refinement

    ↓

v0.4.0
CHANGE
Fingerprint + Snapshot + Verify + Diff
```

`v0.4.0` reste réservée au jalon CHANGE.

`v0.3.3` ne doit introduire :

- aucun nouveau mode CLI ;
- aucune nouvelle option publique ;
- aucun nouveau format ;
- aucun nouveau renderer ;
- aucune nouvelle architecture de scan ;
- aucun changement de configuration ;
- aucun changement du contrat `style × icons`.

---

# 3. Workflow Git obligatoire

Le workflow de release du dépôt est normatif.

## 3.1 Créer la branche de release

Depuis `main` propre et à jour :

```bash
git fetch --prune origin

git switch main
git pull --ff-only origin main

git status --short
git rev-parse HEAD

git switch -c release/v0.3.3
git push -u origin release/v0.3.3
```

La branche doit partir du `main` contenant `v0.3.2`.

## 3.2 Créer la branche dédiée d'implémentation

Tout le développement de ce plan doit être effectué sur :

```text
feat/v0.3.3-nerd-catalog-classification
```

Création :

```bash
git switch release/v0.3.3
git pull --ff-only origin release/v0.3.3

git switch -c feat/v0.3.3-nerd-catalog-classification
git push -u origin feat/v0.3.3-nerd-catalog-classification
```

Le PR d'implémentation doit cibler :

```text
feat/v0.3.3-nerd-catalog-classification
        ↓
release/v0.3.3
```

Il ne doit pas cibler directement `main`.

Une fois le scope terminé, testé et intégré dans `release/v0.3.3`, la branche
de release suivra le workflow normal :

```text
release/v0.3.3
        ↓
      main
        ↓
     v0.3.3
```

La création et publication du tag restent une opération de Release Owner.

---

# 4. Baseline obligatoire

Avant toute modification :

```bash
git status --short
git rev-parse HEAD
go version
go env
go mod verify
go test ./...
go vet ./...
go build ./...
```

Si disponibles dans l'environnement :

```bash
go test -race ./...
golangci-lint run
govulncheck ./...
```

Enregistrer :

```text
base_branch
base_commit
go_version
go_mod_verify
baseline_tests
baseline_vet
baseline_build
baseline_race
baseline_lint
baseline_vuln
```

Si la baseline est rouge avant changement :

**STOP.**

Diagnostiquer d'abord le problème et ne pas l'attribuer au chantier v0.3.3.

---

# 5. Invariants absolus

Les invariants suivants doivent rester vrais.

## 5.1 Contrat style × icons

```text
style
  = géométrie du tree

icons
  = préfixe iconographique

theme
  = présentation / couleurs / emphase
```

Par conséquent :

```bash
dirloom --style ascii --icons nerd
dirloom --style unicode --icons nerd
dirloom --style ascii --icons unicode
dirloom --style unicode --icons ascii
```

restent toutes des combinaisons valides.

Ne jamais coupler :

```text
style == ascii
```

avec :

```text
icons == ascii
```

ou toute autre déduction implicite.

## 5.2 Modes d'icônes inchangés

Le contrat v0.3.2 reste :

```text
never
ascii
unicode
nerd
auto
```

Aucune valeur n'est ajoutée ou supprimée.

## 5.3 Auto inchangé

```text
--icons auto
  nerd     si capability Nerd déclarée
  unicode  sinon
```

Aucune auto-détection de police.

Aucune heuristique :

```text
Windows Terminal => Nerd
WT_SESSION       => Nerd
modern terminal  => Nerd
TTY              => Nerd
```

## 5.4 Canonicité

Les formats machine et documentaires doivent rester exempts de décoration
terminale :

```text
JSON
semantic Markdown
Mermaid
Graphviz
D2
diagnostics machine
```

Aucun glyphe Nerd ne doit contaminer une sortie canonique.

---

# 6. Scope contractuel du catalogue

L'objectif n'est pas d'agrandir massivement le catalogue.

Le baseline est :

```text
506 matchers
119 kinds
16 roles
catalogVersion: 1
```

Objectif v0.3.3 :

```text
matcher count: 506
kind count:    119
role count:     16
catalogVersion: 1
```

Les classifications corrigées dans cette release utilisent des matchers
**déjà existants**.

Ne pas ajouter un matcher uniquement pour les cas suivants : ils existent déjà.

---

# 7. Classification refinement obligatoire

Cette section est une modification intentionnelle du contrat sémantique.

Elle doit être explicitement testée et documentée.

## 7.1 Docker Compose

État actuel :

```text
docker-compose.yml   → data.yaml
docker-compose.yaml  → data.yaml
compose.yml          → data.yaml
compose.yaml         → data.yaml
```

Cible :

```text
docker-compose.yml   → manifest.container
docker-compose.yaml  → manifest.container
compose.yml          → manifest.container
compose.yaml         → manifest.container
```

Rôles :

```text
infra
config
```

à conserver sauf justification forte issue de l'architecture réelle.

Raison :

Compose est un manifeste de container orchestration, pas simplement un
document YAML générique.

Le matcher filename existe déjà ; ne pas créer de doublon.

---

# 8. Docker Bake

État actuel :

```text
docker-bake.hcl
  → document.text
  roles: infra, config
```

Cible :

```text
docker-bake.hcl
  → manifest.container
  roles: infra, config
```

Raison :

le fichier décrit un manifeste/build definition Docker Buildx Bake.

Il ne doit plus apparaître comme simple texte.

Ne pas changer le matcher.

---

# 9. Terraform dependency lock

État actuel :

```text
.terraform.lock.hcl
  → document.text
  roles: lock, infra
```

Cible :

```text
.terraform.lock.hcl
  → manifest.terraform
  roles: lock, infra
```

Le rôle `lock` doit impérativement être préservé.

Ne pas transformer ce fichier en `source.hcl` :

```text
.terraform.lock.hcl != generic HCL source
```

Les sources génériques `.hcl` doivent continuer à utiliser :

```text
source.hcl
```

Les sources Terraform/OpenTofu doivent continuer à utiliser :

```text
manifest.terraform
```

pour :

```text
.tf
.tfvars
.tofu
```

La nouvelle classification harmonise donc le lock avec la famille Terraform
sans perdre sa fonction structurelle de lockfile.

---

# 10. Non-régression de classification

Les corrections précédentes doivent modifier uniquement les chemins
intentionnellement approuvés.

Créer un test de promotion explicite avec au minimum :

```text
docker-compose.yml
docker-compose.yaml
compose.yml
compose.yaml
docker-bake.hcl
.terraform.lock.hcl
```

Le test doit vérifier :

```text
Kind
Roles
MatchedBy
MatcherKey
```

Attendus :

```text
filename
```

pour tous ces cas.

Ajouter également des contrôles négatifs :

```text
random.yaml          → data.yaml
settings.hcl         → source.hcl
application.hcl      → source.hcl
random.lock          ≠ manifest.terraform
```

La priorité :

```text
filename
> suffix
> extension
> fallback
```

doit être explicitement couverte.

---

# 11. Fixture contractuelle classification-v1

Le dépôt possède :

```text
internal/presentation/catalog/testdata/classification-v1.yaml
```

et un mécanisme contrôlé via :

```text
DIRLOOM_WRITE_CATALOG_FIXTURE=1
```

La fixture doit être mise à jour uniquement pour les classifications
intentionnellement modifiées.

Ne pas régénérer aveuglément et accepter le diff.

Après génération :

1. inspecter le diff ;
2. vérifier que seuls les cas attendus changent ;
3. vérifier que le nombre de cas reste `506` ;
4. vérifier que les IDs restent stables ;
5. vérifier que les matcher identities restent stables.

Ajouter un test ou assertion garantissant que les six promotions listées
ci-dessus sont bien les seules modifications classificationnelles prévues par
v0.3.3, hors changement explicitement ajouté au présent scope après audit.

---

# 12. Revue de cohérence des manifestes existants

Effectuer un audit ciblé des filename matchers existants autour de :

```text
container
Docker
Terraform
OpenTofu
Helm
Kubernetes
```

Objectif :

identifier d'éventuelles incohérences évidentes du même type.

Cependant :

**ne pas élargir automatiquement le scope.**

Un fichier supplémentaire ne doit être reclassifié que si :

1. son matcher exact existe déjà ;
2. le kind cible existe déjà ;
3. le changement est sémantiquement évident ;
4. il ne requiert aucun nouveau kind ;
5. il est ajouté à la liste contractuelle v0.3.3 ;
6. il reçoit tests + changelog + documentation.

Les cas discutables doivent rester inchangés.

Pas de “cleanup opportuniste”.

---

# 13. Politique Nerd v0.3.3

La projection Nerd devient officiellement **multi-source et gouvernée**.

Politique :

```text
1. Glyphe technologique exact
   provenant d'une collection Nerd Fonts approuvée

2. Glyphe sémantique exact MDI
   pour les concepts génériques

3. Glyphe générique correspondant à la famille

4. Glyphe file/source générique en dernier recours
```

Interdiction :

```text
technologie connue
    → métaphore arbitraire pouvant être prise pour son logo
```

Principe :

> Une icône générique correcte est préférable à une fausse précision.

---

# 14. Version Nerd Fonts épinglée

La v0.3.3 doit définir une version Nerd Fonts de référence.

Ne jamais documenter simplement :

```text
Nerd Fonts latest
```

L'agent doit déterminer une release stable appropriée et enregistrer
explicitement :

```text
nerd_fonts_version
glyphnames_source
source_commit_or_tag
retrieval_date
```

Exemple conceptuel :

```text
Nerd Fonts: vX.Y.Z
glyphnames.json:
  repository: ryanoasis/nerd-fonts
  tag: vX.Y.Z
```

Le build et l'exécution de Dirloom ne doivent pas nécessiter de réseau.

Dirloom ne doit toujours embarquer :

- aucune font ;
- aucun `.ttf` ;
- aucun `.otf` ;
- aucun SVG ;
- aucune image de logo.

---

# 15. Catalogue Nerd structuré

Le code actuel stocke essentiellement :

```go
type KindDefinition struct {
    Kind    Kind
    Parent  Kind
    ASCII   string
    Unicode string
    Nerd    string
}
```

v0.3.3 doit rendre la provenance Nerd audit-able.

Éviter une simple accumulation supplémentaire de codepoints anonymes dans
`registry.go`.

Introduire une représentation dédiée, par exemple :

```go
type NerdGlyphDefinition struct {
    Glyph        string
    OfficialName string
    Codepoint    string
    Collection   string
    Upstream     string
    License      string
}
```

La structure exacte peut être adaptée à l'architecture Go existante.

Exigence :

pour chaque override Nerd spécifique, il doit être possible de retrouver :

```text
kind
glyph
official Nerd Fonts name
codepoint
collection
upstream
license
```

Ne pas nécessairement exposer ce metadata comme nouvelle API publique.

Cette structure peut rester interne.

---

# 16. Source de vérité Nerd

Créer un emplacement clairement identifié, par exemple :

```text
internal/presentation/catalog/nerd_glyphs.go
```

ou équivalent cohérent avec l'architecture existante.

Le fichier doit distinguer :

```text
semantic generic glyphs
technology-specific glyphs
fallbacks
```

Éviter que `registry.go` devienne un mélange de :

- hiérarchie des kinds ;
- fallbacks Unicode ;
- provenance ;
- licence ;
- centaines de codepoints PUA.

`registry.go` peut continuer à construire les `KindDefinition`, mais les
données spécifiques Nerd doivent devenir lisibles et auditables.

---

# 17. Table contractuelle obligatoire

Créer une table contractuelle vérifiable contenant au minimum :

```text
kind
glyph
official_name
codepoint
collection
upstream
license
```

Elle doit être dérivée de la source de vérité utilisée par le code, ou testée
contre celle-ci.

Ne pas maintenir deux listes indépendantes susceptibles de diverger.

Exemple logique :

```text
source.svelte
  glyph
  nf-...
  U+....
  Seti / Devicons / Custom / ...
  upstream project
  license
```

Le nom exact de collection doit correspondre au registre de la version Nerd
Fonts épinglée.

Ne pas inventer les collections.

---

# 18. Collections autorisées

Approche attendue :

```text
MDI
  → concepts génériques

Devicons / Seti / Nerd Fonts Custom
  → technologie identifiable
```

Mais chaque collection doit être validée individuellement.

Ne pas considérer `Custom` comme une licence ou une provenance en soi.

Pour chaque glyphe technologique, déterminer :

```text
Nerd Fonts collection
original icon project
original upstream
license
```

Si la provenance ou la licence est ambiguë :

```text
DO NOT USE THAT GLYPH
```

et utiliser un fallback générique propre.

---

# 19. Corrections Nerd objectives

Corriger au minimum :

## JPEG

```text
media.image.jpeg
```

Ne doit plus apparaître avec une icône de dossier image.

Utiliser le glyphe image/JPEG approprié dans la version Nerd Fonts épinglée.

## SVG

```text
media.image.svg
```

Ne doit plus apparaître avec une icône de dossier image.

Utiliser le glyphe SVG approprié si sa provenance est approuvée, sinon le
glyphe image générique.

## LICENSE

```text
document.license
```

Utiliser le glyphe sémantique licence approprié.

## CHANGELOG

```text
document.changelog
```

Utiliser un glyphe historique/changelog approprié.

Ces quatre mappings doivent recevoir des tests directs.

---

# 20. Technologies actuellement trompeuses

Auditer et corriger les mappings de :

```text
source.svelte
source.astro
source.dart
manifest.helm

source.ocaml
source.nim
source.d
source.gleam
source.elm
source.v
source.crystal
source.cue
```

État constaté v0.3.2 : plusieurs utilisent un pictogramme MDI qui n'est pas le
logo de la technologie et peut être interprété comme tel.

Politique :

```text
exact approved logo available
    => use it

exact logo unavailable / licensing uncertain
    => generic semantic source glyph
```

Ne jamais conserver une métaphore arbitraire uniquement parce qu'elle
“ressemble” au nom de la technologie.

Exemples de patterns à éliminer lorsqu'ils ne correspondent pas réellement au
logo officiel :

```text
hexagon
sigma
pine tree
diamond
cube
function
code array
infinity
generic application
```

---

# 21. Audit complet des 119 kinds

Ne pas s'arrêter aux 16 anomalies déjà connues.

Effectuer un audit automatique + manuel des :

```text
119 kinds
```

Objectif :

classifier chaque projection Nerd dans l'une des catégories :

```text
exact technology
exact semantic
generic semantic
generic fallback
```

Le résultat doit permettre de détecter :

- file kind utilisant folder glyph ;
- technologie utilisant une métaphore arbitraire ;
- doublons suspects ;
- codepoint absent de la version Nerd Fonts épinglée ;
- collection inconnue ;
- provenance non documentée ;
- licence inconnue.

Cela ne signifie pas que les 119 kinds doivent recevoir un logo spécifique.

Les fallbacks sont intentionnels et acceptables.

---

# 22. Harmonisation source / manifest d'une même technologie

Lorsqu'une technologie possède plusieurs kinds :

```text
source.X
manifest.X
```

les glyphes doivent être cohérents lorsque cela a du sens.

Exemples :

```text
source.go
manifest.go

source.rust
manifest.rust

source.python
manifest.python

source.dart
manifest.dart

source.nix
manifest.nix

manifest.terraform
```

Politique :

- même technologie identifiable : même identité visuelle de base possible ;
- le kind et la couleur/thème continuent à distinguer source/manifest ;
- ne pas créer artificiellement un nouveau logo de “manifest”.

Le rôle et la couleur peuvent différencier la fonction structurelle.

---

# 23. Unicode strictement gelé

La projection Unicode existante doit rester inchangée.

Créer un test de snapshot/contrat du canal Unicode si aucun test suffisamment
strict n'existe déjà.

Exigence :

```text
before v0.3.3 Unicode map
==
after v0.3.3 Unicode map
```

pour les 119 kinds.

La correction Nerd ne doit pas modifier Unicode.

---

# 24. ASCII strictement gelé

Même règle pour ASCII.

Le catalogue ASCII introduit par v0.3.2 doit rester byte-for-byte identique.

Tester tous les kinds :

```text
ASCII before == ASCII after
```

Et conserver l'invariant :

```text
U+0020..U+007E only
```

pour tous les glyphes ASCII.

---

# 25. Fallback Nerd

Conserver le comportement :

```text
Nerd glyph
    ↓ absent
Unicode glyph
    ↓ absent
no glyph
```

Ne jamais faire :

```text
unknown technology
→ random logo
```

Les fallbacks doivent rester déterministes.

---

# 26. Largeur et spacing

Le renderer ne doit pas introduire de calcul manuel basé sur :

```text
len(glyph)
utf8.RuneCount
assumed terminal width
```

Le spacing existant reste le contrat.

Le glyphe et le nom doivent rester séparés par :

```text
theme.icons.spacing
```

selon le comportement existant.

Tester au minimum :

```text
spacing = 0
spacing = 1
spacing = 4
```

avec Nerd glyphs.

Aucune tentative de détection Mono/non-Mono ne doit être ajoutée.

---

# 27. Validation Nerd Fonts Mono / non-Mono

Une validation visuelle manuelle est obligatoire.

Matrice minimale :

| Terminal | Nerd Font Mono | Nerd Font non-Mono |
|---|---:|---:|
| Windows Terminal | ✅ | ✅ |
| WezTerm | ✅ | ✅ |
| Alacritty | ✅ | ✅ |

Tester au minimum :

```text
--style unicode --icons nerd
--style ascii   --icons nerd
--theme vivid   --icons nerd
```

Tester un showcase suffisamment riche.

Recommandé :

```text
testdata/showcase/
```

ou un nouveau corpus dédié si nécessaire.

PowerShell n'est pas considéré comme moteur de rendu.

Sur Windows Terminal, le shell peut être PowerShell 7.

---

# 28. Tests de provenance

Ajouter des tests garantissant que chaque Nerd override possède :

```text
official name != ""
codepoint != ""
collection != ""
upstream != ""
license != ""
```

pour les mappings spécifiques.

Valider le format du codepoint :

```text
U+XXXX
ou
U+XXXXX
```

selon les valeurs réelles.

Valider que :

```text
glyph rune == declared codepoint
```

lorsqu'un seul codepoint est attendu.

---

# 29. Validation contre Nerd Fonts épinglé

La source de vérité du repository doit être vérifiée contre le
`glyphnames.json` officiel de la version épinglée.

Cette validation peut être :

- un script développeur ;
- une fixture vendored minimale ;
- un test généré ;
- un outil interne.

Mais :

**les tests normaux et le build ne doivent pas dépendre du réseau.**

Approche recommandée :

```text
official pinned glyphnames.json
          ↓ one-time audit/generation
minimal committed contract fixture
          ↓
Go tests offline
```

Ne pas committer inutilement plusieurs mégaoctets de données upstream si une
fixture minimale déterministe suffit.

---

# 30. Test anti-pseudo-logo

Ajouter un contrat permettant de signaler les mappings explicitement interdits
identifiés pendant l'audit.

L'objectif n'est pas de coder une opinion esthétique générale.

L'objectif est d'empêcher la réintroduction accidentelle de mappings connus
comme trompeurs.

Exemple conceptuel :

```go
func TestNerdCatalogDoesNotUseKnownMisleadingMappings(t *testing.T)
```

ou meilleure abstraction équivalente.

---

# 31. Tests de classification existants à préserver

Les suites existantes suivantes doivent continuer à passer :

```text
catalog_contract_test.go
catalog_internal_test.go
catalog_v02_compat_test.go
showcase_test.go
ascii_glyphs_test.go
presentation catalog tests
decorator tests
capabilities tests
```

Ne pas affaiblir un test existant pour faire passer v0.3.3.

Si un expected change est légitime :

- mettre à jour précisément l'assertion ;
- documenter pourquoi.

---

# 32. Compatibilité v0.2

Les tests de compatibilité v0.2 doivent être examinés avec attention.

v0.3.3 change intentionnellement certaines **résolutions de paths existants**
déjà reconnues.

Ne pas supprimer les frozen matcher identities.

Les identités suivantes restent stables :

```text
source + matcher value
```

Les promotions v0.3.3 doivent être traitées de la même manière que les
promotions SemVer-reviewed déjà documentées en v0.3.

La documentation doit distinguer :

```text
matcher identity stability
```

de :

```text
intentional semantic classification correction
```

---

# 33. Tests de counts

Après implémentation :

```go
FilenameEntryCount  == 179
DirectoryEntryCount == 74
SuffixEntryCount    == 58
ExtensionEntryCount == 195

EntryCount == 506
KindCount  == 119
RoleCount  == 16
Version    == 1
```

Si un de ces chiffres change, considérer cela comme un scope violation sauf
approbation explicite du Release Owner.

---

# 34. Tests du renderer

Ajouter ou renforcer les tests vérifiant que les modifications Nerd :

- ne changent pas les connecteurs ;
- ne changent pas les noms de fichiers ;
- ne changent pas le tri ;
- ne changent pas le modèle tree ;
- ne changent pas les sorties sans icônes ;
- ne changent pas ASCII ;
- ne changent pas Unicode.

Cas minimum :

```text
icons=never
icons=ascii
icons=unicode
icons=nerd
icons=auto without capability
icons=auto with capability
```

---

# 35. Snapshots / golden tests

Si le dépôt possède des snapshots intégrant les glyphes Nerd :

- mettre à jour uniquement les snapshots touchés ;
- relire manuellement les diffs ;
- ne pas faire d'update global sans inspection.

Une modification massive inexpliquée des snapshots est un échec de review.

---

# 36. Showcase v0.3.3

Compléter si nécessaire `testdata/showcase` afin qu'il expose visuellement :

```text
README.md
LICENSE
CHANGELOG.md

image.png
image.jpg
image.jpeg
image.svg

Dockerfile
docker-compose.yml
docker-compose.yaml
compose.yml
compose.yaml
docker-bake.hcl

main.tf
variables.tf
terraform.tfvars
.terraform.lock.hcl

Chart.yaml
values.yaml

Svelte
Astro
Dart
OCaml
Nim
D
Gleam
Elm
V
Crystal
CUE
```

Ne pas créer un fixture géant si l'existant peut être enrichi proprement.

---

# 37. THIRD_PARTY_NOTICES.md

Mettre à jour :

```text
THIRD_PARTY_NOTICES.md
```

Le document doit expliquer :

- Dirloom n'embarque aucune police ;
- Dirloom embarque uniquement des caractères/codepoints dans ses sorties ;
- version Nerd Fonts de référence ;
- collections utilisées ;
- provenance upstream ;
- licences applicables ;
- distinction entre Nerd Fonts et projets d'icônes d'origine.

Ne pas affirmer que tous les glyphes sont MDI après v0.3.3.

---

# 38. LICENSES/

Ajouter uniquement les licences réellement nécessaires.

Exemples possibles selon audit réel :

```text
LICENSES/MIT-devicons.txt
LICENSES/MIT-seti-ui.txt
...
```

Les noms exacts doivent refléter la provenance réelle.

Ne pas copier une licence “probable”.

Pour chaque fichier :

- vérifier upstream ;
- conserver le copyright approprié ;
- conserver le texte exact requis ;
- tracer la source.

Si une collection Custom contient un logo provenant d'un projet sous une
licence distincte, cette licence doit être traitée individuellement.

---

# 39. docs/catalog.md

Mettre à jour le guide du catalogue.

Ajouter une section :

```text
Nerd glyph governance
```

Elle doit expliquer :

```text
exact technology glyph
> exact semantic glyph
> generic semantic glyph
> generic fallback
```

Documenter :

- version Nerd Fonts épinglée ;
- collections approuvées ;
- absence de font embarquée ;
- provenance ;
- stratégie de fallback ;
- Unicode gelé ;
- ASCII gelé.

Documenter également les promotions v0.3.3 :

```text
docker-compose.yml
docker-compose.yaml
compose.yml
compose.yaml
docker-bake.hcl
.terraform.lock.hcl
```

---

# 40. docs/themes.md

Mettre à jour la documentation Nerd sans modifier le schéma public.

Préciser :

```text
--icons nerd
```

utilise désormais un catalogue gouverné multi-collection.

Conserver :

```text
Dirloom does not detect fonts.
Dirloom does not bundle fonts.
Explicit nerd means the user asserts compatibility.
```

Ne pas modifier :

```text
schemaVersion: 1
catalogVersion: 1
icons.spacing
```

---

# 41. docs/architecture.md

Ajouter uniquement les éléments utiles.

Documenter la séparation :

```text
classification
        ↓
semantic kind
        ↓
glyph projection
        ├── ASCII
        ├── Unicode
        └── Nerd + provenance metadata
```

Souligner que :

```text
classification != Nerd mapping
```

et qu'un refinement classificationnel est volontairement localisé au catalogue,
pas au renderer.

---

# 42. README.md

Mettre à jour le statut produit.

Le README actuellement observé conserve encore des références historiques du
type :

```text
Latest published release: v0.3.1
Current freeze: release/v0.3.2
```

alors que `v0.3.2` est déjà publiée.

Pour le travail v0.3.3, mettre le statut cohérent avec le workflow réel :

```text
Latest published release: v0.3.2
Current development/release: v0.3.3
```

Employer la formulation exacte cohérente avec la manière dont
`release/v0.3.3` est ouverte.

Mettre également à jour la roadmap courte :

```text
v0.3.3
Nerd catalog fidelity, provenance & classification refinement
```

Ne pas annoncer v0.3.3 comme publiée avant sa publication.

---

# 43. docs/release-workflow.md

Mettre à jour :

```text
Active release
```

pour `v0.3.3`.

Archiver v0.3.2 comme release publiée.

La section v0.3.3 doit respecter le workflow existant :

```text
feature → release/v0.3.3
release/v0.3.3 → main
main → annotated tag v0.3.3
tag workflow → draft
human GO → publish
```

Ne pas publier depuis la branche d'implémentation.

---

# 44. CHANGELOG.md

Ajouter sous `[Unreleased]` ou préparer la section selon l'état de la branche de
release.

Contenu attendu au minimum :

```markdown
### Fixed

- Correct Nerd glyph mappings for JPEG, SVG, LICENSE and CHANGELOG.
- Replace misleading technology pseudo-logos with verified Nerd Fonts glyphs
  or conservative semantic fallbacks.

### Changed

- Govern technology-specific Nerd glyphs using pinned, documented upstream
  collections and provenance.
- Classify Docker Compose and Docker Bake files as `manifest.container`.
- Classify `.terraform.lock.hcl` as `manifest.terraform` while preserving its
  lock and infrastructure roles.
```

Mentionner explicitement :

```text
catalogVersion remains 1
506 matchers
119 kinds
16 roles
ASCII unchanged
Unicode unchanged
```

si ces assertions ont été validées.

---

# 45. Plan v0.3.3 dans le dépôt

Créer :

```text
plans/Plan-v0.3.3-Nerd-Catalog-Fidelity-Provenance-And-Classification-Refinement.md
```

Le document doit représenter fidèlement le scope effectivement livré.

Il ne doit pas devenir une documentation fictive différente du code final.

---

# 46. Pas de runtime dependency supplémentaire inutile

Ce chantier ne justifie pas une bibliothèque externe pour gérer les glyphes.

Préférer :

```text
compiled deterministic data
```

à :

```text
runtime JSON parser
runtime Nerd registry
network lookup
font introspection
```

Toute nouvelle dépendance Go doit être justifiée explicitement.

A priori :

```text
new runtime dependency expected: none
```

---

# 47. Sécurité

Aucun code de ce chantier ne doit :

- lire la police installée ;
- scanner le système ;
- contacter Internet ;
- exécuter un binaire externe ;
- télécharger Nerd Fonts ;
- modifier une configuration utilisateur.

Les données de provenance sont build-time/repository-time uniquement.

---

# 48. Performance

Le coût de classification doit rester du même ordre.

Ne pas transformer :

```text
map lookup
```

en :

```text
linear scan over all Nerd definitions
```

sur chaque fichier.

La provenance n'a pas besoin d'être résolue pendant chaque rendu.

Les hot paths doivent continuer à utiliser des maps/registries préconstruits.

---

# 49. Déterminisme

Même entrée + même options = mêmes octets.

La v0.3.3 ne doit dépendre :

- ni de l'OS ;
- ni du terminal ;
- ni du locale ;
- ni de la version Nerd Font réellement installée ;
- ni d'un lookup réseau.

`--icons nerd` produit le catalogue Nerd compilé.

---

# 50. Matrice de tests minimum

## Catalogue

```text
Validate()
counts
kind inheritance
roles
matcher uniqueness
classification precedence
fixture v1
v0.2 compatibility
```

## Classification v0.3.3

```text
Dockerfile              → manifest.container
Containerfile           → manifest.container
docker-compose.yml      → manifest.container
docker-compose.yaml     → manifest.container
compose.yml             → manifest.container
compose.yaml            → manifest.container
docker-bake.hcl         → manifest.container

main.tf                 → manifest.terraform
variables.tf            → manifest.terraform
terraform.tfvars        → manifest.terraform
.terraform.lock.hcl     → manifest.terraform + lock
generic.hcl             → source.hcl
```

## Nerd correctness

```text
JPEG
SVG
LICENSE
CHANGELOG
Svelte
Astro
Dart
Helm
OCaml
Nim
D
Gleam
Elm
V
Crystal
CUE
```

## Channels

```text
ASCII unchanged
Unicode unchanged
Nerd expected changes
never unchanged
```

## Capability

```text
explicit nerd
explicit unicode
explicit ascii
explicit never
auto no capability
auto with capability
```

## Renderer

```text
unicode style × nerd
ascii style × nerd
spacing 0
spacing 1
spacing 4
```

---

# 51. Tests de totalité Nerd

Ajouter un test itérant sur :

```go
catalog.Kinds()
```

pour vérifier que chaque kind :

- possède un fallback cohérent ;
- a un Nerd glyph valide ou hérite correctement ;
- ne contient pas de contrôle ;
- ne contient pas d'ANSI ;
- respecte les contraintes UTF-8 existantes.

Pour les glyphes PUA :

- ne pas imposer arbitrairement un seul range si les collections officielles
  utilisées en nécessitent plusieurs ;
- valider par rapport à la table contractuelle épinglée.

---

# 52. Go quality gate local

Avant de déclarer l'implémentation terminée :

```bash
gofmt -w ./cmd ./internal
git diff --check

go mod tidy
git diff --exit-code -- go.mod go.sum

go mod verify
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/dirloom
```

Si disponibles :

```bash
golangci-lint run
govulncheck ./...
```

`go mod tidy` ne doit pas introduire de diff inexpliqué.

---

# 53. CI

La PR doit être entièrement verte.

Inclure toutes les validations déjà imposées par le dépôt :

```text
format
go mod tidy clean diff
go mod verify
vet
tests
race
build Windows
build Linux
build macOS
golangci-lint
govulncheck
completion checks
diagram parser checks
GoReleaser check
release inventory checks
```

Ne pas neutraliser une étape CI pour faire passer la branche.

---

# 54. Validation visuelle manuelle

Construire un binaire local :

```bash
go build -o ./bin/dirloom ./cmd/dirloom
```

Windows :

```powershell
go build -o .\bin\dirloom.exe .\cmd\dirloom
```

Exécuter sur le showcase :

```bash
dirloom testdata/showcase --theme vivid --icons nerd
dirloom testdata/showcase --style ascii --theme vivid --icons nerd
dirloom testdata/showcase --icons unicode
dirloom testdata/showcase --icons ascii
dirloom testdata/showcase --icons never
```

Vérifier visuellement :

- alignement ;
- pas de tofu `□` avec une Nerd Font compatible ;
- pas de glyphes de dossier sur JPEG/SVG ;
- logos technologiques exacts lorsqu'ils ont été approuvés ;
- fallback sobre ailleurs ;
- Compose/Bake/Terraform lock cohérents.

---

# 55. Non-objectifs

Explicitement hors scope :

```text
new CLI option
new icon mode
font detection
font installation
font embedding
SVG embedding
image rendering
new theme schema
catalogVersion 2
new roles
new renderer
browse/explorer
fingerprint
snapshot
verify
diff
Mosaera
Architecture Packs
```

---

# 56. Critères d'acceptation

La v0.3.3 est fonctionnellement complète lorsque :

- [ ] branche `release/v0.3.3` créée conformément au workflow ;
- [ ] développement effectué sur la branche dédiée ;
- [ ] baseline enregistrée ;
- [ ] Compose utilise `manifest.container` ;
- [ ] Docker Bake utilise `manifest.container` ;
- [ ] `.terraform.lock.hcl` utilise `manifest.terraform` ;
- [ ] rôle `lock` Terraform préservé ;
- [ ] JPEG Nerd corrigé ;
- [ ] SVG Nerd corrigé ;
- [ ] LICENSE Nerd corrigé ;
- [ ] CHANGELOG Nerd corrigé ;
- [ ] faux logos connus audités ;
- [ ] technologies utilisent vrai logo approuvé ou fallback générique ;
- [ ] version Nerd Fonts épinglée ;
- [ ] noms officiels documentés ;
- [ ] codepoints documentés ;
- [ ] collections documentées ;
- [ ] provenance documentée ;
- [ ] licences vérifiées ;
- [ ] THIRD_PARTY_NOTICES mis à jour ;
- [ ] LICENSES mis à jour ;
- [ ] table contractuelle présente ;
- [ ] audit des 119 kinds effectué ;
- [ ] 506 matchers conservés ;
- [ ] 119 kinds conservés ;
- [ ] 16 rôles conservés ;
- [ ] catalogVersion reste 1 ;
- [ ] ASCII inchangé ;
- [ ] Unicode inchangé ;
- [ ] fixture classification v1 mise à jour proprement ;
- [ ] tests de classification ajoutés ;
- [ ] tests Nerd ajoutés ;
- [ ] tests de provenance ajoutés ;
- [ ] renderer non régressé ;
- [ ] capability contract non régressé ;
- [ ] docs/catalog.md à jour ;
- [ ] docs/themes.md à jour ;
- [ ] docs/architecture.md à jour si nécessaire ;
- [ ] README à jour ;
- [ ] release-workflow à jour ;
- [ ] CHANGELOG à jour ;
- [ ] plan v0.3.3 committé ;
- [ ] gofmt OK ;
- [ ] git diff --check OK ;
- [ ] go mod verify OK ;
- [ ] go vet ./... OK ;
- [ ] go test ./... OK ;
- [ ] go test -race ./... OK ;
- [ ] build OK ;
- [ ] lint OK ;
- [ ] govulncheck OK ;
- [ ] CI complète verte ;
- [ ] validation visuelle effectuée ;
- [ ] diff final relu ;
- [ ] aucun TODO/stub/skip ;
- [ ] aucun problème restant.

---

# 57. Règle de complétude

L'agent ne doit pas répondre :

```text
"le cœur est terminé"
```

si les docs, licences, fixtures ou validations restantes ne le sont pas.

Il ne doit pas répondre :

```text
"on pourra faire la matrice visuelle plus tard"
```

si l'environnement permet de la réaliser.

Il ne doit pas reporter :

```text
provenance
licences
tests
docs
classification fixture
```

à une version ultérieure.

Le plan doit être exécuté **de bout en bout, dans son intégralité**.

---

# 58. Compte-rendu final obligatoire

À la fin du chantier, fournir exactement un rapport structuré de cette forme :

```text
Version
- v0.3.3

Branch
- release branch:
- implementation branch:

Baseline
- base branch:
- base SHA:
- Go version:

Final
- final SHA:
- PR:
- target:

Classification changes
- docker-compose.yml:
- docker-compose.yaml:
- compose.yml:
- compose.yaml:
- docker-bake.hcl:
- .terraform.lock.hcl:

Catalog invariants
- catalogVersion:
- matchers:
- filename matchers:
- directory matchers:
- suffix matchers:
- extension matchers:
- kinds:
- roles:

Nerd Fonts
- pinned version:
- source tag/commit:
- collections used:
- provenance table:
- license files added:

Nerd mappings corrected
- JPEG:
- SVG:
- LICENSE:
- CHANGELOG:
- Svelte:
- Astro:
- Dart:
- Helm:
- OCaml:
- Nim:
- D:
- Gleam:
- Elm:
- V:
- Crystal:
- CUE:

Compatibility
- ASCII:
- Unicode:
- never:
- auto without capability:
- auto with capability:
- style × icons:
- canonical machine formats:
- theme schema:
- config schema:

Tests added/updated
- ...

Documentation
- ...

Validation
- git diff --check:
- go mod tidy clean:
- go mod verify:
- gofmt:
- go vet ./...:
- go test ./...:
- go test -race ./...:
- golangci-lint:
- govulncheck:
- go build:
- CI:

Manual terminal matrix
- Windows Terminal / Mono:
- Windows Terminal / non-Mono:
- WezTerm / Mono:
- WezTerm / non-Mono:
- Alacritty / Mono:
- Alacritty / non-Mono:

Final diff review
- unexpected classification changes:
- unexpected Unicode changes:
- unexpected ASCII changes:
- unexpected dependency changes:

Remaining issues
- none
```

Si :

```text
Remaining issues
```

n'est pas :

```text
none
```

alors le plan v0.3.3 n'est pas considéré comme intégralement exécuté.

---

# 59. Résultat final attendu

Après v0.3.3 :

```text
Dirloom semantic catalog
        │
        ├── stable 506 matcher identities
        ├── stable 119 kinds
        ├── stable 16 roles
        ├── catalogVersion 1
        │
        ├── corrected conventional classification
        │     ├── Compose → manifest.container
        │     ├── Bake    → manifest.container
        │     └── Terraform lock → manifest.terraform
        │
        └── presentation projections
              ├── ASCII
              │     └── compatibility frozen
              │
              ├── Unicode
              │     └── compatibility frozen
              │
              └── Nerd
                    ├── pinned Nerd Fonts registry
                    ├── exact approved technology glyphs
                    ├── semantic MDI glyphs
                    ├── conservative generic fallbacks
                    ├── documented collections
                    ├── documented provenance
                    └── documented licences
```

Le contrat architectural reste :

```text
filesystem entry
      ↓
classification
      ↓
semantic kind + roles
      ↓
presentation projection
      ├── ASCII
      ├── Unicode
      └── Nerd
      ↓
theme
      ↓
decorator
      ↓
text renderer
```

Et l'invariant produit reste :

```text
classification gives meaning
icons project that meaning
theme styles that meaning
renderer draws the tree
```

Aucune de ces responsabilités ne doit être fusionnée.

---

# 60. Definition of Done v0.3.3

`v0.3.3` est DONE uniquement lorsque :

```text
code
+ classification refinement
+ Nerd fidelity
+ provenance
+ licences
+ tests
+ fixtures
+ docs
+ changelog
+ release metadata
+ full validation
+ manual rendering matrix
+ final review
= complete
```

Pas avant.

---

# 61. Audit additions delivered with this implementation

The 119-kind Nerd audit produced three extra mappings that stay inside the
original policy and do not add matchers, kinds, or roles:

```text
source.fsharp  md-lambda metaphor
               → nf-dev-fsharp (exact approved Devicons logo)

media family   nf-md-folder-image on a file kind
               → nf-md-image (generic semantic media glyph)

source.v
source.cue     no approved logo in Nerd Fonts v3.5.1
               → nf-md-code-braces (generic source fallback)
```

Nerd Fonts is pinned to **v3.5.1** (`glyphnames.json`, tag `v3.5.1`,
retrieved 2026-09-18). Collections actually compiled into the catalog are
Material Design Icons and Devicons only.

