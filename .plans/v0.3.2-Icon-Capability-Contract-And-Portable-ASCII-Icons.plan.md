# Dirloom v0.3.2 — Icon Capability Contract & Portable ASCII Icons

> **Release cible :** `v0.3.2`
> **Nature :** raffinement PRESENTATION — contrat d'icônes, capability Nerd déclarative, catalogue ASCII portable
> **Important :** ne pas intégrer ce chantier dans le freeze `v0.3.0`.
> Ne pas détourner `v0.4.0`, qui reste réservée au Structural Version Control : fingerprint, snapshot, verify, diff.

## 0. Mandat d'exécution

Ce plan est **normatif**.

L'agent doit l'exécuter **de bout en bout, dans son intégralité**, y compris :

- création de la branche dédiée ;
- audit de confirmation de l'état de départ ;
- implémentation ;
- migrations internes nécessaires ;
- tests unitaires ;
- tests d'intégration ;
- tests de contrat ;
- tests de non-régression ;
- mises à jour des fixtures/goldens nécessaires ;
- documentation utilisateur ;
- documentation de configuration ;
- documentation du catalogue ;
- changelog ;
- validation Linux / Windows / macOS via les tests compatibles ;
- lint, vet, race, vulnérabilités et build final.

Il est interdit de considérer le travail terminé avec :

- un TODO ;
- un stub ;
- un test désactivé ;
- une fonctionnalité documentée mais non implémentée ;
- une fonctionnalité implémentée mais non documentée ;
- une modification du contrat sans test de non-régression ;
- un comportement spécifique à Windows non couvert par un test injecté/déterministe ;
- une implémentation partielle reportée à une version ultérieure.

Si un détail du plan entre en conflit avec l'état réel du dépôt, l'agent doit :

1. préserver les invariants produit définis dans ce document ;
2. adapter l'implémentation à l'architecture réellement présente ;
3. documenter clairement l'écart dans son compte-rendu final ;
4. ne pas réduire silencieusement le périmètre.

La release GitHub, le tag et la publication vers Scoop/Homebrew/Winget ne font **pas** partie de cette branche d'implémentation. Ils restent soumis au workflow de release Dirloom.

---

# 1. Version et branche

## Version cible

```text
v0.3.2
```

`v0.4.0` n'est pas retenue.

Tant que Dirloom reste en `0.x` :

```text
0.Y.0
    = product milestone / capability boundary
      ex. PRESENTATION, CHANGE, MATERIALIZE

0.Y.Z
    = backward-compatible maintenance release within that milestone:
      bug fixes, UX refinements, CLI ergonomics,
      documentation/discoverability improvements,
      and small additive capabilities that do not redefine the milestone.
```

Trajectoire :

```text
v0.3.0
PRESENTATION

      ↓

v0.3.1
PRESENTATION refinement
Contextual Help & CLI Guidance

      ↓

v0.3.2
PRESENTATION refinement
Icon Capability Contract & Portable ASCII Icons

      ↓

v0.4.0
CHANGE
Fingerprint + Snapshot + Verify + Diff
```

Cette évolution reste dans le jalon PRESENTATION. Elle ajoute des capacités publiques rétrocompatibles, sans redéfinir le jalon ni ouvrir CHANGE :

- nouveau mode `--icons ascii` ;
- nouveau comportement public de `--icons auto` ;
- capability Nerd Font déclarative ;
- variable d'environnement publique ;
- extension additive du fichier de configuration ;
- extension additive des icônes de thèmes.

`v0.4.0` reste réservée au Structural Version Control : fingerprint, snapshot, verify, diff.

## Branche d'implémentation

Créer depuis le `main` à jour :

```bash
git fetch --prune origin
git switch main
git pull --ff-only origin main
git switch -c feat/v0.3.2-icon-capabilities
```

Ne pas développer directement sur :

```text
main
release/v0.3.0
```

La future branche de gel/release sera :

```text
release/v0.3.2
```

mais elle ne doit pas être créée pendant ce chantier sauf instruction explicite du Release Owner.

---

# 2. Baseline obligatoire avant modification

Avant tout changement :

```bash
git status --short
git rev-parse HEAD
go version
go mod verify
go test ./...
go vet ./...
go build ./...
```

Enregistrer dans le compte-rendu final :

```text
base_branch
base_commit
go_version
baseline_tests
baseline_vet
baseline_build
```

La branche doit partir d'un workspace propre.

Si la baseline est rouge avant modification, arrêter l'implémentation et identifier précisément le problème au lieu de l'attribuer à ce chantier.

---

# 3. État actuel à préserver

L'implémentation existante possède déjà les responsabilités suivantes :

```text
config
  résout les préférences utilisateur/projet/CLI

presentation.CapabilityEvaluator
  résout les capacités terminal au moment de la sortie

presentation catalog
  classifie les nœuds et leur associe Unicode/Nerd

presentation.Decorator
  choisit l'icône effective et décore le nom

render
  contrôle la géométrie de l'arbre
```

Ne pas fusionner ces responsabilités.

En particulier :

```text
--style
```

et :

```text
--icons
```

doivent rester strictement orthogonaux.

---

# 4. Contrat public v0.3.2

Le contrat définitif devient :

```text
--icons never
  Aucun préfixe iconographique.

--icons ascii
  Catalogue strictement ASCII.
  Aucun caractère > U+007E.
  Aucun Unicode requis.
  Aucun PUA.
  Aucun emoji.
  Aucun besoin de Nerd Font.

--icons unicode
  Catalogue Unicode portable.
  Aucun Private Use Area.
  Aucun besoin de Nerd Font.

--icons nerd
  Catalogue Nerd Fonts / PUA.
  La sélection explicite affirme que l'environnement
  d'affichage est compatible.

--icons auto
  Résolution conservatrice :
    capability Nerd explicitement déclarée -> nerd
    sinon                              -> unicode
```

Valeurs publiques acceptées :

```text
never
ascii
unicode
nerd
auto
```

## Invariant fondamental

```text
style != icons
```

`--style` contrôle uniquement les connecteurs et branches.

Exemples :

```text
|--
`--
```

ou :

```text
├──
└──
```

`--icons` contrôle uniquement le marqueur sémantique précédant le nom du nœud.

Les combinaisons suivantes doivent donc toutes rester valides :

```bash
dirloom . --style ascii   --icons never
dirloom . --style ascii   --icons ascii
dirloom . --style ascii   --icons unicode
dirloom . --style ascii   --icons nerd

dirloom . --style unicode --icons never
dirloom . --style unicode --icons ascii
dirloom . --style unicode --icons unicode
dirloom . --style unicode --icons nerd
```

Aucun mode `style` ne doit implicitement changer `icons`.

Aucun mode `icons` ne doit implicitement changer `style`.

---

# 5. Conserver le défaut historique

Le défaut global de Dirloom reste :

```yaml
presentation:
  icons: never
```

Ne pas remettre `auto` comme valeur par défaut.

Cette décision est importante pour préserver :

- les sorties historiques ;
- les scripts ;
- les snapshots ;
- les fichiers produits sans demande explicite de décoration.

La v0.3.2 modifie le comportement de `auto`, mais **pas le défaut `never`**.

---

# 6. Nouveau comportement de `auto`

## 6.1 Décorréler icônes et couleur

Le code actuel partage indirectement la notion d'`autoEligible` entre couleur et icônes.

Après v0.3.2 :

```text
color auto
```

continue à dépendre de :

- TTY ;
- CI ;
- `TERM=dumb` ;
- redirection ;
- `--output` ;
- clipboard ;
- préparation ANSI.

Mais :

```text
icons auto
```

ne doit plus dépendre de ces heuristiques.

Pseudo-code cible :

```go
if !outputformat.UsesPresentation(request.Format) {
    return IconsNever
}

switch request.IconMode {
case IconsNever:
    return IconsNever

case IconsASCII:
    return IconsASCII

case IconsUnicode:
    return IconsUnicode

case IconsNerd:
    return IconsNerd

case IconsAuto:
    if nerdFontCapability(request, environment) {
        return IconsNerd
    }
    return IconsUnicode

default:
    return invalidIconMode()
}
```

Les facteurs suivants ne constituent **jamais** une preuve de présence d'une Nerd Font :

```text
runtime.GOOS == windows
Windows Terminal
WT_SESSION
TERM_PROGRAM
COLORTERM
TERM=xterm-256color
TTY
ANSI
truecolor
Unicode disponible
terminal moderne
```

Aucune heuristique de ce type ne doit influencer `IconsAuto`.

---

# 7. Capability Nerd Font déclarative

## 7.1 Principe

Dirloom ne détecte pas une police.

Dirloom consomme une **capability déclarée**.

Le vocabulaire code/documentation doit privilégier :

```text
declared
configured
effective
```

et éviter :

```text
detected
font detected
Nerd Font found
```

sauf si une réelle détection fiable est un jour implémentée.

## 7.2 Variable d'environnement

Ajouter :

```text
DIRLOOM_NERD_FONT
```

Valeurs vraies acceptées, insensibles à la casse :

```text
1
true
yes
on
```

Valeurs fausses :

```text
0
false
no
off
```

Unset ou chaîne vide :

```text
capability non déclarée
```

Une valeur non vide invalide doit générer une erreur utilisateur explicite lorsqu'elle est pertinente pour `icons=auto`.

Exemple :

```text
invalid DIRLOOM_NERD_FONT value "maybe"
expected one of: 1, true, yes, on, 0, false, no, off
```

Ne pas faire échouer :

```bash
DIRLOOM_NERD_FONT=invalid dirloom --icons never
```

ou :

```bash
DIRLOOM_NERD_FONT=invalid dirloom --icons unicode
```

puisque la capability n'est pas consultée dans ces modes.

## 7.3 Configuration utilisateur

Ajouter au schéma `.dirloom.yaml` :

```yaml
terminal:
  capabilities:
    nerdFont: true
```

Utiliser `nerdFont`, et non `nerd_font`, pour rester cohérent avec les clés camelCase déjà utilisées par Dirloom.

Cette capability représente la **machine/utilisateur courant**, pas un attribut du projet.

### Règle de sécurité/portabilité

Elle est autorisée dans :

```text
<user-config-dir>/dirloom/config.yaml
```

Elle est interdite dans :

```text
<project>/.dirloom.yaml
```

et dans un fichier fourni comme configuration projet via :

```text
--config
```

Une config projet contenant :

```yaml
terminal:
  capabilities:
    nerdFont: true
```

doit échouer avec un message actionnable du type :

```text
terminal.capabilities.nerdFont is a user/host capability
and cannot be declared by project configuration
```

Raison :

> un dépôt ne doit jamais pouvoir annoncer à la place de l'utilisateur que son terminal possède une Nerd Font.

## 7.4 Priorité

Pour `--icons auto` :

```text
DIRLOOM_NERD_FONT
        ↓
user config terminal.capabilities.nerdFont
        ↓
false / capability absente
```

Donc :

```text
env=true  + config=false -> nerd
env=false + config=true  -> unicode
unset     + config=true  -> nerd
unset     + config=false -> unicode
unset     + absent       -> unicode
```

`--no-user-config` désactive naturellement la capability provenant du user config.

`--no-config` désactive naturellement la capability provenant des fichiers de configuration.

La variable d'environnement reste une déclaration runtime indépendante des fichiers.

---

# 8. Modes explicites prioritaires

Une capability ne doit influencer **que** `auto`.

Exemples obligatoires :

```text
DIRLOOM_NERD_FONT=0
--icons nerd
=> nerd
```

```text
DIRLOOM_NERD_FONT=1
--icons unicode
=> unicode
```

```text
DIRLOOM_NERD_FONT=1
--icons ascii
=> ascii
```

```text
DIRLOOM_NERD_FONT=1
--icons never
=> never
```

Autrement dit :

```text
requested icon mode != auto
    => aucune résolution automatique
```

---

# 9. Formats machine

Le comportement existant reste prioritaire :

```text
format sans couche presentation
    => IconsNever
```

Même avec :

```bash
DIRLOOM_NERD_FONT=1
dirloom . --format json --icons nerd
```

un format canonique/machine ne doit pas recevoir de décoration terminal si son contrat actuel l'interdit.

Ne modifier aucun contrat JSON, diagramme ou format canonique pour satisfaire ce chantier.

---

# 10. Catalogue ASCII

## 10.1 Étendre le modèle de glyphes

Le catalogue ne doit plus être conceptuellement limité à une paire :

```go
Unicode
Nerd
```

Introduire un modèle de type :

```go
type GlyphSet struct {
    ASCII   string
    Unicode string
    Nerd    string
}
```

ou équivalent adapté au code existant.

Éviter une API du type :

```go
Glyphs(kind) (ascii, unicode, nerd string)
```

si un objet structuré réduit les erreurs et prépare mieux l'évolution future.

Le classifier reste totalement indépendant du mode d'icônes.

Architecture cible :

```text
filesystem node
      ↓
semantic classification
      ↓
Kind
      ↓
GlyphSet
 ┌────┼────────┐
ASCII Unicode Nerd
```

## 10.2 Catalogue ASCII de base

Utiliser des marqueurs courts, explicites et de largeur homogène.

Base recommandée :

```text
file       [FI]
source     [SC]
manifest   [MF]
data       [DT]
document   [DC]
media      [ME]
archive    [AR]
font       [FT]
binary     [BN]
directory  [DR]
symlink    [LN]
```

Tous sont :

- ASCII imprimable ;
- quatre octets ;
- quatre colonnes dans un terminal ASCII normal ;
- sans ambiguïté forte entre familles.

Les sous-kinds héritent de leur famille par défaut.

Exemple :

```text
source.go
source.rust
source.python
```

peuvent tous produire :

```text
[SC]
```

dans v0.3.2.

Ne pas chercher à reproduire toute la richesse Nerd avec un alphabet ASCII cryptique.

Le contrat ASCII privilégie :

```text
portabilité
lisibilité
stabilité
```

plutôt que :

```text
maximum de variété
```

## 10.3 Fallback

Tous les modes doivent posséder un fallback défini.

Hiérarchie sémantique existante :

```text
exact kind
   ↓
parent kind
   ↓
generic file
```

Pour ASCII :

```text
aucun fallback Unicode autorisé
```

Un mode strictement ASCII ne doit jamais produire un caractère non ASCII.

Pour Nerd, conserver le comportement actuel :

```text
Nerd glyph
   ↓ si absent
Unicode glyph
   ↓ si absent
aucun glyph
```

sauf si un test existant démontre qu'un autre contrat public est déjà garanti.

---

# 11. Extension additive des thèmes

Le système de thème v1 possède actuellement les canaux :

```yaml
icons:
  unicode:
  nerd:
```

L'étendre additivement avec :

```yaml
icons:
  ascii:
  unicode:
  nerd:
```

## 11.1 Compatibilité

Ne pas casser les thèmes v1 existants.

Un thème existant :

```yaml
icons:
  unicode: "◇"
  nerd: "..."
```

doit continuer à fonctionner sans modification.

`ascii` est facultatif.

Utiliser `omitempty` dans les contrats JSON publics lorsque nécessaire afin de ne pas introduire artificiellement :

```json
"ascii": ""
```

dans des diagnostics qui ne l'exposaient pas auparavant.

Ne pas incrémenter `schemaVersion` uniquement pour l'ajout d'un champ facultatif rétrocompatible, sauf si les tests/contrats existants démontrent explicitement qu'un ajout de champ est considéré comme une nouvelle version de schéma.

## 11.2 `null`

Étendre la sémantique actuelle :

```yaml
icons:
  ascii: null
```

doit supprimer le glyph ASCII hérité au niveau de binding concerné, exactement comme :

```yaml
icons:
  unicode: null
icons:
  nerd: null
```

le font actuellement pour leurs canaux.

## 11.3 Validation

Un glyph ASCII personnalisé doit :

- être une chaîne UTF-8 valide ;
- contenir uniquement U+0020 à U+007E ;
- ne contenir aucun contrôle ;
- ne contenir aucun ESC ;
- respecter la limite existante de longueur/runes.

Un glyph ASCII contenant :

```text
é
→
📁
U+Exxx
```

doit être refusé.

---

# 12. Contrat Unicode

Ne pas profiter de cette version pour redessiner le catalogue Unicode.

Les glyphes actuels sont considérés comme compatibility-frozen.

La v0.3.2 doit cependant formaliser par tests que les glyphes Unicode intégrés :

- sont du UTF-8 valide ;
- ne contiennent pas de Private Use Area ;
- ne contiennent aucun caractère de contrôle ;
- ne contiennent aucune séquence ZWJ ;
- ne contiennent aucun variation selector emoji ;
- ne dépendent pas d'une Nerd Font.

Tester au minimum les trois plages PUA :

```text
U+E000–U+F8FF
U+F0000–U+FFFFD
U+100000–U+10FFFD
```

Les glyphes Nerd peuvent naturellement utiliser les PUA.

---

# 13. Modifications attendues dans le code

Les chemins exacts peuvent évoluer pendant l'implémentation, mais auditer et modifier au minimum les zones suivantes.

## `internal/presentation/types.go`

Ajouter :

```go
IconsASCII = "ascii"
```

et :

```go
IconModes() == [
    never,
    ascii,
    unicode,
    nerd,
    auto,
]
```

Faire évoluer le modèle `IconPair` vers un modèle à trois canaux si nécessaire.

## `internal/presentation/capabilities.go`

Séparer explicitement :

```text
color auto eligibility
```

de :

```text
icon auto resolution
```

Ajouter la résolution de `DIRLOOM_NERD_FONT`.

Ajouter à `CapabilityRequest` l'information issue de la user config nécessaire à la résolution Nerd.

Ne jamais inspecter :

```text
WT_SESSION
TERM_PROGRAM
nom de police
Windows Registry
Windows Terminal settings.json
```

pour décider du mode Nerd.

## `internal/presentation/catalog/*`

Étendre la définition d'un kind avec son glyph ASCII.

Conserver :

- le classifier ;
- les parents ;
- les rôles ;
- les matchers ;
- les 119 kinds actuels ;
- la compatibilité du catalogue v1.

Ne pas ajouter ou retirer des matchers uniquement pour cette feature.

## `internal/presentation/compile.go`

Compiler le canal ASCII.

Propager les overrides de thème ASCII.

Conserver la provenance des icônes.

## `internal/presentation/decorator.go`

Ajouter :

```go
case IconsASCII:
    icon = style.icons.ASCII
```

et préserver les comportements Unicode/Nerd existants.

## `internal/presentation/yaml_decode.go`

Accepter :

```yaml
icons:
  ascii:
```

avec la même sémantique nullable que les deux autres canaux.

## `internal/config/*`

Ajouter :

```yaml
terminal:
  capabilities:
    nerdFont: true|false
```

avec :

- parsing strict ;
- provenance ;
- limitation user-config-only ;
- diagnostics ;
- tests de superposition.

## `internal/cli/root.go`

Mettre à jour l'aide :

```text
terminal icons: never, ascii, unicode, nerd, or auto
```

Conserver les mécanismes de completion existants et les faire inclure `ascii`.

---

# 14. `config explain`

Rendre la capability configurée observable.

Exemple texte :

```text
Terminal:
  capabilities.nerdFont: true (user: ...)
```

ou structure équivalente cohérente avec l'affichage existant.

Le diagnostic doit distinguer :

```text
configuration
```

de :

```text
résolution runtime
```

Ne pas écrire :

```text
Nerd Font detected
```

`config explain` n'a pas besoin de connaître la police active.

Pour le JSON diagnostic, si l'ajout modifie un contrat machine stable :

1. vérifier la politique de version du diagnostic existant ;
2. incrémenter uniquement son `schemaVersion` si nécessaire ;
3. ajouter les tests de compatibilité correspondants.

Ne pas modifier silencieusement un JSON déclaré stable.

---

# 15. Pas de `doctor` dans cette release

Ne pas ajouter :

```text
dirloom doctor
dirloom doctor icons
```

dans v0.3.2.

Le besoin est légitime, mais il constitue une commande publique supplémentaire avec son propre contrat et mérite un chantier distinct.

La v0.3.2 doit fournir toutes les primitives propres permettant de l'ajouter ensuite :

```text
capability déclarative
résolution déterministe
provenance
catalogues séparés
```

Ne pas gonfler ce chantier au-delà du contrat d'icônes.

---

# 16. Tests unitaires obligatoires

## 16.1 `IconModes`

Tester exactement :

```text
never
ascii
unicode
nerd
auto
```

et le rejet de toute autre valeur.

## 16.2 Capability evaluator

Construire une table couvrant au minimum :

| Requested | Nerd config | Env | TTY | Destination | Expected |
|---|---:|---:|---:|---|---|
| auto | absent | absent | yes | stdout | unicode |
| auto | absent | absent | no | pipe | unicode |
| auto | absent | absent | yes | output file | unicode |
| auto | absent | absent | yes | clipboard | unicode |
| auto | absent | absent | yes | CI | unicode |
| auto | absent | absent | yes | TERM=dumb | unicode |
| auto | true | absent | yes | stdout | nerd |
| auto | true | absent | no | pipe | nerd |
| auto | false | absent | yes | stdout | unicode |
| auto | false | true | yes | stdout | nerd |
| auto | true | false | yes | stdout | unicode |
| nerd | false | false | any | any | nerd |
| unicode | true | true | any | any | unicode |
| ascii | true | true | any | any | ascii |
| never | true | true | any | any | never |

Ajouter une preuve explicite :

```text
Windows Terminal-like environment
+ aucune capability Nerd
=> unicode
```

Même avec :

```text
WT_SESSION=<non-empty>
COLORTERM=truecolor
TERM=xterm-256color
```

la résolution reste :

```text
unicode
```

## 16.3 Formats machine

Tester que les formats non-présentation produisent :

```text
IconsNever
```

même avec :

```text
DIRLOOM_NERD_FONT=1
IconMode=nerd
```

## 16.4 Env parsing

Tester :

```text
1
true
TRUE
yes
YES
on
ON
```

et :

```text
0
false
FALSE
no
NO
off
OFF
```

Tester valeur invalide.

Tester que la valeur invalide n'est pas consultée pour les modes explicites.

---

# 17. Tests configuration obligatoires

Ajouter au minimum :

```text
user config nerdFont=true
user config nerdFont=false
absence de capability
project config nerdFont=true -> erreur
project config nerdFont=false -> erreur
--config avec capability terminal -> erreur
--no-user-config
--no-config
env override user true -> false
env override user false -> true
```

Tester la provenance user.

Ne pas permettre qu'une `.dirloom.yaml` versionnée dans un repo affirme une capability machine.

---

# 18. Tests catalogue obligatoires

Pour chaque `KindDefinition` :

- ASCII non vide ;
- ASCII printable strict ;
- ASCII sans Unicode ;
- Unicode valide ;
- Unicode hors PUA ;
- Nerd valide ;
- parent existant ;
- fallback fonctionnel.

Ajouter un test de couverture :

```text
every registered kind resolves an ASCII glyph
every registered kind resolves a Unicode glyph
every registered kind resolves a Nerd glyph or its documented fallback
```

Le catalogue doit rester déterministe.

Ne pas modifier :

```text
KindCount
matcher identities
classifier precedence
catalogVersion
```

sans nécessité directement démontrée par ce chantier.

---

# 19. Tests thèmes obligatoires

Tester :

```yaml
icons:
  ascii: "[X]"
```

sur :

- token ;
- kind ;
- role ;
- rule.

Tester :

```yaml
icons:
  ascii: null
```

et l'héritage.

Tester le rejet de :

```yaml
icons:
  ascii: "→"
```

et :

```yaml
icons:
  ascii: "📁"
```

Tester qu'un ancien thème v1 ne possédant que :

```yaml
unicode:
nerd:
```

reste accepté sans modification.

---

# 20. Tests renderer / décorateur

Ajouter des tests couvrant :

```text
style=ascii + icons=ascii
style=ascii + icons=unicode
style=ascii + icons=nerd

style=unicode + icons=ascii
style=unicode + icons=unicode
style=unicode + icons=nerd
```

Vérifier que changer `style` ne change pas le glyph d'icône.

Vérifier que changer `icons` ne change pas les branches.

Ajouter une golden ASCII-icons dédiée si cela rend le contrat plus lisible.

Exemple attendu conceptuel :

```text
[DR] project/
├── [DR] src/
│   └── [SC] main.go
└── [DC] README.md
```

et en style ASCII :

```text
[DR] project/
|-- [DR] src/
|   `-- [SC] main.go
`-- [DC] README.md
```

Les marqueurs exacts doivent suivre le catalogue défini au §10.

---

# 21. Tests CLI

Mettre à jour les tests de :

```text
--help
invalid --icons
config override
completion bash
completion zsh
completion fish
completion powershell
```

Les completions doivent proposer :

```text
never
ascii
unicode
nerd
auto
```

Ajouter un test CLI de bout en bout :

```bash
dirloom <fixture> --no-config --style ascii --icons ascii
```

et vérifier qu'aucun octet >= 0x80 n'apparaît dans la sortie.

C'est un invariant très important du mode ASCII.

---

# 22. Tests de non-régression

Les comportements suivants doivent rester inchangés :

```text
default icons = never
explicit --icons unicode
explicit --icons nerd
NO_COLOR behavior
color auto TTY behavior
theme colors
semantic classification
canonical JSON
markdown
markdown-tree
mermaid
graphviz
d2
clipboard
output files
```

Les goldens existants ne doivent être modifiés que lorsqu'un changement est explicitement attendu par ce plan.

Une mise à jour massive de goldens sans justification est interdite.

---

# 23. Documentation

Mettre à jour au minimum :

```text
README.md
docs/themes.md
docs/configuration.md
docs/catalog.md
CHANGELOG.md
```

et toute documentation produit dont les tests indiquent qu'elle constitue un contrat.

## README

Présenter succinctement :

```bash
dirloom --icons ascii
dirloom --icons unicode
dirloom --icons nerd
dirloom --icons auto
```

## `docs/themes.md`

Remplacer le contrat actuel de `auto`.

Documenter clairement :

```text
auto does not detect fonts
auto does not infer Nerd support from the terminal
```

Documenter :

```text
DIRLOOM_NERD_FONT
```

Documenter `icons.ascii` dans le thème v1.

## `docs/configuration.md`

Documenter :

```yaml
terminal:
  capabilities:
    nerdFont: true
```

Préciser très visiblement :

```text
USER CONFIG ONLY
```

et expliquer pourquoi les configs projet ne peuvent pas définir cette capability.

Documenter la priorité :

```text
environment > user config > safe fallback
```

## `docs/catalog.md`

Ajouter le canal ASCII.

Publier le mapping des familles :

```text
file       [FI]
source     [SC]
manifest   [MF]
data       [DT]
document   [DC]
media      [ME]
archive    [AR]
font       [FT]
binary     [BN]
directory  [DR]
symlink    [LN]
```

Préciser que :

```text
Unicode = portable
Nerd    = richer PUA catalog
ASCII   = maximum portability
```

## Changelog

Pendant l'implémentation :

```markdown
## [Unreleased]

### Added
- Add strict ASCII semantic icons...
- Add declarative Nerd Font capability...

### Changed
- Resolve `--icons auto` conservatively...
```

Ne pas dater ni créer artificiellement `[0.3.2]` avant le freeze/release si le workflow du projet réserve cette opération à la branche de release.

---

# 24. Documentation : cas réel à expliquer

Ajouter un exemple proche de :

```text
Windows Terminal + Cascadia/Caskaydia non-Nerd
  --icons nerd
  => glyphes PUA non rendus : responsabilité du mode explicite

Windows Terminal + JetBrainsMono Nerd Font
  --icons nerd
  => glyphes Nerd rendus

Windows Terminal + n'importe laquelle de ces polices
  --icons auto
  + aucune capability déclarée
  => unicode

Windows Terminal + JetBrainsMono Nerd Font
  DIRLOOM_NERD_FONT=1
  --icons auto
  => nerd
```

Le but est de démontrer explicitement :

```text
terminal capability != font capability
```

---

# 25. Validation manuelle Windows

Avec Windows Terminal et PowerShell.

## Sans capability

```powershell
Remove-Item Env:DIRLOOM_NERD_FONT -ErrorAction SilentlyContinue

.\dirloom.exe . --icons auto
```

Attendu :

```text
unicode
```

et jamais Nerd automatiquement.

## Capability déclarée

```powershell
$env:DIRLOOM_NERD_FONT = "1"
.\dirloom.exe . --icons auto
```

Avec JetBrainsMono Nerd Font :

```text
glyphes Nerd correctement rendus
```

## Override false

```powershell
$env:DIRLOOM_NERD_FONT = "0"
.\dirloom.exe . --icons auto
```

Attendu :

```text
unicode
```

## Explicite Nerd

```powershell
$env:DIRLOOM_NERD_FONT = "0"
.\dirloom.exe . --icons nerd
```

Attendu :

```text
nerd
```

## ASCII strict

```powershell
.\dirloom.exe . --style unicode --icons ascii
.\dirloom.exe . --style ascii   --icons ascii
```

Vérifier visuellement l'orthogonalité.

---

# 26. Gates qualité locales

Avant de considérer le chantier terminé :

```bash
go mod verify

go mod tidy
git diff --exit-code go.mod go.sum
```

Si `go.mod` ou `go.sum` changent volontairement, expliquer pourquoi.

Puis :

```bash
gofmt -w cmd internal
git diff --check

go vet ./...
go test ./...
go test -race ./...
go build -trimpath ./cmd/dirloom
```

Exécuter aussi le lint configuré par le repo :

```text
golangci-lint
```

et :

```text
govulncheck
```

selon les versions/pins du workflow CI actuel.

Aucun warning nouveau pertinent ne doit être ignoré.

---

# 27. CI obligatoire

La PR doit être verte sur le workflow existant :

```text
Verify Ubuntu
Verify Windows
Verify macOS
Race detector
Lint
Vulnerability scan
Diagram syntax compatibility
Pin check
Release snapshot
```

Ne pas modifier les workflows uniquement pour faire passer un test.

Toute modification de workflow doit être techniquement justifiée par la feature elle-même.

---

# 28. Critères d'acceptation produit

La feature est acceptée uniquement si tous les points suivants sont vrais.

### A. Modes publics

```text
never   OK
ascii   OK
unicode OK
nerd    OK
auto    OK
```

### B. Auto sûr

Sans capability :

```text
auto -> unicode
```

toujours pour un format presentation-enabled.

Avec capability :

```text
auto -> nerd
```

### C. Pas de magie

Ces éléments seuls ne déclenchent jamais Nerd :

```text
Windows
Windows Terminal
WT_SESSION
TERM_PROGRAM
COLORTERM
TTY
truecolor
```

### D. Portabilité projet

Un repo ne peut pas déclarer :

```text
terminal.capabilities.nerdFont
```

pour la machine de son utilisateur.

### E. ASCII strict

```text
--icons ascii
```

ne produit aucun caractère hors ASCII dans ses icônes.

### F. Orthogonalité

```text
style
```

et :

```text
icons
```

sont totalement indépendants.

### G. Compatibilité

Le défaut reste :

```text
icons=never
```

et les formats machine restent inchangés.

### H. Documentation

CLI, config, thèmes et catalogue décrivent exactement le comportement implémenté.

---

# 29. Definition of Done

Le chantier n'est terminé que lorsque :

- [ ] branche `feat/v0.3.2-icon-capabilities` créée depuis `main` à jour ;
- [ ] baseline enregistrée ;
- [ ] `IconsASCII` implémenté ;
- [ ] catalogue ASCII complet avec fallback ;
- [ ] canal ASCII supporté par les thèmes ;
- [ ] `DIRLOOM_NERD_FONT` implémenté ;
- [ ] user capability `terminal.capabilities.nerdFont` implémentée ;
- [ ] capability interdite dans la config projet ;
- [ ] `auto` découplé de `autoEligible` couleur ;
- [ ] `auto -> unicode` sans déclaration ;
- [ ] `auto -> nerd` avec déclaration ;
- [ ] aucun sniffing de terminal/police ;
- [ ] défaut `icons=never` conservé ;
- [ ] tests capability complets ;
- [ ] tests config complets ;
- [ ] tests catalog complets ;
- [ ] tests thèmes complets ;
- [ ] tests renderer complets ;
- [ ] tests CLI/completion complets ;
- [ ] tests de non-régression verts ;
- [ ] documentation mise à jour ;
- [ ] changelog `[Unreleased]` mis à jour ;
- [ ] `go mod verify` vert ;
- [ ] `go vet ./...` vert ;
- [ ] `go test ./...` vert ;
- [ ] `go test -race ./...` vert ;
- [ ] lint vert ;
- [ ] vuln scan vert ;
- [ ] build vert ;
- [ ] CI Linux vert ;
- [ ] CI Windows vert ;
- [ ] CI macOS vert ;
- [ ] workspace final propre hors modifications attendues ;
- [ ] diff final relu ;
- [ ] aucun TODO/stub/test skip introduit.

---

# 30. Compte-rendu obligatoire de l'agent

À la fin, l'agent doit fournir exactement un récapitulatif comprenant :

```text
Branch
Base SHA
Final SHA

Implementation
- ...

Tests added/updated
- ...

Documentation
- ...

Compatibility
- default icons
- auto behavior
- machine formats
- theme schema
- config schema

Validation
- go mod verify
- gofmt
- go vet ./...
- go test ./...
- go test -race ./...
- golangci-lint
- govulncheck
- go build
- CI status

Manual smoke
- ASCII
- Unicode
- Nerd
- Auto without capability
- Auto with capability

Remaining issues
- none
```

Si `Remaining issues` n'est pas `none`, le plan n'est pas considéré comme exécuté intégralement.

---

# 31. Résultat fonctionnel attendu

Exemple sans capability :

```bash
dirloom . --icons auto
```

équivaut côté sélection d'icônes à :

```bash
dirloom . --icons unicode
```

Exemple avec :

```bash
export DIRLOOM_NERD_FONT=1
```

alors :

```bash
dirloom . --icons auto
```

équivaut côté sélection d'icônes à :

```bash
dirloom . --icons nerd
```

mais :

```bash
dirloom . --icons unicode
```

reste Unicode,

```bash
dirloom . --icons ascii
```

reste ASCII,

et :

```bash
dirloom . --icons never
```

reste sans icônes.

Le contrat peut donc être résumé par :

```text
explicit choice wins
declared capability enables enhancement
absence of knowledge produces the portable choice
```

et jamais par :

```text
modern terminal => Nerd
Windows => Nerd
Unicode => Nerd
guess the user's font
```