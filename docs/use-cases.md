# Dirloom — Cas d’usage et exemples pratiques

> **Statut :** guide utilisateur évolutif<br>
> **Périmètre :** capacités natives actuellement implémentées<br>
> **Dernière vérification :** 15 août 2026<br>
> **Sources d’autorité :** CLI, tests, [README](../README.md), [configuration](configuration.md), [presets](presets.md), [Markdown sémantique](markdown-tree.md) et [spécification v0.1](../SPEC-v0.1.md)

Ce guide montre ce que Dirloom permet de faire aujourd’hui. Il privilégie les recettes exécutables, les combinaisons utiles et les résultats attendus. Les fonctionnalités uniquement prévues dans la [roadmap](product/roadmap.md) sont identifiées comme indisponibles afin de ne pas les confondre avec le produit actuel.

Dirloom inspecte des **noms et des relations structurelles**. Il ne lit pas le contenu des fichiers et ne modifie pas le projet, sauf lorsqu’une destination est explicitement fournie avec `--output`.

## 1. Comment lire les exemples

La syntaxe générale est :

```text
dirloom [directory] [flags]
```

- `directory` est facultatif ; le répertoire courant est utilisé par défaut.
- Les commandes commençant par `dirloom` utilisent uniquement la CLI native.
- Les commandes PowerShell, Git, `jq`, `gh`, `sed` ou les utilitaires de presse-papiers sont des compositions externes, signalées comme telles.
- Les motifs sont placés entre guillemets pour éviter leur expansion préalable par le shell.
- Les chemins Windows peuvent utiliser `\` ; les motifs `--ignore` utilisent `/` comme séparateur normalisé.
- Les sorties illustrées sont volontairement courtes, mais respectent le contrat réel de la CLI.

Pour connaître la version et les options installées :

```bash
dirloom --version
dirloom --help
```

## 2. Carte rapide des usages

| Besoin | Commande de départ |
| --- | --- |
| Voir la structure du projet courant | `dirloom` |
| Inspecter un autre répertoire | `dirloom ./src` |
| Limiter le niveau de détail | `dirloom --depth 3` |
| Voir uniquement les dossiers | `dirloom --dirs-only` |
| Inclure les fichiers cachés | `dirloom --hidden` |
| Exclure des fichiers ou dossiers | `dirloom --ignore PATTERN` |
| Ignorer plusieurs motifs | `dirloom --ignore dist --ignore "*.log"` |
| Désactiver les exclusions intégrées | `dirloom --no-default-ignore` |
| Désactiver la lecture des `.gitignore` | `dirloom --no-gitignore` |
| Produire un arbre ASCII | `dirloom --style ascii` |
| Produire du Markdown prêt à insérer | `dirloom --format markdown` |
| Produire une liste Markdown sémantique | `dirloom --format markdown-tree` |
| Produire un document machine versionné | `dirloom --format json` |
| Écrire sûrement dans un fichier | `dirloom --output structure.txt` |
| Générer une documentation d’architecture | `dirloom --format markdown --output structure.md` |
| Générer un artefact pour la CI | `dirloom --format json --output structure.json` |
| Auditer ce que masque `.gitignore` | `dirloom --no-gitignore` |
| Auditer absolument toutes les couches de filtrage | `dirloom --hidden --no-gitignore --no-default-ignore` |
| Expliquer la configuration effective | `dirloom config explain` |
| Ignorer les préférences personnelles en CI | `dirloom --no-user-config` |
| Désactiver toute configuration persistante | `dirloom --no-config` |
| Produire une vue documentaire | `dirloom --preset docs` |
| Produire une vue compacte | `dirloom --preset compact` |
| Voir la forme d’un monorepo | `dirloom --preset monorepo` |
| Préparer un contexte structurel pour une IA | `dirloom --preset ai` |
| Expliquer un preset intégré | `dirloom preset explain ai` |

## 3. Prendre en main l’inspection

### 3.1 Inspecter le répertoire courant

```bash
dirloom
```

Exemple de sortie :

```text
my-project/
├── src/
│   ├── components/
│   └── index.ts
├── package.json
└── README.md
```

Les dossiers apparaissent avant les fichiers. Chaque groupe est trié de manière déterministe, indépendamment de l’ordre fourni par le système de fichiers.

### 3.2 Inspecter un autre répertoire

```bash
dirloom ./src
dirloom ../shared
dirloom "C:\Projects\My App"
```

Un seul répertoire peut être fourni. Le chemin doit exister et désigner un dossier.

### 3.3 Afficher uniquement la racine

```bash
dirloom --depth 0
```

```text
my-project/
```

Ce cas est utile pour vérifier le nom canonique de la racine ou tester rapidement la résolution d’un chemin.

### 3.4 Limiter la profondeur

```bash
dirloom --depth 1
dirloom --depth 2
dirloom ./src --depth 3
```

- `--depth 1` affiche les enfants directs de la racine.
- `--depth 2` affiche aussi leurs enfants.
- Sans `--depth`, le parcours est illimité.

Une profondeur limitée est souvent préférable pour un README, une pull request ou un prompt : elle montre l’architecture sans exposer chaque fichier terminal.

### 3.5 Afficher uniquement les dossiers

```bash
dirloom --dirs-only
```

```text
my-project/
└── src/
    └── components/
```

Combinaison utile pour obtenir une carte architecturale compacte :

```bash
dirloom --dirs-only --depth 4
```

Les fichiers et les liens symboliques terminaux sont omis, y compris lorsqu’un lien pointe vers un dossier.

### 3.6 Inspecter explicitement une racine normalement exclue

La racine demandée est toujours conservée. Cette commande inspecte donc `node_modules` même si ce nom fait partie des exclusions intégrées :

```bash
dirloom node_modules --depth 2
```

L’exception concerne uniquement la racine explicitement sélectionnée ; les descendants continuent d’être filtrés normalement.

## 4. Maîtriser le filtrage

Dirloom applique les couches dans cet ordre. La première exclusion est définitive :

1. destination de `--output` ;
2. exclusions intégrées ;
3. règles `--ignore` ;
4. règles `.gitignore` ;
5. visibilité des entrées cachées.

Une option située plus tard dans cette liste ne peut pas réinclure un élément déjà exclu.

### 4.1 Connaître les exclusions intégrées

Les dossiers suivants sont exclus par défaut :

```text
.git  node_modules  .next  .nuxt  coverage  .cache  .turbo
```

`dist` et `build` ne sont pas exclus automatiquement.

### 4.2 Désactiver les exclusions intégrées

```bash
dirloom --no-default-ignore
```

Cette option permet notamment d’observer `node_modules` ou `.next`. Elle ne désactive ni `.gitignore`, ni `--ignore`, ni le filtre des fichiers cachés.

Pour afficher un dossier `.git`, les deux options suivantes sont nécessaires :

```bash
dirloom --no-default-ignore --hidden --depth 2
```

> Cette commande peut produire un arbre très volumineux. Commencez avec une profondeur faible.

### 4.3 Inclure les entrées cachées

```bash
dirloom --hidden
```

Sur tous les systèmes, les noms commençant par `.` sont considérés comme cachés. Sous Windows, l’attribut `hidden` est également pris en compte.

`--hidden` ne neutralise pas les autres filtres. Un fichier caché exclu par `.gitignore` ou `--ignore` reste absent.

### 4.4 Exclure un nom partout dans l’arbre

```bash
dirloom --ignore temp
dirloom --ignore vendor
dirloom --ignore generated
```

Un motif sans `/` cible le nom correspondant à n’importe quelle profondeur. S’il correspond à un dossier, tout le sous-arbre est élagué immédiatement.

Pour ne cibler que les dossiers portant ce nom :

```bash
dirloom --ignore "temp/"
```

### 4.5 Exclure par extension ou motif de nom

```bash
dirloom --ignore "*.log"
dirloom --ignore "*.map"
dirloom --ignore "*.generated.*"
dirloom --ignore "cache?"
```

- `*` correspond à zéro ou plusieurs caractères dans un seul segment.
- `?` correspond à un caractère dans un seul segment.
- La correspondance est sensible à la casse sur toutes les plateformes.

### 4.6 Exclure un chemin relatif précis

```bash
dirloom --ignore "docs/private/**"
dirloom --ignore "src/generated/**"
dirloom --ignore "packages/legacy"
```

Un motif contenant `/` est évalué depuis la racine inspectée. Il ne correspond pas automatiquement à un chemin de même suffixe situé ailleurs.

### 4.7 Traverser plusieurs niveaux avec `**`

```bash
dirloom --ignore "src/**/generated?.go"
dirloom --ignore "**/fixtures/**"
dirloom --ignore "packages/**/dist"
```

`**` traverse zéro ou plusieurs segments. Il permet de cibler une convention répétée dans un monorepo ou plusieurs modules.

### 4.8 Combiner plusieurs exclusions

```bash
dirloom \
  --ignore dist \
  --ignore build \
  --ignore "*.log" \
  --ignore "**/generated/**"
```

Sous PowerShell, utilisez le caractère de continuation `` ` `` ou écrivez la commande sur une seule ligne :

```powershell
dirloom `
  --ignore dist `
  --ignore build `
  --ignore '*.log' `
  --ignore '**/generated/**'
```

Chaque motif exige sa propre occurrence de `--ignore`. Les virgules sont littérales :

```bash
# Incorrect si l’intention est d’exclure trois noms
dirloom --ignore "dist,build,coverage"

# Correct
dirloom --ignore dist --ignore build --ignore coverage
```

### 4.9 Utiliser les `.gitignore`

Par défaut, Dirloom charge le `.gitignore` de la racine et ceux des sous-dossiers traversés :

```bash
dirloom
```

Sont pris en charge :

- portée des `.gitignore` imbriqués ;
- motifs ancrés et règles de dossiers ;
- jokers Git ;
- négations `!` ;
- sémantique « dernière règle gagnante » au sein de cette couche.

Dirloom ne lit pas `.git/info/exclude`, les `.gitignore` situés au-dessus de la racine ni le fichier global `core.excludesFile`.

Un `.gitignore` qui est lui-même un lien symbolique n’est pas suivi. Un motif `.gitignore` mal formé est traité comme une non-correspondance et ne fait pas échouer le scan.

### 4.10 Désactiver `.gitignore`

```bash
dirloom --no-gitignore
```

Cas d’usage :

- vérifier quels artefacts de build existent réellement ;
- examiner les fichiers ignorés avant une suppression ;
- préparer un diagnostic où la structure physique compte davantage que la vue Git ;
- comparer la structure visible par Git à la structure complète.

Pour inclure aussi les entrées cachées :

```bash
dirloom --no-gitignore --hidden
```

### 4.11 Auditer toutes les couches de visibilité

```bash
dirloom --hidden --no-gitignore --no-default-ignore --depth 3
```

Cette combinaison se rapproche le plus d’une vue physique complète, à l’exception des règles `--ignore` explicitement ajoutées et de la destination `--output`.

> L’arbre peut révéler des noms de fichiers sensibles et les cibles enregistrées des liens symboliques. Dirloom ne lit pas leur contenu, mais la sortie doit tout de même être manipulée comme une information potentiellement sensible.

### 4.12 Comprendre les limites de `--ignore`

Les règles CLI n’acceptent pas la réinclusion avec `!` :

```bash
# Non pris en charge comme règle de réinclusion CLI
dirloom --ignore "*.log" --ignore "!important.log"
```

Pour une logique de réinclusion, placez les règles dans un `.gitignore` adapté ou filtrez la sortie JSON avec un outil externe.

Sont également rejetés :

- motif vide ;
- chemin absolu ;
- chemin Windows avec lettre de lecteur ;
- segment `..` ;
- motif syntaxiquement invalide.

### 4.13 Réutiliser une configuration persistante

Un fichier projet `.dirloom.yaml` permet de partager les mêmes limites, formats et exclusions. L'[exemple minimal canonique](configuration.md#quick-start) peut être copié tel quel puis adapté au projet.

La priorité est `CLI explicite > projet > utilisateur > défauts intégrés`. Les exclusions sont fusionnées dans l’ordre utilisateur, projet puis CLI, avec les règles du preset gagnant insérées dans sa couche avant les exclusions explicites et suppression des doublons exacts.

Pour vérifier les valeurs et leur origine :

```bash
dirloom config explain
dirloom config explain --as json
```

Pour une CI indépendante des préférences personnelles :

```bash
dirloom . --no-user-config --config .dirloom.yaml --format json --output structure.json
```

Dans un monorepo Git, Dirloom charge le `.dirloom.yaml` le plus proche entre la racine inspectée et la frontière du worktree. Les fichiers projet parents ne sont pas fusionnés. Hors Git, seul le fichier placé dans la racine inspectée est découvert automatiquement.

La référence publique complète — schéma, chemins multiplateformes, sécurité, erreurs et recettes — se trouve dans [Configuration persistante](configuration.md).

## 5. Choisir le bon rendu

Tous les formats sont encodés en UTF-8 sans BOM, utilisent des fins de ligne LF sur chaque plateforme et se terminent par exactement un saut de ligne.

### 5.1 Texte Unicode — rendu par défaut

```bash
dirloom --format text --style unicode
```

Équivalent court :

```bash
dirloom
```

Le rendu Unicode est idéal pour un terminal moderne, une capture ou un document conservant l’UTF-8.

### 5.2 Texte ASCII — compatibilité maximale

```bash
dirloom --style ascii
```

```text
my-project/
|-- src/
|   `-- index.ts
`-- README.md
```

Utilisez-le pour des consoles limitées, certains logs, des systèmes anciens ou des canaux qui dégradent les caractères de dessin Unicode.

### 5.3 Markdown prêt à insérer

```bash
dirloom --format markdown
```

Dirloom encapsule l’arbre dans un bloc `text` complet :

````markdown
```text
my-project/
├── src/
└── README.md
```
````

Le style ASCII reste disponible :

```bash
dirloom --format markdown --style ascii
```

### 5.4 Markdown sémantique pour la documentation

```bash
dirloom --format markdown-tree
```

```markdown
- `my-project/`
  - `src/`
    - `components/`
    - `index.ts`
  - `package.json`
  - `README.md`
```

Contrairement au format `markdown`, qui encapsule le dessin textuel dans un bloc clôturé, `markdown-tree` produit une vraie liste imbriquée. Il est adapté aux README, descriptions de pull request, systèmes documentaires et lecteurs d’écran.

Ce format ne contient ni ANSI, ni icône terminal, ni HTML. Le style `unicode` ou `ascii` n’a aucun effet et une combinaison CLI explicite avec `--style` est rejetée. Le contrat complet est décrit dans le [guide Markdown sémantique](markdown-tree.md).

### 5.5 JSON pour les machines

```bash
dirloom --format json
```

```json
{
  "schemaVersion": 1,
  "root": {
    "name": "src",
    "type": "directory",
    "children": [
      {
        "name": "index.ts",
        "type": "file"
      }
    ]
  }
}
```

Le document JSON respecte les règles suivantes :

- `schemaVersion` vaut actuellement `1` ;
- les types sont `directory`, `file` et `symlink` ;
- un dossier possède toujours `children`, même lorsqu’il est vide ;
- un fichier ne possède jamais `children` ;
- un lien symbolique peut exposer `target` ;
- les chemins absolus, dates, permissions et métadonnées non déterministes sont absents.

JSON n’a pas de style de dessin. Cette combinaison est donc volontairement invalide :

```bash
dirloom --format json --style ascii
```

## 6. Enregistrer, partager et intégrer la sortie

### 6.1 Écrire transactionnellement dans un fichier

```bash
dirloom --output structure.txt
dirloom --format markdown --output docs/structure.md
dirloom --format markdown-tree --output docs/project-tree.md
dirloom --format json --output structure.json
```

Avec `--output` :

- stdout reste vide en cas de succès ;
- la destination est exclue de son propre scan ;
- le rendu est d’abord écrit dans un fichier temporaire voisin ;
- la destination est remplacée atomiquement lorsque la plateforme le permet ;
- une destination existante reste intacte si le remplacement sûr échoue ;
- les permissions d’un fichier existant sont conservées ;
- le dossier parent doit déjà exister ;
- une destination qui est un lien symbolique est refusée.

La destination peut être placée dans le projet sans s’auto-inclure :

```bash
dirloom . --format markdown --output docs/structure.md
```

Seule la destination active est exclue. D’anciens exports portant d’autres noms restent visibles, sauf s’ils sont filtrés explicitement.

### 6.2 Utiliser la redirection du shell

```bash
dirloom --style ascii > structure.txt
dirloom --format json > structure.json
```

La redirection dépend du shell et ne bénéficie pas de l’écriture transactionnelle de `--output`. Pour un artefact durable ou généré en CI, préférez `--output`.

### 6.3 Copier dans le presse-papiers

PowerShell :

```powershell
dirloom --format markdown | Set-Clipboard
```

macOS, avec `pbcopy` :

```bash
dirloom --format markdown | pbcopy
```

Linux Wayland, avec `wl-copy` installé :

```bash
dirloom --format markdown | wl-copy
```

Linux X11, avec `xclip` installé :

```bash
dirloom --format markdown | xclip -selection clipboard
```

Ces outils de presse-papiers sont externes. Dirloom v0.1 ne possède pas encore d’option native `--copy`.

### 6.4 Ajouter un arbre à un README

```bash
dirloom . --dirs-only --depth 4 --format markdown --output docs/project-structure.md
```

Le fichier produit peut être inclus ou recopié dans le README. Une profondeur bornée et `--dirs-only` limitent le bruit tout en gardant l’architecture principale.

### 6.5 Partager une structure dans une issue ou une pull request

```bash
dirloom . --depth 3 --format markdown --output structure.md
```

Avec GitHub CLI, outil externe :

```bash
gh pr comment --body-file structure.md
```

Relisez toujours la sortie avant de la publier : les noms de fichiers peuvent révéler des informations internes même si leur contenu n’est jamais lu.

## 7. Cas d’usage concrets

### 7.1 Présenter rapidement un projet à un nouveau contributeur

```bash
dirloom . --preset compact --format markdown --depth 4
```

Résultat recherché : montrer les grandes zones du dépôt, leurs niveaux et leur organisation sans submerger le lecteur avec tous les fichiers.

### 7.2 Documenter un sous-système

```bash
dirloom ./src/features/payments --preset docs
```

Le répertoire ciblé devient la racine de l’artefact. Cette approche permet de documenter un module indépendamment du reste du dépôt.

### 7.3 Préparer le contexte structurel d’une conversation avec une IA

```bash
dirloom . --preset ai
```

Sous PowerShell :

```powershell
dirloom . --preset ai | Set-Clipboard
```

Le preset retire les dossiers `dist` et `build` à toute profondeur ainsi que les fichiers `*.map`. Dirloom fournit uniquement la structure : ajoutez séparément les fichiers ou extraits de code réellement nécessaires à la tâche.

### 7.4 Obtenir la forme d’un monorepo

```bash
dirloom . --preset monorepo
```

Le preset affiche uniquement les dossiers jusqu’à la profondeur `4` et masque `**/dist` ainsi que `**/build`. Pour examiner davantage un workspace :

```bash
dirloom packages --preset monorepo --depth 6
```

Les définitions exactes, leur activation YAML et leur priorité sont documentées dans [Presets intégrés](presets.md).

### 7.5 Examiner l’organisation des tests

```bash
dirloom ./tests --depth 4
dirloom . --dirs-only --depth 5 --ignore fixtures
```

Le premier exemple se concentre sur une racine de tests dédiée. Le second montre l’ensemble des dossiers de tests répartis dans le projet, à l’exception des fixtures.

### 7.6 Vérifier la présence réelle d’artefacts ignorés

```bash
dirloom . --no-gitignore --depth 3
```

Ajoutez `--hidden` si les artefacts concernés commencent par `.` :

```bash
dirloom . --no-gitignore --hidden --depth 3
```

### 7.7 Examiner les dépendances installées sans tout développer

```bash
dirloom . --no-default-ignore --depth 2
```

Cette commande peut révéler `node_modules`, `.next`, `.nuxt`, `coverage`, `.cache` et `.turbo`, sauf si `.gitignore` ou le filtre des entrées cachées les exclut encore.

### 7.8 Préparer un artefact structurel pour la CI

```bash
dirloom . --format json --output structure.json
```

Exemple de contrôle shell :

```bash
dirloom . --format json --output structure.json
test -s structure.json
```

Exemple PowerShell :

```powershell
dirloom . --format json --output structure.json
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if (-not (Test-Path structure.json)) { throw 'structure.json is missing' }
```

### 7.9 Détecter une évolution structurelle avec Git

Dirloom v0.1 n’a pas encore de commande native `diff`, mais son artefact déterministe peut être versionné :

```bash
dirloom . --format json --output structure.json
git diff --exit-code -- structure.json
```

Workflow initial :

```bash
dirloom . --format json --output structure.json
git add structure.json
git commit -m "docs: record project structure"
```

Aux exécutions suivantes, Git montre les changements du JSON. La future capacité Structural Diff décrite dans la roadmap n’est pas encore utilisée ici.

### 7.10 Vérifier le déterminisme d’un environnement

Placez les deux sorties en dehors de la racine inspectée afin qu’aucune ne contamine l’autre :

```powershell
dirloom . --format json --output ..\structure-a.json
dirloom . --format json --output ..\structure-b.json

(Get-FileHash ..\structure-a.json -Algorithm SHA256).Hash
(Get-FileHash ..\structure-b.json -Algorithm SHA256).Hash
```

Sous un shell POSIX :

```bash
dirloom . --format json --output ../structure-a.json
dirloom . --format json --output ../structure-b.json
sha256sum ../structure-a.json ../structure-b.json
```

À état identique et options identiques, les empreintes doivent être identiques.

### 7.11 Produire une vue compatible avec des logs anciens

```bash
dirloom . --style ascii --depth 4 --output structure.log
```

Le style ASCII évite les problèmes d’affichage dans les terminaux, encodeurs ou systèmes de collecte qui ne préservent pas correctement Unicode.

### 7.12 Contrôler une structure avant et après une génération externe

```bash
dirloom . --format json --output ../before.json

# Exécuter ici le générateur, la migration ou l’installation externe.

dirloom . --format json --output ../after.json
```

Comparer ensuite avec un outil externe :

```bash
git diff --no-index -- ../before.json ../after.json
```

Dirloom observe la structure ; il n’exécute pas lui-même la génération ou la migration en v0.1.

## 8. Recettes par écosystème

Ces recettes illustrent des filtres courants. Adaptez-les aux conventions réelles du dépôt.

### 8.1 Node.js, Next.js ou Nuxt

`node_modules`, `.next` et `.nuxt` sont déjà exclus. Ajoutez les sorties propres au projet :

```bash
dirloom . --depth 4 --ignore dist --ignore build --ignore "*.map"
```

### 8.2 Go

```bash
dirloom . \
  --depth 4 \
  --ignore vendor \
  --ignore "*.out" \
  --ignore coverage.out
```

Pour une vue centrée sur les packages :

```bash
dirloom . --dirs-only --depth 4 --ignore vendor
```

### 8.3 Flutter ou Dart

```bash
dirloom . \
  --depth 4 \
  --ignore .dart_tool \
  --ignore build \
  --ignore "ios/Pods" \
  --ignore "android/.gradle"
```

### 8.4 Python

```bash
dirloom . \
  --depth 4 \
  --ignore .venv \
  --ignore venv \
  --ignore __pycache__ \
  --ignore .pytest_cache \
  --ignore "*.pyc"
```

### 8.5 Rust

```bash
dirloom . --depth 4 --ignore target
```

### 8.6 Documentation

```bash
dirloom ./docs \
  --depth 4 \
  --ignore "*.png" \
  --ignore "*.jpg" \
  --ignore "*.gif" \
  --format markdown
```

Cette vue conserve l’organisation documentaire tout en masquant les médias lourds.

### 8.7 Dépôt contenant beaucoup de code généré

```bash
dirloom . \
  --depth 5 \
  --ignore generated \
  --ignore "**/*.generated.*" \
  --ignore "**/gen/**"
```

### 8.8 Architecture centrée sur des modules ou des fonctionnalités

```bash
dirloom ./src/features --dirs-only --depth 4
dirloom ./packages --dirs-only --depth 3
dirloom ./services --dirs-only --depth 3
```

Ces commandes permettent de comparer visuellement la forme des modules. La comparaison automatique de formes reste une fonctionnalité future.

## 9. Exploiter le JSON

### 9.1 Lire le document avec PowerShell

```powershell
$data = (dirloom . --format json | Out-String) | ConvertFrom-Json

$data.schemaVersion
$data.root.name
$data.root.children | Select-Object name, type, target
```

### 9.2 Vérifier la version du schéma

```powershell
$data = (dirloom . --format json | Out-String) | ConvertFrom-Json

if ($data.schemaVersion -ne 1) {
  throw "Unsupported Dirloom schema: $($data.schemaVersion)"
}
```

Tout consommateur machine doit vérifier `schemaVersion` avant d’interpréter le reste du document.

### 9.3 Aplatir récursivement l’arbre avec PowerShell

```powershell
function Get-DirloomNode {
  param(
    [Parameter(Mandatory)] $Node,
    [string] $Parent = ''
  )

  $path = if ($Parent) { "$Parent/$($Node.name)" } else { $Node.name }

  [pscustomobject]@{
    Path   = $path
    Type   = $Node.type
    Target = $Node.target
  }

  if ($Node.type -eq 'directory') {
    foreach ($child in $Node.children) {
      Get-DirloomNode -Node $child -Parent $path
    }
  }
}

$data = (dirloom . --format json | Out-String) | ConvertFrom-Json
$nodes = Get-DirloomNode -Node $data.root
$nodes | Format-Table
```

À partir de `$nodes` :

```powershell
# Compter les types de nœuds
$nodes | Group-Object Type | Select-Object Name, Count

# Lister seulement les fichiers
$nodes | Where-Object Type -eq 'file' | Select-Object -ExpandProperty Path

# Lister les liens symboliques et leurs cibles enregistrées
$nodes | Where-Object Type -eq 'symlink' | Select-Object Path, Target
```

### 9.4 Interroger la sortie avec `jq`

`jq` est un outil externe.

```bash
# Lire la version du schéma
dirloom . --format json | jq '.schemaVersion'

# Voir les enfants directs de la racine
dirloom . --format json | jq '.root.children[] | {name, type}'

# Compter tous les fichiers
dirloom . --format json | jq '[.. | objects | select(.type? == "file")] | length'

# Lister les liens symboliques
dirloom . --format json | jq -r '.. | objects | select(.type? == "symlink") | [.name, .target] | @tsv'
```

### 9.5 Utiliser JSON lorsqu’une sélection positive est nécessaire

Dirloom v0.1 sait exclure, mais ne possède pas de filtre natif « inclure uniquement ». Pour ne conserver que certains types, extensions ou chemins, produisez JSON puis appliquez la sélection dans le consommateur.

Exemple PowerShell après aplatissement :

```powershell
$nodes |
  Where-Object { $_.Type -eq 'file' -and $_.Path -match '\.(go|ts|dart)$' } |
  Select-Object -ExpandProperty Path
```

## 10. Liens symboliques, jonctions et sécurité

### 10.1 Comportement des liens rencontrés

Les liens symboliques, liens symboliques Windows et jonctions rencontrés sous la racine sont affichés comme des nœuds terminaux. Dirloom ne les traverse pas.

Exemple textuel :

```text
shared -> ../shared
```

Exemple JSON :

```json
{
  "name": "shared",
  "type": "symlink",
  "target": "../shared"
}
```

Cette règle évite les cycles, les sorties de périmètre involontaires et les parcours non déterministes.

### 10.2 Racine explicitement symbolique

Une racine explicitement sélectionnée peut être résolue une fois si elle pointe vers un dossier. Les liens rencontrés ensuite restent terminaux.

### 10.3 Absence de résultat partiel

Le scan, le filtrage et le tri se terminent avant le début du rendu. Une erreur de lecture fait échouer toute l’opération ; Dirloom ne présente pas un arbre incomplet comme valide.

## 11. Diagnostiquer les erreurs

Dirloom utilise des codes de sortie stables :

| Code | Signification | Exemples |
| ---: | --- | --- |
| `0` | Succès | inspection, `--help`, `--version` |
| `1` | Erreur d’exécution | dossier absent, permission refusée, échec d’écriture |
| `2` | Arguments invalides | profondeur négative, format inconnu, combinaison interdite |

### 11.1 Dossier inexistant

```bash
dirloom ./does-not-exist
```

La commande échoue avec le code `1` et écrit l’erreur sur stderr.

### 11.2 Chemin qui n’est pas un dossier

```bash
dirloom README.md
```

Dirloom refuse d’inspecter un fichier comme racine.

### 11.3 Trop d’arguments positionnels

```bash
dirloom ./src ./tests
```

Inspectez les deux racines séparément ou choisissez une racine commune.

### 11.4 Profondeur invalide

```bash
dirloom --depth -1
dirloom --depth invalid
```

`--depth` accepte un entier positif ou nul, ainsi que `unlimited` pour retirer une limite héritée.

### 11.5 Format ou style invalide

```bash
dirloom --format yaml
dirloom --style auto
dirloom --format json --style ascii
```

Formats disponibles : `text`, `markdown`, `markdown-tree`, `json`. Styles disponibles pour les rendus texte et Markdown clôturé : `unicode`, `ascii`.

### 11.6 Destination impossible

```bash
dirloom --output missing-parent/structure.md
```

Dirloom ne crée pas le dossier parent. Créez-le explicitement avant de relancer la commande.

### 11.7 Lire le code de sortie

PowerShell :

```powershell
dirloom . --format json --output structure.json
$LASTEXITCODE
```

Shell POSIX :

```bash
dirloom . --format json --output structure.json
echo $?
```

## 12. Ce qui n’est pas encore disponible

La roadmap contient de nombreux exemples prospectifs. Les capacités suivantes ne font pas encore partie de la CLI actuelle :

| Capacité future | Alternative actuelle |
| --- | --- |
| `--copy` natif | Pipe vers `Set-Clipboard`, `pbcopy`, `wl-copy` ou `xclip` |
| Couleurs, icônes et thèmes | Rendus Unicode ou ASCII non colorés |
| `browse` ou TUI | Générer une sortie texte, Markdown ou JSON |
| Fingerprint, snapshot, verify et diff natifs | Versionner le JSON et utiliser Git ou un outil de diff |
| `watch` et flux d’événements | Relancer Dirloom depuis un outil externe |
| Scaffold, templates et Architecture Packs | Utiliser un générateur externe, puis réinspecter la structure |
| Contracts, drift, conform et réconciliation | Contrôles externes sur le JSON, sans moteur natif Dirloom |
| Query et metrics | Interroger le JSON avec PowerShell, `jq` ou un programme |
| Analyse des imports et dépendances | Dirloom v0.1 observe uniquement le système de fichiers |
| Context Compiler, receipts, MCP et Agent Skills | Fournir manuellement le Markdown ou le JSON aux outils concernés |
| `--follow-symlinks` | Les liens restent terminaux ; aucune traversée native |
| Desktop et intégrations IDE | Utiliser la CLI et ses artefacts |

Cette séparation doit être maintenue à chaque évolution du guide : une commande ne passe dans les sections principales qu’après son implémentation et la stabilisation de son contrat.

## 13. Mémento des combinaisons utiles

```bash
# Vue rapide
dirloom --depth 2

# Expliquer les valeurs persistantes et leur origine
dirloom config explain

# Expliquer la définition intrinsèque d’un preset
dirloom preset explain ai

# Appliquer uniquement la configuration projet et la CLI
dirloom --no-user-config

# Architecture uniquement
dirloom --preset compact --depth 4

# Markdown partageable
dirloom --preset docs

# Compatibilité ASCII
dirloom --depth 4 --style ascii

# Artefact machine
dirloom --format json --output structure.json

# Masquer les sorties générées
dirloom --ignore dist --ignore build --ignore "**/generated/**"

# Montrer les entrées cachées encore autorisées
dirloom --hidden

# Désactiver seulement .gitignore
dirloom --no-gitignore

# Vue physique étendue
dirloom --hidden --no-gitignore --no-default-ignore --depth 3

# Contexte structurel compact pour une IA
dirloom --preset ai
```

## 14. Faire évoluer ce guide

Lorsqu’une fonctionnalité Dirloom est livrée :

1. vérifier la commande avec `dirloom --help` et les tests de la version ;
2. ajouter au moins un exemple minimal et un scénario réaliste ;
3. documenter le résultat attendu, les erreurs et les interactions avec les options existantes ;
4. distinguer les étapes natives des outils externes ;
5. déplacer la capacité correspondante hors de la section « Ce qui n’est pas encore disponible » ;
6. mettre à jour la carte rapide et le mémento si la commande devient fréquente ;
7. préserver les exemples des anciennes versions uniquement s’ils restent valides ou clairement versionnés.

L’objectif n’est pas d’accumuler des commandes, mais de maintenir un catalogue fiable des problèmes que la version courante de Dirloom sait réellement résoudre.
