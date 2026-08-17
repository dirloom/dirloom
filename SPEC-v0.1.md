> **Statut :**  Spécification consolidée — décisions v0.1 figées

**Cible initiale :** v0.1.0

**Type :** CLI cross-platform

**Langage :** Go

**Binaire :** `dirloom` / `dirloom.exe`

**Plateformes cibles :** Windows, Linux, macOS



---

# 1. Vision

**Dirloom** est un outil CLI permettant de transformer l'arborescence réelle d'un répertoire en une représentation textuelle propre, déterministe, filtrable et directement exploitable dans : 

- un terminal ; 
- une documentation Markdown ; 
- un README ; 
- une issue ou Pull Request ; 
- un prompt destiné à un LLM ; 
- un agent de développement ; 
- une CI/CD ; 
- un fichier ou autre pipeline automatisé. 

Dirloom ne doit pas être pensé comme une simple réécriture de la commande `tree`. 

La vision à long terme est : 

> **Transformer la structure d'un projet en une représentation portable, lisible et exploitable aussi bien par les humains que par les outils.** 



Le MVP doit rester volontairement simple, mais son architecture doit permettre cette évolution sans réécriture majeure. 

---

# 2. Problème à résoudre

Aujourd'hui, lorsqu'un développeur souhaite partager la structure d'un projet, il doit généralement : 

1. utiliser `tree` ou une commande spécifique à son OS ; 
2. nettoyer manuellement la sortie ; 
3. supprimer `node_modules`, `.git`, `dist`, caches et autres répertoires inutiles ; 
4. limiter manuellement la profondeur ; 
5. adapter le résultat au Markdown ; 
6. copier le résultat ; 
7. éventuellement le retraiter avant de le transmettre à un agent IA. 

Le résultat varie également selon le système d'exploitation et les outils installés. 

Dirloom doit fournir une commande unique, cohérente et portable. 

Exemple : 

```bash
dirloom
```

Sortie :

```text
my-project/
├── src/
│   ├── features/
│   │   ├── occupancy/
│   │   ├── revenues/
│   │   └── new-feature/
│   │       ├── components/
│   │       ├── hooks/
│   │       │   └── useNewFeature.ts
│   │       ├── new-feature.service.ts
│   │       ├── new-feature.model.ts
│   │       ├── new-feature.schema.ts
│   │       ├── new-feature.types.ts
│   │       └── index.ts
│   └── main.ts
├── package.json
└── README.md
```

---

# 3. Principes produit

Dirloom doit respecter les principes suivants.

## 3.1 Simple par défaut

La commande :

```bash
dirloom
```

doit être immédiatement utile sans configuration préalable.

Elle analyse le répertoire courant.

---

## 3.2 Puissant lorsque nécessaire

Les comportements avancés doivent être accessibles via des options explicites :

```bash
dirloom src --depth 4
```

```bash
dirloom --format markdown
```

```bash
dirloom --ignore "*.log" --ignore tmp --ignore coverage
```

---

## 3.3 Déterministe

À contenu identique, Dirloom doit produire une sortie identique.

Cela implique notamment :

- ordre stable ;
- représentation stable ;
- normalisation des chemins ;
- comportement prévisible entre plateformes.

> Les garanties de déterminisme s'appliquent, à options et contenu filesystem équivalents, entre les plateformes supportées pour une même version de Dirloom.

---

## 3.4 Cross-platform

Aucun comportement métier ne doit dépendre de PowerShell.

Le binaire doit fonctionner depuis : PowerShell, `cmd.exe`, Windows Terminal, bash, zsh, fish et CI/CD.

---

## 3.5 Human-friendly et machine-friendly

L'architecture doit séparer :

```text
Filesystem
    ↓
Filter-aware Scanner
    ↓
Tree Model
    ↓
Deterministic Sort
    ↓
Application Service
    ↓
Renderer / future TUI
```

Le scanner consulte les policies de filtrage pendant son parcours, sans que le package `tree` sache comment `.gitignore` fonctionne. Le moteur ne doit jamais être couplé au format texte affiché.

Cette séparation permettra plus tard de supporter Unicode, ASCII, Markdown, JSON, YAML et des sorties orientées IA sans réécrire le scanner.

---

# 4. Positionnement du MVP

La `v0.1.0` doit fournir un **excellent générateur d'arborescence**, et rien de plus.

Ne pas implémenter prématurément :

- interface graphique ;
- serveur HTTP ;
- MCP server ;
- plugins ;
- synchronisation cloud ;
- analyse sémantique du code ;
- génération par IA ;
- télémétrie ;
- base de données ;
- système de comptes.

L'architecture peut anticiper certaines extensions, mais aucune abstraction ne doit être créée sans besoin concret.

---

# 5. Expérience CLI cible

## 5.1 Commande minimale

```bash
dirloom
```

Équivalent :

```bash
dirloom .
```

---

## 5.2 Répertoire spécifique

```bash
dirloom ./src
```

Windows :

```powershell
dirloom .\src
```

---

## 5.3 Profondeur maximale

```bash
dirloom --depth 3
```

Alias court :

```bash
dirloom -d 3
```

`0` signifie uniquement la racine. La racine est toujours incluse et porte la profondeur `0` ; ses enfants directs sont à la profondeur `1`.

En v0.1, l'absence de `--depth` signifie une profondeur illimitée. Aucune valeur sentinelle négative n'est exposée dans la CLI. Une valeur négative ou non entière est invalide, produit une erreur sur stderr et termine avec le code de sortie `2`.

---

## 5.4 Afficher uniquement les répertoires

```bash
dirloom --dirs-only
```

---

## 5.5 Afficher les fichiers cachés

```bash
dirloom --hidden
```

Par défaut, une entrée est considérée comme cachée si son nom commence par `.` sur toutes les plateformes ou, sous Windows, si l'attribut filesystem `FILE_ATTRIBUTE_HIDDEN` est présent. Les deux critères sont cumulatifs sous Windows.

`--hidden` désactive uniquement ce filtre de visibilité. Il ne neutralise ni `.gitignore`, ni les exclusions par défaut, ni `--ignore`. Une entrée cachée reste donc absente si une autre règle indépendante l'exclut. La racine explicitement fournie est toujours affichée, même si elle est cachée ; la règle s'applique à ses descendants.

---

## 5.6 Ignorer des éléments

```bash
dirloom --ignore node_modules
```

Chaque règle doit être fournie dans une occurrence distincte :

```bash
dirloom --ignore node_modules --ignore dist --ignore "*.log"
```

Les règles doivent supporter au minimum :

- un nom littéral, appliqué à toute entrée portant exactement ce nom ;
- un chemin relatif à la racine, avec des séparateurs normalisés en `/` ;
- les jokers `*` et `?` à l'intérieur d'un segment ;
- le joker `**` pour traverser zéro ou plusieurs segments ;
- le pruning d'un répertoire dès qu'il correspond, afin de ne pas parcourir inutilement ses descendants.

Les occurrences répétées de `--ignore` sont combinées par union. Les virgules n'ont aucune sémantique spéciale et font partie littéralement du motif. Les motifs absolus, les segments `..` qui sortent de la racine et les motifs invalides sont rejetés comme arguments CLI invalides. La correspondance respecte la casse afin de conserver la même sémantique sur tous les OS ; aucune règle de réinclusion n'est prise en charge par `--ignore` en v0.1.

---

## 5.7 Désactiver les exclusions par défaut

```bash
dirloom --no-default-ignore
```

---

## 5.8 Prise en compte de `.gitignore`

Par défaut, Dirloom applique les fichiers `.gitignore` depuis la racine analysée et dans chaque sous-répertoire parcouru.

Les règles suivent la sémantique Git pertinente : ordre des règles, ancrage, séparateurs, jokers, répertoires et négations avec `!`.

Prévoir :

```bash
dirloom --no-gitignore
```

Les fichiers `.gitignore` sont des fichiers de contrôle et doivent être lus indépendamment du filtre d'affichage des fichiers cachés ; ils ne sont affichés dans l'arbre que si les règles de visibilité le permettent.

Une règle d'un `.gitignore` imbriqué ne s'applique qu'à son répertoire et à ses descendants. Dirloom ne recherche pas les `.gitignore` au-dessus de la racine explicitement analysée et n'applique pas en v0.1 `.git/info/exclude` ni le fichier global `core.excludesFile`. `--no-gitignore` désactive toute cette couche, sans désactiver les exclusions par défaut ni `--ignore`. Le moteur doit utiliser une implémentation éprouvée ou être couvert par des tests de compatibilité dédiés ; une imitation partielle non documentée de Git est interdite.

---

# 6. Formats de sortie

## 6.1 Unicode

Format par défaut lorsque le terminal le permet :

```text
src/
├── components/
├── hooks/
│   └── useFeature.ts
└── index.ts
```

---

## 6.2 ASCII

```bash
dirloom --style ascii
```

Exemple :

```text
src/
|-- components/
|-- hooks/
|   `-- useFeature.ts
`-- index.ts
```

Ce mode répond notamment au besoin initial ayant motivé la création du projet.

---

## 6.3 Markdown

```bash
dirloom --format markdown
```

Sortie :

```markdown
```text
src/
├── components/
├── hooks/
│   └── useFeature.ts
└── index.ts
```
```

Le Markdown doit être directement copiable dans : 

- GitHub ; 
- GitLab ; 
- documentation ; 
- README ; 
- prompt LLM. 

---

## 6.4 JSON

Le format JSON fait partie du périmètre obligatoire de la v0.1 et doit être implémenté dès cette version. 

Il est exposé par : 

```bash
dirloom --format json
```

Contrat minimal v0.1 :

```json
{
  "schemaVersion": 1,
  "root": {
    "name": "src",
    "type": "directory",
    "children": [
      {
        "name": "components",
        "type": "directory",
        "children": []
      },
      {
        "name": "index.ts",
        "type": "file"
      }
    ]
  }
}
```

Le JSON est encodé en UTF-8, indenté de façon stable et terminé par un unique saut de ligne. Il expose un objet racine contenant `schemaVersion` fixé à `1` et le nœud `root`. Chaque nœud contient au minimum `name` et `type` ; `children` est présent uniquement pour un répertoire et vaut toujours un tableau, y compris lorsqu'il est vide. Les chemins absolus, timestamps, permissions et autres métadonnées non déterministes ou sensibles sont exclus du contrat v0.1. L'ordre de `children` est identique à celui des rendus textuels. Toute évolution incompatible exige une nouvelle valeur de `schemaVersion` et des tests de contrat.

## 6.5 Relation entre `--format` et `--style`

`--format` définit le contrat de sortie. Valeurs v0.1 : `text` par défaut, `markdown` et `json`.

`--style` définit uniquement le dessin de l'arbre pour les formats textuels. Valeurs v0.1 : `unicode` par défaut et `ascii`.

- `--format text --style unicode` produit l'arbre Unicode standard ;
- `--format text --style ascii` produit l'arbre ASCII ;
- `--format markdown` encapsule le rendu textuel sélectionné dans un bloc `text` ; `--style ascii` y est donc valide ;
- `--format json` utilise le renderer JSON et n'accepte pas `--style`.

La combinaison explicite `--format json --style ...` est rejetée avec un message actionnable et le code de sortie `2`, plutôt que d'ignorer silencieusement une option.

---

# 7. Redirection et export

La sortie standard doit toujours être utilisable avec les mécanismes natifs du shell :

```powershell
dirloom > structure.txt
```

```powershell
dirloom --format markdown > structure.md
```

Prévoir également :

```bash
dirloom --output structure.md
```

Alias :

```bash
dirloom -o structure.md
```

Règle :

> Le moteur de rendu écrit dans un `io.Writer`; il ne doit pas dépendre directement de stdout ou d'un fichier.

Cela permettra de tester les renderers facilement.

La destination est résolue en chemin absolu et comparée aux entrées scannées. Si le fichier demandé par `--output` se trouve dans la racine analysée, il est exclu implicitement du scan, qu'il existe déjà ou non, afin que la sortie ne puisse jamais s'auto-inclure.

L'écriture fichier doit être transactionnelle : rendre vers un fichier temporaire créé dans le même répertoire, vérifier toutes les erreurs d'écriture et de fermeture, puis remplacer atomiquement la destination lorsque la plateforme le permet. En cas d'échec, conserver l'ancien fichier intact et supprimer le temporaire. **Aucun fallback non sûr n'est autorisé** : si la primitive de remplacement sûre échoue, Dirloom retourne une erreur et conserve la destination. Pas de séquence `delete old → rename temp`, car un crash entre les deux ferait disparaître l'ancien résultat. Créer les répertoires parents implicitement est interdit ; un parent absent est une erreur actionnable. Dirloom ne doit pas suivre un symlink utilisé comme destination de sortie.

Avec `--output`, stdout reste vide en cas de succès. Les erreurs vont sur stderr. Le fichier produit doit contenir exactement les mêmes octets que ceux qui auraient été écrits sur stdout avec les mêmes options.

---

# 8. Exclusions par défaut

Fournir une liste initiale raisonnable de dossiers générés ou généralement inutiles lors du partage d'une architecture.

Exemples :

`.git`, `node_modules`, `.next`, `.nuxt`, `dist`, `build`, `coverage`, `.cache`, `.turbo`

Attention :

- cette liste doit rester courte ;
- aucun dossier métier potentiellement important ne doit être masqué arbitrairement ;
- elle doit être centralisée ;
- elle doit être documentée ;
- elle doit pouvoir être désactivée.

Ne pas embarquer des dizaines de règles spécifiques à des frameworks dans le MVP.

---

# 9. Ordonnancement

Par défaut :

1. répertoires ;
2. fichiers.

Dans chaque groupe :

tri alphabétique case-insensitive

Le comparateur est indépendant de la locale et identique sur toutes les plateformes. Il compare d'abord les noms par points de code Unicode après conversion en casse avec une règle Unicode déterministe, sans collation linguistique ni normalisation implicite. En cas d'égalité, il compare les noms originaux octet par octet en UTF-8 ; si nécessaire, le chemin relatif normalisé en `/` constitue le dernier départage. Le tri ne doit jamais dépendre de l'ordre retourné par le filesystem.

Exemple :

```text
features/
├── auth/
├── billing/
├── users/
├── index.ts
└── types.ts
```

Le tri doit être implémenté indépendamment du renderer et couvert par des tests incluant des noms qui ne diffèrent que par la casse, des caractères Unicode et des ordres d'énumération filesystem différents.

---

# 10. Symbolic links, junctions et cycles

Le scanner doit être défensif.

Par défaut, les symlinks POSIX, les liens symboliques Windows et les junctions Windows ne sont jamais suivis récursivement. Ils apparaissent comme des nœuds terminaux dans l’arbre.

Objectif :

> Éliminer tout risque de boucle infinie.

Le scanner doit identifier les liens à partir des métadonnées de l'entrée elle-même, avant toute résolution de cible. Sous Windows, les junctions et autres reparse points de redirection de noms sont classés dans le même type logique que les symlinks ; un reparse point qui ne redirige pas le namespace ne doit pas être reclassé automatiquement comme lien. Cette unification est volontaire : en v0.1, ces entrées utilisent toutes le type interne et JSON `symlink`, restent sans `children` et ne sont jamais parcourues. Dans les formats texte, un lien est rendu sous la forme stable `name -> target` lorsque sa cible enregistrée peut être obtenue sans parcours, sinon sous la forme `name [symlink]`. La cible est affichée telle qu'enregistrée, avec séparateurs normalisés en `/`, et ne doit jamais être transformée en chemin absolu. En JSON, le nœud porte `type: "symlink"` et un champ optionnel `target`. Aucun champ `children` n'est autorisé pour ce type.

La racine explicitement fournie peut être résolue une fois si elle pointe vers un répertoire ; aucun symlink rencontré ensuite n'est suivi. Par exemple, `dirloom .\my-project-link` analyse le contenu de la cible si `my-project-link` est un lien symbolique vers un répertoire.

Une future option pourra autoriser :

```bash
dirloom --follow-symlinks
```

mais elle n'est pas obligatoire pour `v0.1.0`.

---

# 11. Gros dépôts

Dirloom doit rester utilisable sur des monorepos.

Le scanner ne doit pas :

- charger inutilement le contenu des fichiers ;
- calculer des hashes ;
- ouvrir les fichiers métiers ;
- effectuer d'analyse AST ;
- utiliser un goroutine par fichier ;
- construire des structures excessivement coûteuses.

Seules les métadonnées nécessaires à la représentation de l'arborescence doivent être lues.

Prévoir une protection future :

```bash
--max-entries
```

mais ne l'implémenter dans le MVP que si nécessaire.

---

# 12. Architecture interne

Utiliser une architecture simple, modulaire et testable.

Structure cible indicative :

```text
dirloom/
├── cmd/
│   └── dirloom/
│       └── main.go
├── internal/
│   ├── cli/
│   │   ├── root.go
│   │   └── options.go
│   ├── app/
│   │   ├── inspect.go
│   │   └── inspect_test.go
│   ├── tree/
│   │   ├── node.go
│   │   ├── scanner.go
│   │   └── scanner_test.go
│   ├── filter/
│   │   ├── filter.go
│   │   ├── defaults.go
│   │   ├── gitignore.go
│   │   └── filter_test.go
│   ├── render/
│   │   ├── renderer.go
│   │   ├── unicode.go
│   │   ├── ascii.go
│   │   ├── markdown.go
│   │   ├── json.go
│   │   └── render_test.go
│   └── config/
│       └── config.go
├── testdata/
├── docs/
├── .github/
│   └── workflows/
├── .gitignore
├── .goreleaser.yaml
├── go.mod
├── go.sum
├── LICENSE
├── README.md
├── CONTRIBUTING.md
└── SECURITY.md
```

Cette structure peut être adaptée si une organisation plus idiomatique apparaît pendant l'implémentation.



Le package `internal/app` expose le service applicatif partagé par le CLI et, plus tard, `dirloom browse`. Exemple conceptuel :

```go
Inspect(ctx context.Context, request InspectRequest) (*tree.Node, error)
```

Ne pas créer des packages vides uniquement pour reproduire cette arborescence.

## 12.1 Décision de langage et extension interactive future

Go reste le langage retenu pour Dirloom. Ce choix privilégie un binaire natif sans runtime à installer, une distribution cross-platform simple, une bibliothèque standard adaptée au filesystem et un coût de maintenance proportionné au produit. Rust, Zig, C#/.NET, TypeScript/Bun et Python ne sont pas retenus pour la v0.1 ; aucune réécriture n’est planifiée.

Dirloom reste un produit **CLI first**, mais pourra recevoir une seconde interface interactive explicite sous la commande `dirloom browse`. L'exécution de `dirloom` sans sous-commande ne doit jamais lancer un TUI, même lorsqu'un terminal interactif est détecté : son contrat demeure une génération immédiate, déterministe et redirigeable.

Le TUI futur devra consommer les mêmes services applicatifs que le CLI non interactif. Il lui est interdit de réimplémenter indépendamment le parcours filesystem, le filtrage, le tri, la construction de l'arbre ou l'export. Le cœur ne doit connaître ni Bubble Tea ni un autre framework d'interface. Bubble Tea et Bubbles constituent le choix envisagé pour `dirloom browse`, mais aucune dépendance TUI, package vide ou abstraction spéculative ne doit être ajouté en v0.1.

```text
Filesystem → Filter-aware Scanner → Tree Model → Application Services
                                              ├── CLI renderers → stdout / files
                                              └── TUI browse    → preview / copy / export
```

---

# 13. Modèle interne

Le filesystem doit être converti vers un modèle indépendant du renderer.

Exemple conceptuel :

```go
type NodeType string

const (
    NodeDirectory NodeType = "directory"
    NodeFile      NodeType = "file"
    NodeSymlink   NodeType = "symlink"
)

type Node struct {
    Name     string
    Path     string
    Type     NodeType
    Children []*Node
}
```

Le modèle final peut différer si cela améliore réellement l'implémentation.

Éviter cependant d'inclure dans `Node` :

- logique d'affichage ;
- stdout ;
- configuration CLI ;
- parsing `.gitignore`.

---

# 14. Pipeline interne

Le flux principal doit conceptuellement être :

```text
CLI arguments
      ↓
configuration resolution
      ↓
filter-aware scanner
      ↓
tree model
      ↓
deterministic sort
      ↓
application service
      ↓
renderer
      ↓
stdout / file
```

Les étapes doivent être suffisamment découplées pour être testées séparément.

---

# 15. Interface des renderers

Définir une abstraction légère.

Exemple conceptuel :

```go
type Renderer interface {
    Render(w io.Writer, tree *tree.Node, options Options) error
}
```

Implémentations :

`UnicodeRenderer`, `ASCIIRenderer`, `MarkdownRenderer`, `JSONRenderer`

Éviter tout framework ou système de plugins pour le MVP.

---

# 16. Configuration

Ne pas rendre un fichier de configuration obligatoire.

Priorité de résolution souhaitée :

CLI arguments > project configuration > user configuration > defaults

Pour `v0.1`, seuls :

CLI arguments + defaults internes

sont obligatoires.

L'architecture ne doit toutefois pas empêcher l'arrivée future d'un :

`.dirloom.yaml`

ou :

`.dirloom.toml`

Ne pas implémenter plusieurs formats de configuration.

---

# 17. Gestion des erreurs

Les erreurs doivent être :

- compréhensibles ;
- courtes ;
- actionnables ;
- envoyées sur stderr ;
- accompagnées d'un code de sortie non nul.

Exemples :

`Error: directory "./foo" does not exist.`

`Error: permission denied while reading "./secrets".`

Éviter :

- stack trace par défaut ;
- panics pour des erreurs utilisateur ;

messages issus directement d'implémentations internes lorsque ceux-ci sont incompréhensibles.

## 17.1 Erreurs de parcours et résultats partiels

En v0.1, toute erreur empêchant de lire une entrée ou d'énumérer un répertoire rend l'exécution entière invalide. Dirloom doit interrompre le parcours, ne produire aucun résultat sur stdout et retourner le code `1`. Il est interdit d'omettre silencieusement une branche ou de présenter un arbre partiel comme complet.

Le message sur stderr doit identifier le chemin relatif concerné et la catégorie d'erreur, sans exposer de stack trace. Les erreurs apparues après un début de rendu ne doivent pas laisser une sortie présentée comme valide : le pipeline doit terminer scan, filtrage et tri avant d'écrire la sortie finale. Pour `--output`, l'écriture transactionnelle garantit que la destination précédente reste intacte.

Une future option explicite de type `--continue-on-error` pourra autoriser des résultats partiels accompagnés d'avertissements et d'un statut machine-readable, mais elle est hors périmètre v0.1. 

---

# 18. Exit codes

Au minimum : 

```text
0 = succès
1 = erreur générale
2 = arguments CLI invalides
```

Ne pas multiplier les codes d'erreur dans le MVP sans besoin démontré.

---

# 19. `--help`

La documentation intégrée doit être considérée comme une fonctionnalité produit.

```bash
dirloom --help
```

doit expliquer clairement :

Usage, Arguments, Options, Examples

Exemples minimum :

```bash
dirloom
dirloom ./src
dirloom --depth 3
dirloom --dirs-only
dirloom --style ascii
dirloom --format markdown
dirloom --ignore node_modules --ignore dist
dirloom --output structure.md
```

---

# 20. `--version`

Implémenter :

```bash
dirloom --version
```

La version ne doit pas être codée en dur dans plusieurs endroits.

Prévoir l'injection des métadonnées au build :

version, commit, build date

Seule la version utilisateur doit obligatoirement apparaître avec `--version`.

---

# 21. Qualité du code

Le projet doit être écrit comme un véritable produit maintenable.

Exigences :

`go fmt`, `go vet`, `go test ./...`

doivent réussir.

Configurer également un linter Go adapté dans la CI.

Interdire :

- code mort ;
- erreurs ignorées ;
- fonctions gigantesques ;
- duplication évidente ;
- dépendances inutiles ;
- abstractions prématurées ;
- variables globales mutables sans justification ;
- `panic()` pour traiter une entrée utilisateur invalide.

---

# 22. Tests

Les tests sont obligatoires dès la première version.

## 22.1 Unit tests

Tester au minimum :

### Scanner

- dossier vide ;
- fichiers simples ;
- sous-répertoires ;
- profondeur ;
- permissions ;
- fichiers cachés ;
- symlinks ;
- ordre déterministe.

### Filters

- exclusions par défaut ;
- `--ignore` ;
- glob patterns ;
- `.gitignore` ;
- désactivation des exclusions.

### Renderers

- Unicode ;
- ASCII ;
- Markdown ;
- JSON schema v1 ;
- symlinks ;
- répertoire vide ;
- UTF-8 ;
- saut de ligne final.

Utiliser des golden tests lorsque cela apporte de la valeur pour les sorties textuelles. Le JSON étant un contrat public, prévoir des contract tests dédiés validant `schemaVersion`, la structure des nœuds et l'absence de champs non déterministes.

---

## 22.2 Integration tests

Créer des fixtures sous :

`testdata/`

Exemple :

```text
testdata/
└── basic-project/
    ├── src/
    │   ├── components/
    │   └── index.ts
    ├── node_modules/
    │   └── ignored.js
    └── README.md
```

Vérifier l'exécution complète :

filesystem → filter-aware scanner → tree model → sort → renderer → output

---

## 22.3 Tests CLI

Tester au minimum :

```bash
dirloom --help
dirloom --version
dirloom invalid-directory
dirloom --depth invalid
dirloom --depth -1
dirloom --format json --style ascii
```

---

# 23. Tests Windows

Windows est une plateforme de première classe, pas une plateforme secondaire.

La CI doit obligatoirement tester au minimum :

Windows, Linux

macOS doit être ajouté également si la CI publique utilisée le permet raisonnablement.

Tester particulièrement :

- séparateurs `\` et `/` ;
- chemins contenant des espaces ;
- Unicode ;
- hidden files ;
- symlinks/junctions lorsque possible ;
- redirections PowerShell.

---

# 24. CI

Créer :

`.github/workflows/ci.yml`

La CI doit exécuter sur chaque Pull Request et push vers la branche principale :

format check, vet, lint, tests, build

Matrice :

ubuntu, windows, macos

Le pipeline doit échouer dès qu'un contrôle obligatoire échoue.

---

# 25. Release engineering

Préparer le projet pour produire des binaires autonomes :

`dirloom_Windows_x86_64.zip`, `dirloom_Windows_arm64.zip`, `dirloom_Linux_x86_64.tar.gz`, `dirloom_Linux_arm64.tar.gz`, `dirloom_Darwin_x86_64.tar.gz`, `dirloom_Darwin_arm64.tar.gz`

Utiliser GoReleaser.

Une release Git taggée :

`v0.1.0`

doit pouvoir produire automatiquement les artefacts associés.

La publication automatique peut être configurée après stabilisation du premier build.

---

# 26. Sécurité

Dirloom est un outil de lecture du filesystem.

Principe fondamental :

> Une exécution normale ne doit modifier aucun fichier du projet analysé.

Exceptions explicitement demandées :

`--output`

Dirloom ne doit jamais :

- exécuter les fichiers inspectés ;
- interpréter leur contenu ;
- charger automatiquement des scripts du projet ;
- suivre arbitrairement des liens récursifs ;
- envoyer le contenu du filesystem sur Internet ;
- embarquer de télémétrie dans le MVP.

---

# 27. Confidentialité

Dirloom doit fonctionner intégralement localement.

Aucune donnée ne quitte la machine.

Cette propriété doit rester vraie par défaut même si des fonctionnalités réseau apparaissent un jour.

---

# 28. Performance

Créer au moins un benchmark reproductible sur une fixture synthétique importante.

Objectif du benchmark :

- détecter les régressions ;
- mesurer le scanner ;
- mesurer éventuellement le renderer séparément.

Ne pas fixer arbitrairement un objectif en millisecondes avant d'avoir une baseline réelle.

Mesurer avant d'optimiser.

---

# 29. README initial

Le README doit contenir :

Nom + tagline, pourquoi Dirloom existe, screenshot/exemple, installation, Quick Start, commandes, options, exemples, formats, ignore rules, configuration, contributing, roadmap, license

Le premier écran du README doit permettre de comprendre le produit en quelques secondes.

Exemple :

```markdown
# Dirloom

Clean project trees for humans and AI.

Dirloom turns a directory into a clean, deterministic and shareable
project structure directly from your terminal.
```

---

# 30. Installation Windows — cible future

L'objectif est de pouvoir arriver à une expérience comme :

```powershell
winget install Dirloom.Dirloom
```

ou :

```powershell
scoop install dirloom
```

Le support des package managers n'est pas requis pour `v0.1.0`.

Pour le développement local :

```powershell
go install ...
```

ou utilisation directe du binaire compilé.

---

# 31. UX PowerShell

Dirloom doit fonctionner naturellement avec :

```powershell
dirloom
```

```powershell
dirloom .
```

```powershell
dirloom .\src
```

```powershell
dirloom -d 3
```

```powershell
dirloom --format markdown
```

```powershell
dirloom | Set-Clipboard
```

```powershell
dirloom --format markdown > structure.md
```

L'utilisation de `Set-Clipboard` signifie qu'un `--copy` natif n'est pas indispensable dans le MVP.

Un futur :

```bash
dirloom --copy
```

pourra néanmoins fournir une UX uniforme entre plateformes.

---

# 32. Comportement attendu par défaut

La commande :

```bash
dirloom
```

doit :

1. prendre le répertoire courant comme racine ;
2. utiliser le nom du répertoire comme première ligne ;
3. afficher dossiers puis fichiers ;
4. appliquer un tri stable ;
5. respecter `.gitignore` ;
6. appliquer les exclusions par défaut ;
7. ne pas suivre récursivement les symlinks ;
8. produire une arborescence Unicode ;
9. écrire uniquement la structure sur stdout ;
10. envoyer les erreurs sur stderr.

Aucun banner, logo ASCII ou message parasite sur stdout.

Ceci est important pour permettre :

```bash
dirloom > tree.txt
```

---

# 33. Roadmap produit

## v0.1 — Foundation

Objectif :

> Excellent générateur d'arborescence local.

Fonctionnalités :

scan, depth, ignore, `.gitignore`, dirs-only, hidden, Unicode, ASCII, Markdown, JSON schema v1, stdout, output file atomique, help, version, tests, CI, cross-platform builds

---

## v0.2 — Configuration & ergonomie

Envisager :

`.dirloom.yaml`, configuration utilisateur, presets, `--copy`, custom sorting, custom default excludes, completion bash/zsh/fish/PowerShell

---

## v0.3 — Interactive Explorer

Introduire explicitement la seconde interface du produit :

```bash
dirloom browse
```

Objectifs :

- navigation dans l'arbre ;
- expand/collapse ;
- recherche ;
- modification interactive de la profondeur ;
- activation/désactivation des filtres ;
- sélection/exclusion temporaire de branches ;
- preview Unicode/ASCII/Markdown/JSON ;
- copy ;
- export.

Contraintes structurantes : `dirloom` reste strictement non interactif ; `dirloom browse` utilise le même scanner, les mêmes filtres, le même modèle, le même tri et les mêmes exports que le CLI. Bubble Tea et Bubbles sont les frameworks Go envisagés pour cette interface.

```text
CLI non interactif = contrat stable et scriptable
TUI explicite         = composition et exploration interactives
```

---

## v0.4 — AI & Automation

Introduire éventuellement :

```bash
dirloom --preset ai --budget 12000
```

Objectifs :

réduction du bruit, budgets de sortie, représentations compactes, statistiques, sorties orientées machines et agents

Le preset AI doit rester une composition déterministe d’options et de règles documentées, sans appel réseau ni génération par modèle.

---

## v0.5 — Snapshots & Diff

Permettre :

```bash
dirloom snapshot
dirloom diff snapshot-a snapshot-b
```

Cas d’usage : documentation d’évolution d’architecture, review de refactoring, migrations, agents IA et CI.

Les snapshots doivent reposer sur un contrat versionné et ne contenir aucune métadonnée sensible ou non déterministe par défaut.

---

## v0.6 — Annotations

Permettre potentiellement d’enrichir une structure avec des commentaires ou métadonnées de projet, sans modifier les fichiers inspectés. Cette fonctionnalité pourra devenir une différenciation importante de Dirloom, mais son modèle de stockage devra être défini avant implémentation.

```text
features/
├── occupancy/      # équipe indépendante
├── revenues/       # domaine financier
└── new-feature/    # en construction
```

---

## v1.0

Objectif :

> CLI stable, documenté, cross-platform et publiquement distribuable.

Exigences potentielles :

API CLI stabilisée, config stabilisée, formats stabilisés, semver, packages officiels, checksums, SBOM, release signing, documentation complète, contribution guide, security policy, changelog

---

# 34. Fonctionnalités explicitement reportées

Ne pas implémenter pendant `v0.1` :

TUI, GUI, daemon, API HTTP, MCP, LLM, cloud, accounts, telemetry, plugins, remote filesystem, Git history analysis, AST parsing, code dependency graph, file content summaries, watch mode

Tous ces éléments restent hors périmètre v0.1. Le TUI est néanmoins une orientation produit officielle pour la v0.3 : la v0.1 doit seulement préserver la séparation existante entre cœur, services applicatifs et interfaces, sans ajouter Bubble Tea, package TUI vide ou abstraction prématurée.

---

# 35. Critères d'acceptation v0.1

La version n'est considérée comme terminée que si les scénarios suivants fonctionnent réellement.

### Cas 1

```powershell
cd C:\Projects\my-app
dirloom
```

Produit une arborescence propre du projet.

### Cas 2

```powershell
dirloom .\src -d 3
```

La profondeur est correctement limitée.

### Cas 3

```powershell
dirloom --style ascii
```

Produit :

```text
project/
|-- src/
|   |-- components/
|   `-- index.ts
`-- README.md
```

### Cas 4

```powershell
dirloom --format markdown
```

Produit un bloc Markdown directement réutilisable.

### Cas 5

```powershell
dirloom --ignore node_modules --ignore "*.log"
```

Les entrées correspondantes sont absentes.

### Cas 6

Un projet avec `.gitignore` ne fait pas apparaître les éléments ignorés.

### Cas 7

```powershell
dirloom --no-gitignore
```

permet de les faire réapparaître sauf exclusion indépendante.

### Cas 8

```powershell
dirloom > structure.txt
```

ne contient aucun message parasite.

### Cas 9

```powershell
dirloom --output structure.md --format markdown
```

crée correctement le fichier demandé.

### Cas 10

```powershell
dirloom --help
dirloom --version
```

fonctionnent sans erreur.

### Cas 11

Les tests passent sous :

Windows, Linux, macOS

### Cas 12

Une release peut produire les binaires pour les architectures supportées.

### Cas 13 — profondeur et tri déterministe

Sans `--depth`, le parcours est illimité. Avec `--depth 0`, seule la racine apparaît. Les valeurs négatives ou invalides retournent le code `2`. Des entrées ne différant que par la casse ou contenant de l'Unicode conservent le même ordre sur Windows, Linux et macOS.

### Cas 14 — fichiers cachés

Par défaut, les noms commençant par `.` sont masqués sur tous les OS et l'attribut hidden Windows est également reconnu. `--hidden` les révèle uniquement si aucune autre règle ne les exclut.

### Cas 15 — règles d'exclusion

Les occurrences répétées de `--ignore` sont combinées sans découpage par virgule. Les `.gitignore` imbriqués, leur portée et les négations sont respectés depuis la racine analysée, sans lire de configuration Git située au-dessus ou globale.

### Cas 16 — formats

`--format json` produit un document conforme au schéma v1. Markdown accepte les styles Unicode et ASCII. JSON combiné explicitement avec `--style` échoue avec le code `2`.

### Cas 17 — erreurs et export sûr

Une erreur de lecture ne produit aucun arbre partiel. Un fichier `--output` situé dans la racine n'apparaît pas dans sa propre sortie ; l'écriture est transactionnelle, stdout reste vide et une destination existante demeure intacte en cas d'échec.

### Cas 18 — liens et junctions

Les symlinks, liens cassés, junctions Windows et autres reparse points Windows de redirection de noms sont affichés comme des liens terminaux sans parcours de cible, sans boucle et avec une représentation stable dans les formats texte et JSON. Les reparse points non redirectionnels ne sont pas reclassés automatiquement comme liens.

---

# 36. Definition of Done

Une fonctionnalité n'est terminée que si :

- son comportement est implémenté ;
- ses erreurs sont gérées ;
- ses tests existent ;
- les tests passent ;
- la documentation utilisateur est mise à jour ;
- le formatage et le lint passent ;
- aucune régression n'est introduite ;
- le comportement Windows a été considéré ;
- aucun TODO critique n'est caché dans le code.

---

# 37. Méthode de travail attendue de l'agent

Commencer par inspecter entièrement le repository s'il existe déjà.

Ensuite, suivre cette séquence d'implémentation :

1. bootstrap du repository et dépendances minimales ;
2. modèle interne (`tree.Node`, types, tri déterministe) ;
3. policies de filtrage (defaults, `--ignore`, `.gitignore`) ;
4. scanner filter-aware (profondeur, hidden, symlinks, pruning) ;
5. service applicatif (`internal/app.Inspect`) ;
6. renderers (Unicode, ASCII, Markdown, JSON schema v1) ;
7. CLI (`cobra` ou équivalent, options, exit codes, help/version) ;
8. tests unitaires, golden tests, contract tests JSON et tests d'intégration ;
9. CI multi-plateforme ;
10. release engineering (GoReleaser) ;
11. documentation utilisateur (README, CONTRIBUTING, SECURITY) ;
12. validation finale contre les critères d'acceptation v0.1.

Ne pas considérer la tâche comme terminée simplement parce que le code compile.

---

# 38. Règles pour l'agent

L'agent est autorisé à ajuster les détails d'implémentation lorsque cela améliore la qualité du projet.

Il ne doit cependant pas modifier sans justification les invariants suivants :

Nom du produit : Dirloom ; binaire : `dirloom` ; langage : Go ; cross-platform ; Windows first-class ; CLI first ; commande par défaut non interactive et redirigeable ; TUI futur uniquement via `dirloom browse` ; core headless et réutilisable par toutes les interfaces ; local-first ; deterministic output ; scanner séparé du renderer ; pas de modification du projet inspecté ; pas de télémétrie ; pas de fonctionnalités réseau en v0.1 ; tests obligatoires ; CI obligatoire.

En cas de choix entre :

architecture sophistiquée

et :

architecture simple mais extensible

privilégier systématiquement la seconde.

---

# 39. Livrables attendus

À la fin de l'implémentation, le repository doit contenir au minimum :

source code, unit tests, integration tests, README.md, LICENSE, CONTRIBUTING.md, SECURITY.md, .gitignore, CI GitHub Actions, GoReleaser configuration, release-ready build

L'agent doit également fournir un rapport final indiquant :

ce qui a été implémenté, architecture retenue, dépendances ajoutées, tests exécutés, résultats des tests, commandes de build, commandes d'installation locale, limitations connues, éléments volontairement reportés

---

# 40. Résultat attendu

À l'issue de cette première phase, l'expérience minimale doit être :

```powershell
PS C:\Projects\sonora> dirloom
```

```text
sonora/
├── apps/
│   ├── api/
│   └── web/
├── packages/
│   ├── config/
│   ├── database/
│   ├── shared/
│   └── ui/
├── docs/
├── package.json
└── README.md
```

Puis :

```powershell
PS C:\Projects\sonora> dirloom --format markdown | Set-Clipboard
```

et la structure du projet est immédiatement prête à être collée dans une documentation, une Pull Request ou une conversation avec un agent de code.

**C'est la qualité de cette expérience fondamentale qui doit être optimisée avant d'élargir le périmètre de Dirloom.**
