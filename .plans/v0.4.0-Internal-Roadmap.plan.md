# Dirloom v0.4 — Internal Roadmap

## Structural Version Control

**Statut :** roadmap interne d'implémentation
**Release cible :** `v0.4.0`
**Prérequis :** v0.3 entièrement implémentée et stabilisée
**Nature :** découpage normatif des incréments de construction de v0.4
**Objectif :** fournir la base structurelle versionnable sur laquelle reposeront ensuite History, Contracts, Shape Diff, Drift, Packs, Context Receipts et les autres capacités d'intelligence de Dirloom.

---

# 1. Mission de v0.4

v0.4 fait passer Dirloom de :

```text
observe + represent
```

à :

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

La release doit permettre de traiter une structure comme un **état identifiable, sérialisable, comparable et historiquement observable**.

Chaîne fondamentale :

```text
Structural Source
      │
      ▼
Canonical Structural Artifact
      │
      ├── Fingerprint
      │
      ├── Snapshot
      │
      ├── Verify
      │
      ▼
Structural Comparison Engine
      │
      ├── Diff
      ├── Move Detection
      ├── Git Sources
      ├── History
      └── Watch
```

---

# 2. Invariants globaux de v0.4

Ces invariants s'appliquent à tous les incréments.

## 2.1 Une seule vérité structurelle

`fingerprint`, `snapshot`, `verify`, `diff`, Git, `history` et `watch` ne doivent jamais reconstruire chacun leur propre représentation de l'arborescence.

Ils consomment tous :

```text
Canonical Structural Artifact
```

ou une abstraction strictement équivalente.

---

## 2.2 Canonical Mode ≠ Presentation Mode

Aucune information provenant de :

* thème ;
* couleur ;
* ANSI ;
* icône ;
* terminal ;
* largeur de console ;
* TTY ;
* environnement graphique ;

ne doit influencer :

* fingerprint ;
* snapshot structurel ;
* verify ;
* diff machine ;
* identité d'un nœud.

---

## 2.3 Fingerprint structurel uniquement

Le fingerprint v1 doit représenter la **structure observée**, pas l'environnement ayant produit l'observation.

Par défaut sont exclus de l'identité structurelle :

```text
mtime
ctime
owner
permissions variables
absolute root path
machine hostname
current date/time
terminal
theme
presentation
```

Toute future observation metadata devra rester séparée de l'artefact canonique.

---

## 2.4 Même structure → même identité

Pour deux artefacts structurellement équivalents :

```text
Artifact A == Artifact B
```

alors :

```text
fingerprint(A) == fingerprint(B)
diff(A, B) == empty
verify(A, snapshot(B)) == success
```

indépendamment de l'interface ayant produit l'artefact.

---

## 2.5 Local-first

Toutes les fonctions fondamentales de v0.4 doivent fonctionner sans réseau.

Git signifie ici avant tout :

* dépôt local ;
* refs locales ;
* objets Git locaux.

Aucun `git fetch`, clone ou accès distant implicite.

---

## 2.6 Aucun contenu obligatoire

Le contenu des fichiers ne doit pas être nécessaire au fonctionnement normal de :

```text
fingerprint
snapshot
verify
diff
move detection v1
history
watch
```

Les futures stratégies utilisant le contenu devront être explicitement optionnelles.

---

## 2.7 Résultats déterministes et explicables

Lorsque plusieurs interprétations sont possibles, Dirloom doit préférer :

```text
UNKNOWN / AMBIGUOUS
```

à une déduction non fiable.

C'est particulièrement important pour la détection des déplacements.

---

## 2.8 Les formats machines sont des contrats

Tout format machine public introduit par v0.4 doit avoir :

```text
schemaVersion
```

et disposer :

* d'un schéma documenté ;
* de tests de compatibilité ;
* de golden fixtures ;
* d'une stratégie d'évolution.

---

## 2.9 stdout reste exploitable

Les règles existantes restent applicables :

```text
stdout → résultat demandé
stderr → diagnostics
exit code → résultat d'exécution
```

Aucun message décoratif ne doit contaminer JSON ou NDJSON.

---

## 2.10 Pas de compatibilité implicite cassée avec v0.1–v0.3

v0.4 ne doit pas modifier silencieusement :

* scanner ;
* filtres ;
* `.gitignore` ;
* configuration ;
* ordre déterministe ;
* symlinks ;
* thèmes ;
* exports ;
* JSON existant.

Une évolution nécessaire doit être explicite, testée et documentée.

---

# 3. Découpage officiel

```text
v0.4.0-a1  Artifact Identity + Fingerprint
v0.4.0-a2  Snapshot
v0.4.0-a3  Verify
v0.4.0-a4  Structural Diff
v0.4.0-a5  Move Detection
v0.4.0-a6  Git Sources
v0.4.0-a7  History Primitives
v0.4.0-a8  Experimental Watch
v0.4.0-rc  Contract Freeze + Cross-platform Hardening
v0.4.0     Structural Version Control
```

Chaque incrément doit être utilisable et testable sans dépendre d'un incrément futur.

---

# 4. v0.4.0-a1 — Artifact Identity + Fingerprint

## Objectif

Définir ce que signifie exactement :

> « Ces deux observations représentent la même structure Dirloom. »

C'est l'incrément le plus fondamental de toute v0.4.

Aucun autre chantier de la release ne doit être construit tant que ce contrat n'est pas solide.

---

## A1.1 — Canonical Structural Artifact

Formaliser le modèle canonique partagé.

Le modèle doit au minimum représenter les informations structurelles nécessaires à l'identité :

```text
node
├── relative path
├── name
├── node type
├── hierarchy
├── deterministic structural attributes
└── children
```

Le modèle ne doit pas dépendre d'un renderer.

Décider explicitement :

* identité de la racine ;
* représentation des fichiers ;
* représentation des dossiers ;
* symlinks ;
* junctions Windows ;
* liens cassés ;
* fichiers spéciaux si applicables ;
* chemins relatifs ;
* séparateur canonique ;
* ordre des enfants.

---

## A1.2 — Canonicalization Contract v1

Écrire une spécification normative définissant la transformation :

```text
Observed Structure
        ↓
Canonical Projection v1
```

Elle doit couvrir :

### Paths

Tous les chemins structurels doivent utiliser une représentation portable et indépendante du séparateur natif.

Le chemin absolu du projet ne doit pas participer à l'identité.

### Ordering

L'ordre doit être explicitement déterministe et utiliser les règles déjà garanties par Dirloom.

### Unicode

Définir officiellement la stratégie de normalisation Unicode.

Les cas où deux noms distincts deviennent équivalents après normalisation doivent être détectés plutôt que fusionnés silencieusement.

### Case sensitivity

Ne pas convertir arbitrairement les chemins en minuscules.

La sémantique doit permettre qu'un même checkout Git produise un artefact comparable entre plateformes sans masquer les collisions réellement impossibles sur certains systèmes de fichiers.

### Symlinks

La politique existante de Dirloom devient partie du contrat d'identité.

La cible d'un lien ne doit jamais être suivie implicitement uniquement pour le fingerprint.

---

## A1.3 — Identity Projection

Introduire une projection interne spécifiquement destinée au calcul d'identité.

```text
Canonical Artifact
       ↓
Identity Projection v1
       ↓
Canonical Serialization
       ↓
SHA-256
```

Cela permet de faire évoluer l'artefact avec de nouvelles métadonnées sans changer automatiquement tous les fingerprints.

---

## A1.4 — Canonical Serialization

Définir une sérialisation stable indépendante :

* de `map` Go ;
* de l'ordre d'allocation ;
* de la plateforme ;
* du renderer ;
* des espaces ou indentation JSON.

Ne jamais calculer un fingerprint directement sur un JSON arbitrairement sérialisé.

---

## A1.5 — Fingerprint Specification v1

Format cible :

```text
dlm:v1:sha256:<digest>
```

Le préfixe doit rendre visibles :

```text
namespace
identity version
hash algorithm
digest
```

Le SHA-256 v1 doit être considéré comme un détail du contrat versionné et non comme une constante éternelle.

---

## A1.6 — CLI `fingerprint`

Commande :

```bash
dirloom fingerprint
```

Doivent être étudiés et stabilisés :

```bash
dirloom fingerprint --root .
dirloom fingerprint --format text
dirloom fingerprint --format json
```

Les filtres/configurations Dirloom existants doivent continuer à déterminer **la vue structurelle capturée**.

Par conséquent :

```text
same filesystem
+ different structural filters
= potentially different artifact
= potentially different fingerprint
```

Cela doit être documenté.

---

## A1.7 — Source abstraction

Créer dès a1 l'abstraction nécessaire pour que le moteur ne soit pas couplé au filesystem.

Conceptuellement :

```text
StructuralSource
    ↓
BuildArtifact()
    ↓
Canonical Artifact
```

Première implémentation :

```text
FilesystemSource
```

a6 ajoutera :

```text
GitSource
```

Le diff ne devra donc jamais devenir un moteur « filesystem vs filesystem » spécialisé.

---

## A1.8 — Validation et diagnostics

Définir des erreurs stables pour :

* collisions canoniques ;
* chemins invalides ;
* type non supporté ;
* artefact impossible à canonicaliser ;
* erreur de lecture ;
* incohérence interne.

---

## A1.9 — Tests obligatoires

Corpus au minimum :

```text
empty project
single file
nested directories
deep tree
hidden files
Unicode
spaces
special characters
symlinks
broken symlinks
Windows junctions
case variants
normalization variants
large directory
filters
.gitignore
custom ignore
depth limits
```

Propriétés à tester :

```text
same artifact → same fingerprint
different meaningful structure → different fingerprint
presentation changes → same fingerprint
different absolute roots → same fingerprint
Windows/Linux/macOS fixtures → same canonical identity
```

Ajouter property tests et fuzzing sur canonicalisation/sérialisation.

---

## A1.10 — Performance baseline

Créer les premiers benchmarks v0.4 :

* 1k nodes ;
* 10k nodes ;
* 100k nodes si raisonnable ;
* profondeur élevée ;
* fort fan-out.

Mesurer séparément :

```text
scan
canonicalization
serialization
hash
total
```

---

## Gate de sortie a1

a1 est terminée lorsque :

```text
✓ canonical artifact contract documented
✓ identity projection v1 documented
✓ fingerprint spec v1 frozen for alpha
✓ dirloom fingerprint usable
✓ filesystem source abstraction established
✓ deterministic cross-platform fixtures pass
✓ presentation cannot affect fingerprint
✓ fuzz/property tests exist
✓ performance baseline recorded
```

---

# 5. v0.4.0-a2 — Snapshot

## Objectif

Transformer un artefact observé en référence persistante et vérifiable.

```text
Live Structure
      ↓
Canonical Artifact
      ↓
Snapshot
```

---

## A2.1 — Snapshot Schema v1

Définir le premier schéma dédié aux snapshots.

Conceptuellement :

```text
Snapshot
├── schemaVersion
├── artifactVersion
├── fingerprint
├── capture semantics
├── structural artifact
└── optional provenance
```

---

## A2.2 — Séparer structure et provenance

Ne pas polluer l'identité structurelle avec :

* date actuelle ;
* hostname ;
* utilisateur ;
* chemin absolu.

Si une provenance est conservée, elle doit être explicitement hors du calcul du fingerprint.

Évaluer si elle doit être :

```text
optional
```

ou complètement séparée du payload canonique.

---

## A2.3 — Capture semantics

Le snapshot doit conserver suffisamment d'information pour comprendre comment il a été produit.

Exemples :

```text
depth
dirsOnly
hidden
ignore semantics
gitignore usage
structural options
artifact schema version
```

Attention :

> Le snapshot doit capturer le résultat structurel, pas dépendre de la relecture future de la configuration pour être compris.

---

## A2.4 — CLI `snapshot`

Commande de référence :

```bash
dirloom snapshot --output architecture.dlm.json
```

Comportements obligatoires :

* stdout possible ;
* `--output` ;
* écriture transactionnelle ;
* pas de fichier partiel ;
* refus propre des conflits ;
* diagnostics stderr ;
* JSON strict.

---

## A2.5 — Snapshot validation

Prévoir un validateur partagé capable de détecter :

```text
invalid JSON
unsupported schema
missing required field
malformed path
duplicate canonical node
fingerprint mismatch
invalid node hierarchy
unknown mandatory feature
```

Ce validateur sera réutilisé par `verify` et `diff`.

---

## A2.6 — Self-verification

Lors du chargement :

```text
snapshot fingerprint
```

doit pouvoir être recalculé depuis son artefact.

Une divergence signifie :

```text
CORRUPT / INVALID SNAPSHOT
```

et non :

```text
STRUCTURE MISMATCH
```

---

## A2.7 — Snapshot compatibility policy

Documenter dès a2 :

* ce qu'un reader v1 doit accepter ;
* comment les extensions futures seront traitées ;
* différence champ inconnu / version inconnue ;
* politique additive.

---

## A2.8 — Golden fixtures

Versionner plusieurs snapshots de référence afin qu'ils deviennent des contrats de non-régression.

---

## Gate de sortie a2

```text
✓ snapshot schema v1 defined
✓ snapshot serialization deterministic
✓ transactional output
✓ snapshot validator shared
✓ embedded fingerprint validated
✓ corrupt snapshot distinguished from mismatch
✓ golden fixtures committed
✓ large snapshot tests pass
```

---

# 6. v0.4.0-a3 — Verify

## Objectif

Répondre de façon fiable à :

> « La structure observée correspond-elle toujours à ce snapshot ? »

---

## A3.1 — Verification Engine

Pipeline :

```text
Snapshot
    ↓ validate
Expected Fingerprint

Current Source
    ↓ artifact
Current Fingerprint

Compare
```

Le moteur ne doit pas refaire une implémentation différente de la canonicalisation.

---

## A3.2 — Résultats distincts

Le modèle doit distinguer au minimum :

```text
MATCH
MISMATCH
INVALID_SNAPSHOT
UNSUPPORTED_SNAPSHOT
SOURCE_ERROR
INTERNAL_ERROR
```

---

## A3.3 — CLI

```bash
dirloom verify architecture.dlm.json
```

Sortie humaine courte :

```text
Structure matches snapshot.
```

ou :

```text
Structure differs from snapshot.
```

Le mode machine doit exposer les fingerprints attendus et observés.

---

## A3.4 — Exit-code contract

Stabiliser une convention compatible avec les pratiques CLI et CI.

Au minimum :

```text
success
structural mismatch
invalid input / usage
operational/internal error
```

Les nombres exacts deviennent un contrat public et doivent être testés.

---

## A3.5 — CI ergonomics

`verify` doit fonctionner proprement dans :

* GitHub Actions ;
* GitLab CI ;
* Azure DevOps ;
* Jenkins ;
* shells POSIX ;
* PowerShell.

Pas de couleur automatique lorsque non pertinente.

---

## A3.6 — Explicabilité limitée mais utile

a3 ne doit pas réimplémenter `diff`.

En revanche, le résultat peut indiquer :

```text
expected fingerprint
actual fingerprint
```

et suggérer :

```bash
dirloom diff ...
```

lorsque disponible à partir de a4.

---

## Gate de sortie a3

```text
✓ verify distinguishes mismatch from invalid snapshot
✓ stable exit-code contract
✓ JSON result available
✓ CI integration tests
✓ no diff logic duplicated
✓ snapshot validation reused
```

---

# 7. v0.4.0-a4 — Structural Diff

## Objectif

Répondre à :

> « Qu'est-ce qui a changé structurellement entre A et B ? »

C'est la première killer-feature visible de v0.4.

---

## A4.1 — Comparison Model

Introduire un modèle interne dédié :

```text
StructuralDiff
├── metadata
├── sourceA
├── sourceB
├── summary
└── changes[]
```

Changements v1 avant move detection :

```text
ADDED
REMOVED
CHANGED
```

Un déplacement apparaîtra temporairement comme :

```text
REMOVED + ADDED
```

jusqu'à a5.

---

## A4.2 — Définir précisément `CHANGED`

Dans un moteur structurel, modifier uniquement le contenu d'un fichier ne constitue pas nécessairement un structural change.

`CHANGED` doit donc être réservé à une modification d'attribut structurel canonique.

Exemples potentiels :

* type de nœud ;
* nature du lien ;
* attribut déterministe faisant partie de l'identity model.

Les métriques dérivées comme :

```text
files: 18 → 27
```

sont des résumés, pas forcément des événements primitifs.

---

## A4.3 — Source combinations

a4 doit couvrir au minimum :

```text
snapshot ↔ snapshot
snapshot ↔ live filesystem
live filesystem ↔ snapshot
```

Le moteur doit déjà utiliser l'abstraction `StructuralSource`.

Git arrivera en a6 sans modifier l'algorithme de diff.

---

## A4.4 — Diff Engine

Exigences :

* déterministe ;
* stable ordering ;
* complexité maîtrisée ;
* traitement de gros arbres ;
* aucune dépendance à la présentation ;
* aucune mutation.

---

## A4.5 — Human Renderer

Exemple :

```text
Structural Diff

ADDED
  + src/features/checkout/

REMOVED
  - src/legacy/payment/

CHANGED
  ~ src/shared/auth
```

Les couleurs éventuelles restent purement présentationnelles.

---

## A4.6 — Machine-readable Diff v1

Créer un schéma dédié.

Conceptuellement :

```json
{
  "schemaVersion": 1,
  "sources": {},
  "summary": {},
  "changes": []
}
```

Chaque changement doit pouvoir être traité automatiquement.

---

## A4.7 — Empty diff contract

Un diff sans changement doit produire :

```text
empty changes[]
```

avec un exit code documenté.

Ne pas confondre :

```text
no differences
```

et :

```text
failure
```

---

## A4.8 — Summary

Produire des agrégats déterministes :

```text
added
removed
changed
total
```

Puis `moved` sera ajouté en a5.

---

## A4.9 — Test matrix

Inclure :

```text
empty ↔ empty
empty ↔ populated
single addition
single removal
nested addition
nested removal
type change
large subtree
massive fan-out
deep path
Unicode
case
symlink
filtered snapshot
different snapshot versions
malformed source
```

Tests de symétrie :

```text
diff(A,B)
vs
diff(B,A)
```

doivent inverser correctement les opérations.

---

## A4.10 — Performance

Benchmarks :

```text
1k vs 1k
10k vs 10k
100k vs 100k
1% changed
50% changed
completely different
```

Pas d'algorithme quadratique non borné sur le chemin standard.

---

## Gate de sortie a4

```text
✓ shared diff engine
✓ snapshot/live combinations
✓ deterministic text diff
✓ machine diff schema v1
✓ empty-diff semantics
✓ stable change ordering
✓ performance benchmarks
✓ no move heuristic yet
```

---

# 8. v0.4.0-a5 — Move Detection v1

## Objectif

Transformer certains couples :

```text
REMOVED + ADDED
```

en :

```text
MOVED
```

lorsqu'une correspondance suffisamment fiable peut être démontrée.

---

## A5.1 — Principle

La détection doit être :

```text
deterministic
explainable
conservative
versioned
```

Une incertitude doit rester :

```text
REMOVED + ADDED
```

plutôt qu'un faux déplacement.

---

## A5.2 — Tier v1

v1 reste structurelle.

Signaux possibles :

```text
node type
basename
relative path similarity
parent similarity
subtree shape
child identities
subtree fingerprint
```

Pas de lecture obligatoire du contenu.

---

## A5.3 — One-to-one matching

Un nœud source ne peut produire qu'un seul mouvement.

Un nœud destination ne peut recevoir qu'un seul mouvement.

Les ambiguïtés doivent être résolues de façon déterministe ou rejetées.

---

## A5.4 — Move identity

Le diff machine doit représenter explicitement :

```text
from
to
```

et éventuellement :

```text
strategy
confidence class
algorithm version
```

si ces informations sont utiles au diagnostic.

Éviter les pseudo-pourcentages opaques.

---

## A5.5 — Rename vs Move

Décider si v1 expose :

```text
MOVED
```

comme notion générale couvrant :

* déplacement ;
* renommage ;
* déplacement + renommage ;

ou plusieurs catégories.

Le choix doit être documenté avant freeze.

---

## A5.6 — Algorithm versioning

La stratégie doit être nommée/versionnée afin qu'une évolution future n'altère pas silencieusement les résultats historiques.

Exemple conceptuel :

```text
moveDetection: structural-v1
```

---

## A5.7 — Adversarial fixtures

Tester :

```text
two identical empty folders
repeated module shapes
renamed sibling directories
large subtree moved intact
partial subtree rewrite
same basename in multiple places
many generated folders
ambiguous candidates
```

---

## Gate de sortie a5

```text
✓ MOVED introduced as first-class diff event
✓ conservative matching
✓ ambiguity safe fallback
✓ algorithm version exposed internally/publicly as needed
✓ deterministic across platforms
✓ adversarial fixtures
✓ no content dependency
```

---

# 9. v0.4.0-a6 — Git Sources

## Objectif

Permettre au même moteur structurel de travailler sur des arbres Git.

```text
FilesystemSource
GitSource
      ↓
Canonical Artifact
      ↓
same fingerprint/diff engines
```

---

## A6.1 — Git source adapter

Implémenter Git derrière l'abstraction créée en a1.

Le core de diff ne doit pas savoir s'il compare :

```text
filesystem
snapshot
Git tree
```

---

## A6.2 — Supported refs v1

Support minimum :

```text
HEAD
branch
tag
commit SHA
relative revisions where supported
```

Exemples cible :

```bash
dirloom diff main feature/payments
dirloom diff HEAD~10 HEAD
```

La syntaxe exacte de désambiguïsation entre fichier et ref doit être explicitement spécifiée.

---

## A6.3 — Working tree semantics

Distinguer clairement :

```text
Git committed tree
working tree
```

Un arbre Git ne contient pas nécessairement :

* untracked files ;
* ignored files ;
* filesystem-only artefacts.

Les différences de sémantique doivent être visibles et documentées.

---

## A6.4 — Ignore semantics

Décider quelles règles sont appliquées à un Git tree déjà versionné.

Une règle `.gitignore` ne doit pas faire disparaître arbitrairement un fichier déjà présent dans un commit sauf si cela correspond au contrat Dirloom choisi.

Le comportement doit être cohérent avec les filtres Dirloom existants.

---

## A6.5 — Git object types

Définir le traitement de :

* regular blob ;
* executable blob ;
* tree ;
* symlink ;
* submodule/gitlink.

Ne pas suivre automatiquement un submodule.

---

## A6.6 — No implicit network

Une ref absente localement doit produire un diagnostic clair.

Dirloom ne doit jamais automatiquement faire :

```text
fetch
pull
clone
```

---

## A6.7 — Repository discovery

Définir :

* repo courant ;
* racine Git ;
* `--root` ;
* invocation depuis sous-dossier ;
* bare repository si supporté ;
* worktree Git.

---

## A6.8 — Security

Traiter les données Git comme non fiables :

* chemins malformés ;
* path traversal conceptuel ;
* noms invalides ;
* arbres énormes ;
* références malicieuses ;
* objets manquants/corrompus.

---

## A6.9 — Tests

Fixtures Git réelles :

```text
branch vs branch
tag vs branch
commit vs commit
HEAD~n
rename
move
symlink
submodule
deleted tree
empty tree
detached HEAD
worktree
bare repo if supported
invalid ref
corrupt object
```

---

## Gate de sortie a6

```text
✓ GitSource uses common artifact pipeline
✓ branch/tag/commit refs
✓ main↔feature and HEAD~n↔HEAD
✓ no implicit network
✓ submodules handled safely
✓ committed-tree semantics documented
✓ cross-platform Git fixtures
```

---

# 10. v0.4.0-a7 — History Primitives

## Objectif

Fournir les primitives nécessaires pour observer l'évolution structurelle dans le temps sans construire prématurément la Time Machine complète.

---

## A7.1 — History Model

Conceptuellement :

```text
History Entry
├── revision identity
├── structural fingerprint
├── parent relationship
├── structural change summary
└── optional source metadata
```

---

## A7.2 — CLI v1

Commande cible :

```bash
dirloom history
```

Options à étudier :

```bash
dirloom history --limit 20
dirloom history --from <ref> --to <ref>
dirloom history --format json
```

---

## A7.3 — Change summaries

Pour chaque étape pertinente :

```text
added
removed
moved
changed
fingerprint
```

Ne pas recalculer ou afficher un diff détaillé énorme par défaut.

---

## A7.4 — Structural-change filtering

Évaluer la possibilité de ne montrer que les commits ayant réellement changé la structure.

Exemple :

```text
commit modifies README content only
→ structural fingerprint unchanged
```

Dirloom peut alors l'ignorer dans une vue purement structurelle.

---

## A7.5 — Cache architecture

L'historique peut nécessiter de nombreux fingerprints.

Introduire si nécessaire un cache local :

```text
revision → structural fingerprint
```

avec :

* invalidation explicable ;
* emplacement documenté ;
* absence d'effet sur le résultat ;
* possibilité de désactivation/nettoyage.

Ne jamais sacrifier la correction au cache.

---

## A7.6 — Merge commits

Définir le comportement :

* first-parent ;
* full graph ;
* parent selection.

Le comportement par défaut doit rester compréhensible et déterministe.

---

## A7.7 — Machine format

Fournir un format versionné utilisable plus tard par :

* Time Machine ;
* Desktop ;
* Drift ;
* reporting.

---

## A7.8 — Hors scope explicite

a7 ne doit pas encore devenir :

* Architecture Drift ;
* analytics avancés ;
* graphiques temporels complexes ;
* Desktop Time Machine ;
* stockage de base de données ;
* service cloud.

---

## Gate de sortie a7

```text
✓ structural history model
✓ revision fingerprints
✓ concise change summaries
✓ JSON history output
✓ merge semantics documented
✓ cache correctness tested if cache exists
✓ content-only commits distinguishable from structural changes
```

---

# 11. v0.4.0-a8 — Experimental Watch

## Objectif

Exposer les changements structurels locaux sous forme de flux d'événements.

```bash
dirloom watch --format ndjson
```

---

## A8.1 — Experimental status

`watch` est explicitement :

```text
EXPERIMENTAL
```

en v0.4.

Sa présence ne doit pas empêcher le reste des contrats v0.4 d'être stable.

---

## A8.2 — Event model

Événements cible :

```text
node.added
node.removed
node.changed
node.moved
rescan.started
rescan.completed
overflow
error
```

La liste exacte doit être minimisée et documentée.

---

## A8.3 — NDJSON

Chaque ligne constitue un événement autonome.

Exemple :

```json
{"schemaVersion":1,"event":"node.added","path":"src/features/payments"}
```

Aucun diagnostic humain sur stdout.

---

## A8.4 — Watcher abstraction

Les notifications natives du filesystem ne doivent pas devenir la vérité métier.

Pipeline cible :

```text
OS filesystem events
       ↓
Watcher Adapter
       ↓
coalescing / debounce
       ↓
re-observation
       ↓
Canonical Artifact / Diff
       ↓
Structural Events
```

Cela évite d'exposer directement les incohérences de `inotify`, FSEvents ou Windows change notifications.

---

## A8.5 — Debounce et coalescing

Les éditeurs peuvent produire plusieurs opérations temporaires :

```text
write temp
rename temp
delete old
rename new
```

Dirloom doit produire autant que possible un événement structurel cohérent plutôt qu'un bruit brut.

---

## A8.6 — Overflow and recovery

Si le watcher perd des événements :

```text
overflow
```

doit déclencher une stratégie fiable de rescan.

Dirloom ne doit jamais continuer silencieusement avec un état possiblement faux.

---

## A8.7 — Filters

Les mêmes règles que le scanner doivent continuer à s'appliquer :

* ignore ;
* gitignore ;
* hidden ;
* depth ;
* configuration.

---

## A8.8 — Lifecycle

Couvrir proprement :

* Ctrl+C ;
* SIGTERM lorsque applicable ;
* fermeture des handles ;
* suppression de la racine ;
* recréation de dossiers ;
* erreurs temporaires.

---

## A8.9 — Backpressure

Un consommateur lent ne doit pas provoquer :

* croissance mémoire non bornée ;
* deadlock ;
* corruption d'état.

Définir la stratégie :

```text
buffer
coalescing
overflow event
rescan
```

---

## A8.10 — Cross-platform tests

Tester réellement :

```text
Linux
Windows
macOS
```

et non uniquement par mocks.

---

## Gate de sortie a8

```text
✓ NDJSON structural event stream
✓ shared canonical model
✓ debounce/coalescing
✓ overflow recovery
✓ filter consistency
✓ graceful shutdown
✓ bounded resource usage
✓ real OS integration tests
✓ clearly marked experimental
```

---

# 12. v0.4.0-rc — Contract Freeze + Cross-platform Hardening

## Objectif

Aucune nouvelle fonctionnalité.

Le RC transforme huit incréments fonctionnels en une **release publique cohérente et prod-grade**.

---

# 12.1 — Public Contract Inventory

Établir la liste exhaustive des contrats introduits :

```text
fingerprint format
identity projection version
snapshot schema
diff schema
history schema
watch event schema
CLI commands
flags
exit codes
Git ref semantics
move detection semantics
error categories
```

Chaque contrat doit avoir :

```text
owner
documentation
tests
compatibility policy
```

---

# 12.2 — CLI consistency audit

Auditer :

```text
dirloom fingerprint
dirloom snapshot
dirloom verify
dirloom diff
dirloom history
dirloom watch
```

Vérifier :

* noms ;
* flags ;
* help ;
* examples ;
* errors ;
* output ;
* `--format` ;
* config resolution ;
* root semantics.

Éliminer les incohérences avant release.

---

# 12.3 — Machine-schema freeze

Geler les schémas publics de v0.4 hors `watch` si celui-ci reste officiellement expérimental.

Ajouter :

* JSON Schema si pertinent ;
* fixtures ;
* compatibility tests ;
* malformed-input tests.

---

# 12.4 — Cross-platform matrix

CI obligatoire :

```text
Ubuntu
Windows
macOS
```

avec tests spécifiques :

```text
path separators
drive roots
case behavior
Unicode
symlinks
junctions
permissions
Git worktrees
line endings
terminal independence
```

---

# 12.5 — Security hardening

Fuzzing et tests adversariaux sur :

```text
snapshot parser
canonical paths
Unicode
Git refs
Git tree paths
deep recursion
huge trees
malformed JSON
duplicate nodes
path traversal
symlink edge cases
resource exhaustion
```

Aucun snapshot ou dépôt malformé ne doit provoquer panic ou écriture hors périmètre.

---

# 12.6 — Performance gates

Établir des baselines reproductibles pour :

```text
fingerprint
snapshot
verify
diff
move detection
Git artifact build
history
watch rescan
```

Cas :

```text
1k
10k
100k nodes
deep tree
wide tree
large diff
mostly identical
completely different
```

Toute régression importante doit être visible en CI ou benchmarks versionnés.

---

# 12.7 — Memory profile

Contrôler les pics mémoire.

Particulièrement pour :

* deux gros snapshots ;
* diff ;
* move detection ;
* history ;
* watcher.

Éviter de conserver plusieurs copies complètes inutiles du même artefact.

---

# 12.8 — Concurrency and race detection

Exécuter :

```text
go test -race
```

sur les composants pertinents.

Tester :

* caches ;
* watcher ;
* output transactionnel ;
* parallélisme éventuel.

---

# 12.9 — Panic policy

Aucune entrée utilisateur valide ou malformée ne doit entraîner un panic récupérable.

Les invariants internes impossibles peuvent rester des assertions de développement, mais l'interface publique doit échouer proprement.

---

# 12.10 — Error taxonomy

Uniformiser les erreurs :

```text
usage
validation
unsupported
mismatch
source
filesystem
git
corruption
internal
```

Les erreurs machines doivent rester stables et exploitables.

---

# 12.11 — Regression suite v0.1–v0.3

Avant publication :

```text
all historical tests pass
all output golden tests pass
theme behavior unchanged
JSON v1 unchanged unless explicitly versioned
configuration precedence unchanged
filters unchanged
exports unchanged
package behavior unchanged
```

---

# 12.12 — Documentation

Documentation publique minimale :

```text
Structural Version Control overview
Fingerprint
Snapshots
Verify
Structural Diff
Move Detection
Git comparisons
History
Watch experimental
Machine schemas
Exit codes
CI examples
Security / trust model
Compatibility policy
```

Ajouter des exemples réellement copiables.

---

# 12.13 — Examples / Showcase

Créer au moins un scénario reproductible :

```text
v1 project
↓
snapshot
↓
refactoring
↓
verify fails
↓
diff
↓
move detected
↓
compare Git branches
↓
history
```

Ce scénario servira à :

* README ;
* documentation ;
* tests E2E ;
* démo release.

---

# 12.14 — Release engineering

Vérifier :

```text
version metadata
GoReleaser
checksums
SBOM
attestations
signing policy
archives
package-manager compatibility
upgrade from v0.3
fresh installation
```

Aucune régression de distribution ne doit être introduite par v0.4.

---

# 12.15 — RC soak

Le RC doit être utilisé réellement sur plusieurs repositories de tailles et technologies différentes avant `v0.4.0`.

Profils recommandés :

```text
small Go CLI
TypeScript monorepo
Flutter project
large repository
repository with symlinks
repository with deep Git history
Windows-heavy repository
```

Les problèmes découverts pendant le soak doivent être classés :

```text
contract bug
correctness bug
performance issue
UX inconsistency
future enhancement
```

Seules les trois premières catégories bloquent systématiquement la release lorsqu'elles affectent les workflows principaux.

---

# 13. Definition of Done globale de v0.4

`v0.4.0` ne peut être publié que lorsque les conditions suivantes sont réunies.

## Identity

```text
✓ deterministic canonical artifact
✓ identity projection v1
✓ stable fingerprint format
✓ same structure = same fingerprint
```

## Persistence

```text
✓ snapshot schema v1
✓ validation
✓ corruption detection
✓ transactional output
```

## Verification

```text
✓ reliable verify
✓ stable exit codes
✓ CI-friendly machine results
```

## Comparison

```text
✓ added
✓ removed
✓ changed
✓ moved
✓ deterministic ordering
✓ machine-readable diff
```

## Git

```text
✓ local refs
✓ commit/branch/tag comparisons
✓ no implicit network
✓ Git-specific edge cases
```

## History

```text
✓ historical structural fingerprints
✓ structural change summaries
✓ machine output
```

## Watch

```text
✓ experimental structural event stream
✓ overflow recovery
✓ bounded resource behavior
```

## Quality

```text
✓ Windows
✓ Linux
✓ macOS
✓ fuzzing
✓ race tests
✓ benchmarks
✓ large-tree tests
✓ v0.1–v0.3 regression suite
```

## Product

```text
✓ coherent CLI
✓ documentation
✓ examples
✓ schemas
✓ compatibility policy
✓ release artifacts
```

---

# 14. Correspondance roadmap stratégique → incréments

| Engagement v0.4               | Incrément |
| ----------------------------- | --------- |
| Canonical structural identity | a1        |
| Fingerprint                   | a1        |
| Snapshot                      | a2        |
| Verify                        | a3        |
| Structural Diff               | a4        |
| Machine-readable Diff         | a4        |
| Move Detection v1             | a5        |
| Git references                | a6        |
| Branch/commit comparisons     | a6        |
| History foundations           | a7        |
| Experimental Watch            | a8        |
| Cross-platform reliability    | RC        |
| Contract stability            | RC        |
| Performance hardening         | RC        |
| Security hardening            | RC        |
| Documentation / E2E           | RC        |

Ainsi, aucun élément explicitement attribué à v0.4 ne reste sans propriétaire.

---

# 15. Dépendances internes

```text
a1 Artifact Identity
 │
 ├────> a2 Snapshot
 │         │
 │         └────> a3 Verify
 │
 └────> a4 Diff
           │
           └────> a5 Move Detection
                    │
                    └────> a6 Git Sources
                              │
                              └────> a7 History
                                        │
                                        └────> a8 Watch*

a1 ──────────────────────────────────────────────┐
a2 ──────────────────────────────────────────────┤
a3 ──────────────────────────────────────────────┤
a4 ──────────────────────────────────────────────┤
a5 ──────────────────────────────────────────────┤
a6 ──────────────────────────────────────────────┤
a7 ──────────────────────────────────────────────┤
a8 ──────────────────────────────────────────────┤
                                                 ▼
                                               RC
```

`watch` dépend conceptuellement surtout de l'artifact + diff ; sa position tardive est volontaire afin d'éviter de stabiliser trop tôt un flux d'événements alors que les primitives changent encore.

---

# 16. Ce qui est explicitement hors scope v0.4

Pour empêcher le scope creep :

```text
Architecture Contracts
Shape Diff
Architecture Drift
Scaffold
Architecture Packs
Conformance
Dependency analysis
content hashing mandatory
semantic code identity
Impact Lens
Architecture Simulator
Context Compiler
MCP
Desktop
full Time Machine UI
remote Git hosting integration
cloud service
```

Ces fonctions pourront consommer les primitives de v0.4 plus tard.

---

# 17. Discipline d'implémentation pour les sous-plans agents

Chaque sous-plan dérivé de cette roadmap doit obligatoirement contenir :

```text
1. Existing-state inspection
2. Public contracts affected
3. Internal architecture changes
4. Data-model changes
5. CLI changes
6. Machine-output changes
7. Error semantics
8. Security considerations
9. Cross-platform considerations
10. Tests
11. Golden fixtures
12. Fuzz/property tests when relevant
13. Benchmarks
14. Documentation
15. Migration/backward compatibility
16. Definition of Done
17. Explicit non-goals
```

L'agent ne doit pas considérer une fonctionnalité comme terminée parce que « la commande fonctionne ».

La fin d'un incrément signifie :

> **Le comportement est architecturé, contractuel, testé, documenté, déterministe et suffisamment robuste pour devenir une dépendance de l'incrément suivant.**

---

# 18. Ordre d'exécution recommandé

Ne pas paralléliser les fondations prématurément.

Ordre strict :

```text
A1.1–A1.5
    ↓
A1.6–A1.10
    ↓
A2
    ↓
A3
    ↓
A4 core
    ↓
A4 machine/human outputs
    ↓
A5
    ↓
A6
    ↓
A7
    ↓
A8
    ↓
RC
```

Des travaux documentaires, fixtures et benchmarks peuvent être parallélisés, mais pas les contrats fondamentaux.

---

# 19. Critère de succès produit

À la fin de v0.4, ce workflow doit être naturel :

```bash
dirloom fingerprint

dirloom snapshot \
  --output architecture.dlm.json

dirloom verify architecture.dlm.json

dirloom diff architecture.dlm.json .

dirloom diff main HEAD

dirloom history

dirloom watch --format ndjson
```

Et tous ces workflows doivent être différentes projections du **même moteur structurel**, pas sept implémentations indépendantes.

---

# 20. Signature de v0.4

> **Dirloom v0.4 transforme la structure d'un projet en état versionnable : identifiable, figé, vérifiable, comparable et historiquement observable.**

La réussite de la release ne se mesure donc pas au nombre de commandes ajoutées.

Elle se mesure à l'établissement d'une primitive fiable :

```text
STRUCTURE
    ↓
CANONICAL ARTIFACT
    ↓
IDENTITY
    ↓
VERSION CONTROL
```

Cette primitive deviendra ensuite l'une des fondations principales de toute l'intelligence structurelle future de Dirloom.
