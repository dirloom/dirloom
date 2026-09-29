# Dirloom v0.3.1 — Contextual Help & CLI Guidance

> **Type :** plan d'implémentation produit + technique
> **Release cible :** `v0.3.1`
> **Nature :** corrective UX + amélioration additive de la découvrabilité CLI
> **Portée :** CLI, erreurs d'utilisation, aide contextuelle, completions, tests, documentation
> **Exécution :** **ce plan doit être exécuté de bout en bout, dans son intégralité.**
> L'agent ne doit pas s'arrêter après le correctif `--icons`, ni considérer les tests ou la documentation comme optionnels.
>
> **Important :** ne pas intégrer ce chantier dans le freeze `v0.3.0`.
> Ne pas détourner `v0.4.0`, qui reste réservée au Structural Version Control : fingerprint, snapshot, verify, diff, Git/history/watch.

---

# 1. Contexte

Le comportement suivant a révélé une faiblesse d'ergonomie :

```powershell
PS> ./dirloom.exe --icons --help

Error: unsupported icon mode "--help" (expected never, unicode, nerd, or auto)
```

L'intention humaine est évidente :

```text
--icons
--help
```

mais `--icons` étant actuellement une string nécessitant explicitement une valeur, `pflag` consomme `--help` comme valeur de `--icons`.

Le problème immédiat est donc :

```text
--icons --help
        ↓
"--help" interprété comme valeur
        ↓
validation tardive
        ↓
unsupported icon mode "--help"
```

Mais le chantier ne doit pas se limiter à ce bug.

La vraie opportunité est d'améliorer le modèle d'aide de Dirloom pour passer de :

```text
generic --help
```

à :

```text
global help
    +
command help
    +
contextual topic help
    +
actionable usage errors
    +
shell discovery
```

L'objectif de v0.3.1 est donc :

> **Faire en sorte que lorsqu'un utilisateur ne sait pas exactement comment utiliser Dirloom, la CLI elle-même puisse l'orienter précisément, localement et sans ambiguïté.**

---

# 2. État du repository audité

Baseline observée au moment de la rédaction :

```text
repository: github.com/dirloom/dirloom
default branch: main
language: Go
CLI framework: Cobra / pflag

v0.2.0:
    publiée

v0.3.0:
    implémentation terminée
    freeze release/v0.3.0
    release candidate fusionnée vers main
    cérémonie de publication encore indépendante
```

Le repository possède déjà :

```text
internal/cli/root.go
internal/cli/root_test.go
internal/cli/completion.go
internal/cli/completion_test.go
internal/cli/documentation_test.go

docs/
docs/product/
docs/release-workflow.md
docs/product/roadmap.md
docs/configuration.md
docs/themes.md
docs/presets.md
docs/clipboard-and-completions.md

README.md
CHANGELOG.md
```

Le CLI possède déjà :

* `usageError` ;
* codes de sortie stabilisés :

  * `0` succès ;
  * `1` runtime ;
  * `2` arguments invalides ;
* `SilenceErrors: true` ;
* `SilenceUsage: true` ;
* `SetFlagErrorFunc(...)` ;
* aide Cobra ;
* sous-commandes ;
* completions shell ;
* config explicable ;
* presets explicables ;
* themes explicables ;
* tests d'aide ;
* tests de présentation ;
* séparation canonical/presentation.

Ces invariants doivent être conservés.

---

# 3. Diagnostic technique précis

Aujourd'hui :

```go
command.Flags().StringVar(
    &opts.icons,
    "icons",
    "",
    "terminal icons: never, unicode, nerd, or auto",
)
```

Puis ultérieurement :

```go
if command.Flags().Changed("icons") {
    if opts.icons == "" {
        return configuration.Overrides{},
            &usageError{
                err: fmt.Errorf("--icons requires a non-empty value"),
            }
    }

    result.Icons = configuration.Optional[string]{
        Set:   true,
        Value: opts.icons,
    }
}
```

Conséquence :

```text
dirloom --icons --help
```

devient essentiellement :

```text
icons = "--help"
```

avant validation.

Ce comportement est techniquement cohérent avec pflag mais ergonomiquement incorrect pour Dirloom.

---

# 4. Décision de release

## 4.1 Version cible

Implémenter dans :

```text
v0.3.1
```

Nom de travail possible :

```text
v0.3.1 — CLI Guidance & Contextual Help
```

ou simplement :

```text
v0.3.1 — CLI ergonomics
```

## 4.2 Pourquoi pas v0.3.0

`v0.3.0` est déjà gelée.

Ne pas rouvrir le scope.

Le correctif ne doit pas être injecté dans :

```text
release/v0.3.0
```

sauf décision humaine exceptionnelle du Release Owner annulant explicitement le freeze.

Ce plan considère donc cette option comme interdite.

## 4.3 Pourquoi pas v0.4.0

`v0.4.0` possède déjà une mission structurante :

```text
observe
   ↓
identify
   ↓
freeze
   ↓
verify
   ↓
compare
   ↓
track evolution
```

avec notamment :

```text
fingerprint
snapshot
verify
diff
move detection
Git sources
history
watch
```

L'aide CLI n'a aucune raison de polluer cette frontière produit.

## 4.4 Sémantique

Même si certains éléments de v0.3.1 sont additifs, ils constituent principalement :

```text
bugfix UX
+
discoverability improvement
+
CLI guidance refinement
```

et restent alignés avec la surface CLI/presentation de v0.3.

---

# 5. Objectifs produit

À la fin du chantier, Dirloom doit supporter trois niveaux d'aide.

## Niveau 1 — découverte globale

```bash
dirloom --help
```

Répond à :

> Qu'est-ce que Dirloom et quelles sont ses commandes/capacités principales ?

---

## Niveau 2 — aide d'une commande

```bash
dirloom config --help
dirloom theme --help
dirloom preset --help
dirloom completion --help
```

ainsi que :

```bash
dirloom help config
dirloom help theme
```

Répond à :

> Comment utiliser cette commande ?

---

## Niveau 3 — aide conceptuelle ciblée

```bash
dirloom help icons
dirloom help colors
dirloom help formats
dirloom help filters
dirloom help configuration
dirloom help themes
dirloom help presets
dirloom help diagrams
dirloom help output
```

Répond à :

> Comment fonctionne ce concept dans Dirloom ?

---

# 6. Invariants de v0.3.1

Les invariants suivants sont obligatoires.

## 6.1 Help reste canonique

Aucune aide ne doit dépendre de :

```text
terminal detection
theme
color
icon mode
Nerd Font
TTY
NO_COLOR
```

Les aides doivent rester :

```text
UTF-8
stable
portable
sans ANSI
sans glyphe décoratif dépendant du terminal
```

La règle actuelle :

> diagnostics, help et erreurs restent canoniques

doit rester vraie.

---

## 6.2 Aucun impact sur l'artefact structurel

Cette release ne doit modifier :

* ni scanner ;
* ni tri ;
* ni modèle de tree ;
* ni JSON ;
* ni Markdown ;
* ni diagram documents ;
* ni catalogue sémantique ;
* ni thèmes ;
* ni moteur de configuration ;
* ni futurs contrats v0.4.

---

## 6.3 Exit codes inchangés

Conserver :

```text
0 = success
1 = runtime error
2 = usage / invalid arguments
```

Les améliorations de message ne doivent pas casser ce contrat.

---

## 6.4 Pas d'aide distante

Tous les topics sont :

```text
compiled
local
deterministic
offline
```

Pas :

* HTTP ;
* ouverture automatique du navigateur ;
* téléchargement ;
* documentation cloud obligatoire.

---

## 6.5 Une seule source logique par topic

Éviter que :

```text
help icons
README
themes.md
flag description
```

contiennent quatre définitions contradictoires.

Les textes courts CLI sont propres au CLI.

La documentation longue reste dans `docs/`.

Les invariants fonctionnels doivent être cohérents entre les deux.

---

# 7. Increment 1 — Corriger immédiatement `--icons`

## 7.1 Ajouter une valeur implicite

Configurer :

```text
--icons
```

comme équivalent de :

```text
--icons=auto
```

Le mécanisme attendu avec pflag est :

```go
flag := command.Flags().Lookup("icons")
flag.NoOptDefVal = "auto"
```

ou une abstraction locale équivalente si elle améliore la lisibilité.

Résultat attendu :

```bash
dirloom --icons
```

équivaut à :

```bash
dirloom --icons=auto
```

---

## 7.2 Cas initial désormais valide

La commande :

```bash
dirloom --icons --help
```

doit afficher l'aide et retourner :

```text
exit code 0
stderr empty
```

`--help` ne doit plus être consommé comme valeur.

---

## 7.3 Préserver les valeurs explicites

Ces commandes doivent continuer à fonctionner :

```bash
dirloom --icons=never
dirloom --icons=unicode
dirloom --icons=nerd
dirloom --icons=auto
```

et également la syntaxe traditionnelle :

```bash
dirloom --icons never
dirloom --icons unicode
dirloom --icons nerd
dirloom --icons auto
```

---

# 8. Increment 2 — Appliquer le même principe à `--color`

Évaluer et implémenter :

```bash
dirloom --color
```

comme alias de :

```bash
dirloom --color=auto
```

Motivation :

`icons` et `color` appartiennent au même modèle conceptuel :

```text
never
explicit
auto
```

Une asymétrie :

```text
--icons        => auto
--color        => error
```

serait difficile à justifier.

Configurer donc également :

```go
colorFlag.NoOptDefVal = "auto"
```

sauf obstacle technique identifié et documenté.

Tests obligatoires.

---

# 9. Ne PAS généraliser aveuglément les valeurs implicites

Ne pas appliquer `NoOptDefVal` à tous les flags string.

En particulier :

```text
--theme
--preset
--format
--style
--diagram-view
--diagram-direction
--config
```

doivent continuer à nécessiter une valeur explicite.

Exemple incorrect :

```text
--theme
```

ne doit pas devenir silencieusement :

```text
--theme=default
```

car l'intention utilisateur n'est pas suffisamment claire.

La règle doit être documentée :

> Une option string ne possède une valeur implicite que lorsqu'une forme booléenne naturelle et non ambiguë existe.

Pour v0.3.1 :

```text
--icons  => auto
--color  => auto
```

---

# 10. Increment 3 — Introduire un registry de Help Topics

Créer une abstraction légère.

Nom recommandé :

```text
internal/cli/help_topics.go
```

Éviter une architecture générique disproportionnée.

Une structure suffisante :

```go
type helpTopic struct {
    Name        string
    Aliases     []string
    Summary     string
    Body        string
    SeeAlso     []string
}
```

ou équivalent.

Registry :

```go
var helpTopics = []helpTopic{
    ...
}
```

Exigences :

* ordre déterministe ;
* noms uniques ;
* aliases uniques ;
* lookup case-sensitive ou politique explicitement documentée ;
* aucune map utilisée comme source d'ordre d'affichage ;
* pas de dépendance réseau ;
* pas d'I/O disque ;
* aucune mutation globale.

---

# 11. Topics v0.3.1 obligatoires

Ne pas créer des dizaines de topics.

Introduire une première surface cohérente.

## 11.1 `icons`

```bash
dirloom help icons
```

Doit expliquer :

```text
never
unicode
nerd
auto
```

Et notamment :

* `--icons` seul = `auto` ;
* `auto` n'active pas Nerd Font automatiquement ;
* `auto` utilise Unicode seulement lorsque l'environnement est éligible ;
* thème seul ≠ activation d'icônes ;
* formats machine/canoniques restent non décorés ;
* exemples.

---

## 11.2 `colors`

```bash
dirloom help colors
```

Expliquer :

```text
never
always
auto
```

et :

```text
NO_COLOR
TTY
explicit CLI override
canonical outputs
```

---

## 11.3 `themes`

```bash
dirloom help themes
```

Expliquer la différence entre :

```text
theme
color
icons
```

Très important pour réduire la confusion.

Exemples :

```bash
dirloom --theme vivid
dirloom --theme vivid --icons
dirloom --theme midnight --icons nerd
dirloom theme list
dirloom theme explain vivid
```

---

## 11.4 `formats`

```bash
dirloom help formats
```

Présenter clairement :

```text
text
markdown
markdown-tree
json
mermaid
graphviz
d2
```

Séparer :

```text
human presentation
canonical/machine output
diagram source
```

---

## 11.5 `filters`

```bash
dirloom help filters
```

Regrouper :

```text
--depth
--dirs-only
--hidden
--ignore
--no-default-ignore
--no-gitignore
```

Expliquer brièvement l'ordre de filtrage sans dupliquer toute la documentation.

---

## 11.6 `configuration`

```bash
dirloom help configuration
```

Présenter :

```text
CLI
project config
user config
defaults
```

et la priorité :

```text
CLI > project > user > built-in
```

Référencer :

```bash
dirloom config explain
```

---

## 11.7 `presets`

```bash
dirloom help presets
```

Présenter :

```text
ai
compact
docs
monorepo
none
```

et :

```bash
dirloom preset explain <preset>
```

---

## 11.8 `diagrams`

```bash
dirloom help diagrams
```

Présenter :

```text
mermaid
graphviz
d2
diagram-view
diagram-direction
diagram-max-nodes
```

et préciser que Dirloom produit le DSL, pas le PNG/SVG lui-même.

---

## 11.9 `output`

```bash
dirloom help output
```

Présenter :

```text
stdout
--output
--copy
pipe
redirection
```

ainsi que les garanties principales :

```text
transactional output
stdout cleanliness
canonical behavior
```

---

# 12. Increment 4 — Étendre proprement `dirloom help`

Cobra possède déjà son mécanisme d'aide de commandes.

Ne pas créer :

```text
help-command
+
help-topic-command
```

comme deux systèmes concurrents.

Le contrat utilisateur doit rester :

```bash
dirloom help <name>
```

avec résolution :

```text
1. command
2. help topic
3. unknown-help-target diagnostic
```

Exemples :

```bash
dirloom help theme
```

→ aide de la commande Cobra `theme`.

```bash
dirloom help icons
```

→ topic `icons`.

---

# 13. `dirloom help topics`

Ajouter :

```bash
dirloom help topics
```

Sortie cible :

```text
Help topics:

  colors          Terminal color behavior
  configuration   Configuration resolution and precedence
  diagrams        Mermaid, Graphviz and D2 exports
  filters         Depth, ignore rules and visibility
  formats         Text, Markdown, JSON and diagram formats
  icons           Unicode, Nerd Font and automatic icons
  output          stdout, clipboard and transactional files
  presets         Built-in project-tree presets
  themes          Built-in and custom terminal themes

Run 'dirloom help <topic>' for details.
```

L'ordre doit être stable.

Préférer l'ordre alphabétique sauf raison UX forte explicitement testée.

---

# 14. Increment 5 — Standardiser les diagnostics des valeurs énumérées

Actuellement plusieurs erreurs suivent le modèle :

```text
unsupported X "foo" (expected ...)
```

Introduire une présentation plus actionnable pour les valeurs CLI connues.

Exemple :

```text
Error: invalid value "foo" for --icons

Valid values:
  auto
  unicode
  nerd
  never

Run 'dirloom help icons' for details.
```

Conserver :

```text
exit code 2
stdout empty
stderr diagnostic
```

---

# 15. Ne pas transformer toutes les erreurs runtime

Le mécanisme de guidance doit cibler les erreurs d'utilisation.

Il ne doit pas modifier artificiellement :

```text
permission denied
filesystem error
broken pipe
output write failure
clipboard runtime failure
theme file read failure
```

La distinction existante :

```text
usageError
runtime error
```

doit rester forte.

---

# 16. Increment 6 — Suggestions sur cible d'aide inconnue

Pour :

```bash
dirloom help icon
```

si `icons` est la seule correspondance raisonnable :

```text
Error: unknown help topic or command "icon"

Did you mean:
  icons

Run 'dirloom help topics' to list available topics.
```

Pour :

```bash
dirloom help formts
```

suggérer :

```text
formats
```

Utiliser autant que possible les mécanismes Cobra existants pour les commandes.

Pour les topics, utiliser un mécanisme simple et déterministe.

Éviter d'introduire une nouvelle dépendance uniquement pour Levenshtein.

Une petite implémentation locale est acceptable si nécessaire.

---

# 17. Increment 7 — Suggestions ciblées depuis certaines erreurs

Lorsqu'une erreur correspond directement à un concept documenté, ajouter une piste d'aide.

Exemple :

```bash
dirloom --icons=foobar
```

Sortie :

```text
Error: invalid value "foobar" for --icons

Valid values:
  auto
  unicode
  nerd
  never

Run 'dirloom help icons' for details.
```

Pour :

```bash
dirloom --format wat
```

proposer :

```text
Run 'dirloom help formats' for details.
```

Pour une incompatibilité diagram :

```text
Run 'dirloom help diagrams' for details.
```

Ne pas ajouter un `Run ...` artificiel à chaque erreur imaginable.

---

# 18. Increment 8 — Mettre à niveau les descriptions courtes des flags

La root help doit montrer clairement les valeurs implicites.

Cible :

```text
--icons [auto]    terminal icons: never, unicode, nerd, or auto
--color [auto]    terminal colors: never, always, or auto
```

Le rendu exact dépend de pflag/Cobra.

Vérifier le résultat réel.

L'aide doit rendre compréhensible que :

```text
--icons
```

est valide.

Si pflag expose automatiquement la valeur implicite de manière satisfaisante, ne pas recréer manuellement le formatage.

---

# 19. Increment 9 — Completions des help topics

Les shells actuellement supportés doivent également découvrir :

```bash
dirloom help <TAB>
```

avec au minimum :

```text
commands
+
topics
```

Shells concernés selon l'existant :

```text
bash
zsh
fish
PowerShell
```

Les completions doivent conserver les garanties existantes.

Ajouter les tests nécessaires dans :

```text
internal/cli/completion_test.go
```

---

# 20. Increment 10 — Architecture recommandée

Organisation suggérée :

```text
internal/cli/
├── root.go
├── help.go
├── help_topics.go
├── help_test.go
├── completion.go
├── completion_test.go
├── options.go
└── ...
```

Répartition :

## `root.go`

Conserve :

* composition root CLI ;
* registration commands/flags ;
* execution wiring.

Ne pas gonfler davantage `root.go`.

Il fait déjà presque 20 KB.

---

## `help_topics.go`

Contient :

* type `helpTopic` ;
* registry ;
* lookup ;
* validation éventuelle du registry.

---

## `help.go`

Contient :

* integration Cobra ;
* help command ;
* rendering topic ;
* suggestions ;
* formatting commun des diagnostics help.

---

## `help_test.go`

Contient les contrats dédiés.

Éviter d'ajouter encore des centaines de lignes dans :

```text
root_test.go
```

qui dépasse déjà largement 40 KB.

---

# 21. Increment 11 — Registry validation

Ajouter un test qui garantit :

```text
topic names unique
aliases unique
no empty topic name
no empty summary
no empty body
no dangling SeeAlso
stable ordering if expected
```

Exemple conceptuel :

```go
func TestHelpTopicRegistryIsValid(t *testing.T)
```

Le registry étant compilé, ces erreurs doivent être détectées à la CI et non au runtime.

---

# 22. Increment 12 — Tests obligatoires : `--icons`

Ajouter au minimum les scénarios suivants.

## Implicit auto

```text
dirloom ROOT --no-config --icons
```

doit être équivalent à :

```text
dirloom ROOT --no-config --icons=auto
```

dans le même environnement terminal simulé.

---

## Explicit modes

Tester :

```text
--icons never
--icons unicode
--icons nerd
--icons auto
```

---

## `--help` non consommé

Test critique de régression :

```text
args:
    --icons
    --help

expect:
    code = 0
    stderr = ""
    stdout contains "Usage:"
```

---

## Invalid mode

```text
--icons invalid
```

doit :

```text
exit = 2
stdout = ""
stderr contains valid values
stderr contains "dirloom help icons"
```

---

# 23. Tests obligatoires : `--color`

Mêmes garanties :

```text
--color
--color=auto
--color auto
--color never
--color always
--color invalid
--color --help
```

---

# 24. Tests obligatoires : help topics

Tester chaque topic public :

```text
help icons
help colors
help themes
help formats
help filters
help configuration
help presets
help diagrams
help output
```

Pour chacun :

```text
code = 0
stderr = ""
stdout != ""
stdout deterministic
stdout without ANSI
```

---

# 25. Tests obligatoires : command vs topic resolution

Tester :

```text
dirloom help theme
```

doit continuer à viser la commande `theme`.

Tester :

```text
dirloom help icons
```

doit viser le topic.

Un topic ne doit jamais masquer une commande publique.

Ajouter un invariant :

> Command names have precedence over help-topic names.

---

# 26. Tests obligatoires : help topic list

Tester :

```bash
dirloom help topics
```

Garantir :

* topics présents ;
* pas de doublons ;
* ordre déterministe ;
* aucun ANSI ;
* newline final ;
* code 0.

---

# 27. Tests obligatoires : suggestions

Tester au minimum :

```text
help icon     -> icons
help formts   -> formats
```

Et un cas suffisamment éloigné :

```text
help banana
```

qui ne doit pas produire une suggestion absurde.

---

# 28. Tests obligatoires : canonical help

Créer un test transversal :

```text
Help remains presentation-neutral
```

Exécuter par exemple :

```bash
dirloom --theme vivid --icons nerd --color always --help
```

La sortie d'aide doit rester :

```text
sans ANSI
sans Nerd glyph
sans décoration
```

Même principe pour :

```bash
dirloom help icons
```

avec les flags globaux disponibles si leur position/scope le permet.

---

# 29. Tests obligatoires : config interactions

Vérifier que l'aide n'entraîne pas inutilement :

* découverte de `.dirloom.yaml` ;
* chargement du user config ;
* lecture d'un thème ;
* scan du filesystem.

`dirloom help icons` doit être purement informationnel.

Un fichier de configuration cassé dans le working directory ne devrait pas empêcher :

```bash
dirloom --help
dirloom help icons
```

si le comportement Cobra actuel le permet.

Cette propriété doit être explicitement testée.

---

# 30. Tests obligatoires : exit codes

Matrice minimale :

| Commande                     |                               Exit |
| ---------------------------- | ---------------------------------: |
| `dirloom --help`             |                                `0` |
| `dirloom help icons`         |                                `0` |
| `dirloom help topics`        |                                `0` |
| `dirloom --icons`            | `0` si invocation autrement valide |
| `dirloom --icons bad`        |                                `2` |
| `dirloom help unknown-topic` |                                `2` |
| filesystem runtime failure   |                                `1` |

Ne pas régresser le contrat public.

---

# 31. Documentation

La documentation est obligatoire et fait partie de l'implémentation.

Mettre à jour au minimum :

```text
README.md
CHANGELOG.md
docs/product/roadmap.md
docs/clipboard-and-completions.md
docs/themes.md
```

Créer éventuellement :

```text
docs/contextual-help.md
```

si la matière dépasse raisonnablement une section de la documentation CLI existante.

Je recommande sa création.

---

# 32. `docs/contextual-help.md`

Contenu attendu :

```text
# Contextual Help

## Three help levels

dirloom --help
dirloom <command> --help
dirloom help <topic>

## Available topics

...

## Optional-value flags

--icons[=MODE]
--color[=MODE]

## Error guidance

...

## Shell completion

...
```

Conserver une documentation concise.

Le fichier ne doit pas devenir une duplication ligne-à-ligne du texte compilé.

---

# 33. README

Ajouter une section courte.

Exemple :

```bash
# General help
dirloom --help

# Command help
dirloom theme --help

# Concept help
dirloom help icons
dirloom help formats
dirloom help filters

# List help topics
dirloom help topics
```

Documenter :

```bash
dirloom --icons
```

comme raccourci de :

```bash
dirloom --icons=auto
```

Même chose pour `--color` si retenu.

---

# 34. CHANGELOG

Après publication de v0.3.0, préparer :

```markdown
## [Unreleased]

### Added

- Contextual CLI help topics through `dirloom help <topic>`.
- Shell completion for contextual help topics.
- Actionable guidance for supported enumerated CLI values.

### Changed

- `--icons` without an explicit value now means `--icons=auto`.
- `--color` without an explicit value now means `--color=auto`.

### Fixed

- `dirloom --icons --help` no longer consumes `--help` as the icon mode.
```

Adapter à la convention exacte du CHANGELOG existant.

---

# 35. Roadmap

Ne pas transformer ce chantier en nouveau milestone stratégique.

Dans la roadmap, enregistrer simplement :

```text
v0.3.1 — CLI guidance / contextual help
```

comme raffinement post-v0.3 avant v0.4.

Préserver :

```text
v0.3 PRESENTATION
v0.3.1 CLI GUIDANCE
v0.4 CHANGE
```

ou une représentation équivalente.

---

# 36. Documentation tests

Le repository contient déjà :

```text
internal/cli/documentation_test.go
```

Étendre les tests de cohérence documentation si pertinent.

Vérifier notamment que les modes publics documentés restent synchronisés :

```text
icons:
    never
    unicode
    nerd
    auto

color:
    never
    always
    auto
```

Éviter un test fragile sur toute la prose.

Tester les contrats, pas la ponctuation.

---

# 37. Amélioration supplémentaire A — `help examples`

Ajouter le topic :

```bash
dirloom help examples
```

**uniquement si l'implémentation reste petite.**

Il peut présenter des workflows plutôt qu'une liste de flags :

```bash
# Inspect current project
dirloom

# Compact overview
dirloom --preset compact

# Pretty interactive terminal view
dirloom --theme vivid --icons

# Markdown for documentation
dirloom --format markdown

# Semantic Markdown tree
dirloom --format markdown-tree

# Machine contract
dirloom --format json

# Mermaid
dirloom --format mermaid

# Copy
dirloom --format markdown --copy
```

Ce topic est intéressant car il répond à :

> « Je veux faire X, quelle commande dois-je lancer ? »

alors que `--help` répond surtout :

> « Quels flags existent ? »

Si ajouté, il doit faire partie des tests et completions.

---

# 38. Amélioration supplémentaire B — `dirloom help concepts`

Optionnel mais recommandé si cela reste simple :

```bash
dirloom help concepts
```

Afficher un index logique :

```text
Presentation
  icons
  colors
  themes

Structure
  filters
  formats

Configuration
  configuration
  presets

Output
  output
  diagrams
```

Ne pas créer de hiérarchie complexe.

Ce n'est qu'un index.

`help topics` reste la liste canonique exhaustive.

---

# 39. Amélioration supplémentaire C — hints sobres après certaines erreurs

Envisager un mécanisme générique minimal :

```go
type guidedUsageError struct {
    err       error
    helpTopic string
}
```

ou enrichir `usageError` :

```go
type usageError struct {
    err       error
    helpTopic string
}
```

Puis le renderer d'erreur peut produire :

```text
Error: ...

Run 'dirloom help <topic>' for details.
```

Avantages :

* logique centralisée ;
* pas de chaînes copiées partout ;
* extensible pour v0.4 ;
* exit code inchangé.

Mais :

> Ne pas introduire cette abstraction si elle rend le système plus complexe que les quelques cas actuels.

Le but est une architecture légère.

---

# 40. Préparer v0.4 sans l'implémenter

Le registry de help topics doit permettre ultérieurement :

```text
dirloom help fingerprint
dirloom help snapshots
dirloom help verify
dirloom help diff
dirloom help history
```

Mais **ne pas créer ces topics maintenant** tant que les fonctionnalités n'existent pas publiquement.

Une aide ne doit jamais documenter une capacité comme disponible avant son implémentation.

---

# 41. Frontière avec `config explain`, `preset explain`, `theme explain`

Ne pas confondre :

```text
help
```

et :

```text
explain
```

Le contrat conceptuel doit être :

```text
help
    teaches how a capability works

explain
    reports concrete resolved state or definition
```

Exemples :

```bash
dirloom help configuration
```

→ explique le modèle.

```bash
dirloom config explain
```

→ montre les valeurs réellement résolues.

---

```bash
dirloom help presets
```

→ explique le concept des presets.

```bash
dirloom preset explain ai
```

→ inspecte le preset concret `ai`.

---

```bash
dirloom help themes
```

→ explique le système.

```bash
dirloom theme explain vivid
```

→ inspecte le thème `vivid`.

Cette distinction doit apparaître dans la documentation.

---

# 42. Frontière avec v0.4

Le chantier v0.3.1 ne doit introduire aucune notion structurelle future telle que :

```text
snapshot model
fingerprint algorithm
diff state
move detection
Git source abstraction
history storage
watch engine
```

Il peut seulement construire une infrastructure d'aide réutilisable.

---

# 43. Backward compatibility

Doit rester valide :

```bash
dirloom --icons auto
dirloom --icons unicode
dirloom --icons nerd
dirloom --icons never
```

Aucun script existant ne doit devoir être modifié.

Le nouveau comportement :

```bash
dirloom --icons
```

est purement additif.

Même règle pour `--color`.

---

# 44. Quality gates

L'agent doit exécuter la totalité des validations applicables au repository.

Au minimum :

```bash
gofmt -w ./cmd ./internal

go vet ./...

go test ./...

go test -race ./...

go build ./cmd/dirloom
```

Puis les contrôles utilisés par la CI du projet, notamment :

```text
golangci-lint
govulncheck
completion syntax checks
GoReleaser check
cross-platform CI
```

Ne pas considérer :

```text
go test ./...
```

comme validation suffisante.

---

# 45. Smoke tests binaires

Construire réellement le binaire et exécuter des smoke tests.

Sous Windows notamment :

```powershell
.\dirloom.exe --help
.\dirloom.exe --icons --help
.\dirloom.exe --icons
.\dirloom.exe --icons=auto
.\dirloom.exe --icons=unicode
.\dirloom.exe --icons=nerd
.\dirloom.exe --icons=never

.\dirloom.exe --color --help

.\dirloom.exe help icons
.\dirloom.exe help colors
.\dirloom.exe help themes
.\dirloom.exe help formats
.\dirloom.exe help filters
.\dirloom.exe help configuration
.\dirloom.exe help presets
.\dirloom.exe help diagrams
.\dirloom.exe help output
.\dirloom.exe help topics

.\dirloom.exe --icons=foobar
.\dirloom.exe help formts
```

Faire l'équivalent sur Linux/macOS via CI.

---

# 46. Snapshot de l'expérience utilisateur

Avant merge, capturer dans le PR les sorties réelles de :

```text
dirloom --help
dirloom help topics
dirloom help icons
dirloom --icons --help
dirloom --icons=invalid
```

Cela permet une review produit, pas seulement technique.

---

# 47. Critères d'acceptation fonctionnels

Le chantier n'est pas terminé tant que tous ces critères ne sont pas vrais.

## AC-01

```bash
dirloom --icons --help
```

retourne :

```text
exit 0
help on stdout
nothing on stderr
```

---

## AC-02

```bash
dirloom --icons
```

équivaut à :

```bash
dirloom --icons=auto
```

---

## AC-03

```bash
dirloom --color
```

équivaut à :

```bash
dirloom --color=auto
```

si aucune incompatibilité sérieuse n'est identifiée.

---

## AC-04

```bash
dirloom help icons
```

fonctionne.

---

## AC-05

```bash
dirloom help topics
```

liste les topics disponibles.

---

## AC-06

L'aide d'une commande existante reste accessible.

---

## AC-07

Une commande a priorité sur un topic de même nom.

---

## AC-08

Les topics sont proposés par les shell completions.

---

## AC-09

Les valeurs invalides des principaux enums produisent un diagnostic actionnable.

---

## AC-10

Les erreurs usage restent exit `2`.

---

## AC-11

Les erreurs runtime restent exit `1`.

---

## AC-12

Aucun help output ne contient de séquence ANSI.

---

## AC-13

Aucune aide ne nécessite scanner/config/theme/network.

---

## AC-14

Les formats canoniques existants sont byte-identical à la baseline hors changements explicitement attendus.

---

## AC-15

Tous les tests et quality gates passent.

---

# 48. Definition of Done

Le chantier est **Done** uniquement lorsque :

```text
[ ] bug --icons --help corrigé
[ ] --icons implicit auto livré
[ ] --color implicit auto évalué et livré sauf justification contraire documentée
[ ] help topic registry livré
[ ] dirloom help topics livré
[ ] topics obligatoires livrés
[ ] command/topic precedence livrée
[ ] invalid-value guidance livrée
[ ] suggestions raisonnables livrées
[ ] completions help topics livrées
[ ] tests unitaires livrés
[ ] tests intégration CLI livrés
[ ] regression tests livrés
[ ] canonical help tests livrés
[ ] exit-code tests livrés
[ ] README mis à jour
[ ] contextual-help doc livrée
[ ] themes docs mises à jour
[ ] completion docs mises à jour
[ ] roadmap mise à jour
[ ] changelog mis à jour
[ ] gofmt OK
[ ] go vet OK
[ ] go test ./... OK
[ ] go test -race ./... OK
[ ] golangci-lint OK
[ ] govulncheck OK
[ ] build OK
[ ] completion checks OK
[ ] GoReleaser check OK
[ ] CI Windows OK
[ ] CI Linux OK
[ ] CI macOS OK
[ ] smoke tests binaires OK
```

**Aucun item code/test/docs de cette liste n'est optionnel.**

---

# 49. Stratégie Git recommandée

Ne pas modifier :

```text
release/v0.3.0
```

pour ce chantier.

Après sécurisation de la cérémonie v0.3.0, travailler depuis la baseline appropriée de `main`.

Branche recommandée :

```text
feat/v0.3.1-contextual-help
```

ou :

```text
fix/v0.3.1-cli-guidance
```

Je préfère :

```text
feat/v0.3.1-contextual-help
```

car le chantier contient à la fois :

```text
bugfix
+
new contextual help capability
```

---

# 50. Découpage commits recommandé

Maintenir des commits atomiques.

```text
fix(cli): support implicit auto icon and color modes
```

```text
feat(cli): add contextual help topic registry
```

```text
feat(cli): add actionable usage guidance
```

```text
feat(completion): complete contextual help topics
```

```text
test(cli): freeze contextual help contracts
```

```text
docs: document contextual help and v0.3.1 ergonomics
```

Adapter si certains commits doivent être fusionnés pour préserver un état toujours vert.

---

# 51. PR attendue

Titre recommandé :

```text
feat(cli): add contextual help and guided usage for v0.3.1
```

La description doit inclure :

```text
Problem
Architecture
Behavior changes
Backward compatibility
Help topics
Diagnostics
Tests
Documentation
Smoke evidence
Release impact
```

Inclure le cas original :

```text
Before:
dirloom --icons --help
→ Error: unsupported icon mode "--help"

After:
dirloom --icons --help
→ normal help, exit 0
```

---

# 52. Non-goals

Ne pas profiter du chantier pour implémenter :

* TUI `browse` ;
* fuzzy finder interactif ;
* man pages générées ;
* website docs generator ;
* HTTP docs ;
* telemetry ;
* AI assistant ;
* automatic browser opening ;
* v0.4 structural commands ;
* refonte complète de Cobra ;
* framework générique de documentation ;
* nouvelle dépendance lourde ;
* redesign du Visual Theme Engine.

---

# 53. Principe architectural final

Le modèle cible devient :

```text
                         ┌──────────────────┐
                         │   dirloom help   │
                         └────────┬─────────┘
                                  │
             ┌────────────────────┼────────────────────┐
             │                    │                    │
             ▼                    ▼                    ▼
      Global discovery      Command help       Concept help
        --help              help theme          help icons
                              │                    │
                              │                    │
                              └────────┬───────────┘
                                       │
                                       ▼
                              Actionable guidance
                                       │
                                       ▼
                              Correct CLI usage
```

et :

```text
invalid CLI input
       │
       ▼
concise diagnostic
       │
       ├── valid values
       │
       ├── sensible suggestion
       │
       └── targeted help command
```

---

# 54. Résultat produit attendu

Avant :

```text
Error: unsupported icon mode "--help"
(expected never, unicode, nerd, or auto)
```

Après :

```text
PS> dirloom --icons --help

Dirloom turns a directory into a clean, deterministic and shareable tree...

Usage:
  ...
```

Et lorsqu'un utilisateur veut comprendre réellement l'option :

```text
PS> dirloom help icons

ICON MODES

Controls how Dirloom renders icons in terminal output.

Usage:
  dirloom [command] --icons[=MODE]

Modes:
  auto       Automatically use portable icons when supported
  unicode    Use portable Unicode icons
  nerd       Use Nerd Font glyphs
  never      Disable icons

Examples:
  dirloom --icons
  dirloom --icons=unicode
  dirloom --theme vivid --icons
  dirloom --theme midnight --icons=nerd

Notes:
  --icons without a value is equivalent to --icons=auto.
  Auto mode never assumes Nerd Font support.
  Canonical and machine-oriented outputs remain undecorated.

See also:
  dirloom help themes
  dirloom help colors
  dirloom help formats
```

---

# 55. Instruction finale à l'agent

**Exécuter ce plan de bout en bout.**

Ne pas :

```text
1. corriger --icons ;
2. constater que le test passe ;
3. s'arrêter.
```

Le travail demandé comprend obligatoirement :

```text
implementation
+
refactoring ciblé
+
contextual help
+
diagnostics
+
completions
+
tests
+
regressions
+
documentation
+
changelog
+
roadmap
+
quality gates
+
binary smoke tests
```

Si un point du plan s'avère techniquement inadapté au repository réel :

1. conserver l'objectif produit ;
2. choisir l'implémentation la plus simple respectant les invariants ;
3. documenter explicitement la déviation ;
4. ajouter les tests prouvant le comportement retenu ;
5. poursuivre l'intégralité du plan.

Ne pas demander une validation intermédiaire pour chaque incrément.

La livraison attendue est une implémentation **Code Complete**, testée, documentée et prête pour le processus de release `v0.3.1`.
