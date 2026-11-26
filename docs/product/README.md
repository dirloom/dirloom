# Documentation produit de Dirloom

> **Statut :** corpus aligné sur la roadmap stratégique votée<br>
> **Dernière consolidation :** 26 novembre 2026<br>
> **Périmètre :** évolution post-v0.1 de Dirloom

Dirloom transforme la structure logicielle en un artefact que l'on peut capturer, comparer, interroger, gouverner, matérialiser et fournir aux outils. La dernière version stable est `v0.3.3`. La v0.4, Structural Version Control, rend la structure versionnable : `fingerprint`, `snapshot`, `verify`, `diff` avec détection de déplacements et sources `git:`, `history`, et `watch` expérimental. Ses fonctionnalités ont été livrées par huit GitHub Pre-releases, de `v0.4.0-a1` à `v0.4.0-a8`. Sa première release candidate, `v0.4.0-rc1`, est en préparation sur `release/v0.4.0-rc1` : elle n'ajoute aucune fonctionnalité et prépare le gel des contrats. Les canaux stables (Winget, Scoop, Homebrew) restent sur `v0.3.3` jusqu'à `v0.4.0`. Les documents de ce dossier décrivent le produit que ce socle permet de construire.

## Parcours de lecture

| Document | Question à laquelle il répond | Public principal |
| --- | --- | --- |
| [Vision et stratégie](vision-and-strategy.md) | Pourquoi Dirloom existe-t-il, pour qui et dans quelle catégorie veut-il gagner ? | Fondateur, produit, design, engineering |
| [Principes produit](product-principles.md) | Quelles règles doivent guider les arbitrages ? | Produit, design, engineering |
| [Spécification fonctionnelle](functional-specification.md) | Comment les grandes capacités doivent-elles se comporter du point de vue utilisateur ? | Produit, design, engineering, QA |
| [Roadmap produit](roadmap.md) | Quelle direction a été votée, dans quel ordre construire les capacités et quels paris privilégier ? | Produit, engineering, contributeurs |
| [Glossaire](glossary.md) | Que signifient précisément les termes structurants ? | Tous les contributeurs |
| [Cas d’usage et exemples pratiques](../use-cases.md) | Que permet réellement la version actuelle et comment l’utiliser ? | Utilisateurs, contributeurs, intégrateurs |
| [Presets intégrés](../presets.md) | Quels profils sont disponibles, comment les activer et comment inspecter leurs effets ? | Utilisateurs, contributeurs, intégrateurs |
| [Arbres Markdown sémantiques](../markdown-tree.md) | Comment produire une liste Markdown native, sûre et déterministe ? | Utilisateurs, rédacteurs techniques, intégrateurs |
| [Exports graphiques](../graph-exports.md) | Comment produire des sources Mermaid, Graphviz et D2 déterministes ? | Utilisateurs, rédacteurs techniques, intégrateurs |
| [Couleurs, icônes et thèmes](../themes.md) | Comment personnaliser le terminal sans modifier les artefacts canoniques ? | Utilisateurs, contributeurs, intégrateurs |
| [Presse-papiers et complétions](../clipboard-and-completions.md) | Comment copier un rendu et installer les complétions shell ? | Utilisateurs, intégrateurs |
| [Aide contextuelle](../contextual-help.md) | Comment découvrir commandes, topics et valeurs CLI ? | Utilisateurs, intégrateurs |
| [Distribution](../distribution.md) | Comment installer Dirloom et vérifier une release de confiance ? | Utilisateurs, mainteneurs |
| [Catalogue sémantique](../catalog.md) | Comment Dirloom classe-t-il kinds, rôles et matchers, et comment diagnostiquer une entrée réelle ? | Utilisateurs, concepteurs de thèmes, contributeurs |
| [Structural Version Control](../structural-version-control.md) (en anglais) | Comment enregistrer, vérifier et comparer la structure d'un projet, d'un snapshot à l'historique Git ? | Utilisateurs, intégrateurs, contributeurs |
| [Intégration CI](../ci-integration.md) (en anglais) | Comment vérifier la structure à chaque pull request, avec quels codes de sortie ? | Intégrateurs, mainteneurs |
| [Codes de sortie et codes machine](../reference/exit-codes.md) (en anglais) | Sur quoi un script peut-il s'appuyer de façon stable ? | Intégrateurs |
| [Registre des contrats](../contracts/README.md) et [politique de compatibilité](../compatibility-policy.md) (en anglais) | Quelles sorties sont des contrats, avec quelle version, et que garantit le gel v0.4 ? | Intégrateurs, contributeurs |
| [Mise à niveau depuis v0.3.3](../upgrading-to-v0.4.md) (en anglais) | Qu'est-ce qui change pour un utilisateur de la version stable ? | Utilisateurs |
| [Politique de sécurité](../../SECURITY.md) (en anglais) | Que lit Dirloom, comment exécute-t-il Git, comment vérifier une release ? | Utilisateurs, mainteneurs |
| [Fingerprint](../reference/fingerprint.md) | Comment identifier la vue structurelle observée sans hasher le contenu des fichiers ? | Utilisateurs, intégrateurs, contributeurs |

Pour implémenter ou vérifier le comportement de la ligne `v0.1`, la source normative reste [SPEC-v0.1.md](../../SPEC-v0.1.md). Les documents présents n'en modifient pas rétroactivement les contrats.

## Modèle d'autorité

En cas d'écart, appliquer l'ordre suivant :

1. la spécification d'une version publiée pour le comportement de cette version ;
2. la [roadmap stratégique votée](roadmap.md) pour la North Star, les piliers, le séquencement et les investissements produit ;
3. une décision d'architecture ou une spécification de fonctionnalité acceptée, à condition qu'elle respecte cette direction ;
4. la [spécification fonctionnelle](functional-specification.md) pour le comportement cible transversal ;
5. la [vision](vision-and-strategy.md) et les [principes](product-principles.md) pour arbitrer les cas non couverts.

## État des décisions

| Sujet | Décision actuelle |
| --- | --- |
| Socle `v0.1.1` | Livré : inspection et rendu déterministes, formats texte/Markdown/JSON, filtrage et export sûr |
| Markdown sémantique `v0.2` | Livré : projection `markdown-tree` native, déterministe et distincte du Markdown textuel clôturé |
| Catégorie visée | Intelligence structurelle pour les systèmes logiciels |
| Primitive centrale | Artefact structurel déterministe et versionné |
| Scaffolding | Plateforme de génération complète : templates, plans, migrations et Architecture Packs |
| Premier Architecture Pack | Architecture FSD-like de référence, sous identifiant provisoire `reference-fsd` jusqu'à son baptême |
| Couleurs et icônes | Socle v0.2 livré : couleur TTY auto, icônes opt-in, catalogue v1, thème `vivid` two-tone indépendant, thèmes personnalisés, `theme classify` et artefacts canoniques intacts ; états architecturaux avancés futurs |
| Configuration | Socle livré : `.dirloom.yaml`, configuration utilisateur, presets intégrés et résolution inspectable |
| TUI | Après l'incrément de richesse visuelle `v0.3` : surface d'exploration de l'artefact, pas gestionnaire de fichiers |
| Accessibilité `v0.2` | Livré : `--copy`, `completion`, GitHub Releases de confiance, Scoop/Homebrew/Winget ; Release Done ≠ fusion Winget |
| Richesse visuelle `v0.3` | Publiée en `v0.3.0` : catalogue sémantique étendu, kinds/glyphes, thèmes inchangés en schéma, showcase project-centric |
| Aide contextuelle `v0.3.1` | Publiée : `dirloom help <topic>`, `--icons`/`--color` implicites `auto`, diagnostics actionnables ; `v0.4.0` reste CHANGE |
| Contrat d'icônes `v0.3.2` | Publié : `--icons ascii`, capability Nerd déclarative, `--icons auto` conservateur ; défaut `icons: never` |
| Fidélité Nerd `v0.3.3` | Publiée : provenance Nerd Fonts v3.5.1, logos vérifiés ou fallbacks, Compose/Bake/Terraform lock |
| Structural Version Control `v0.4` | Livré en GitHub Pre-releases (`v0.4.0-a1` à `v0.4.0-a8`) : `fingerprint`, `snapshot`, `verify`, `diff` (sources `snapshot:`, `live:`, `git:` ; détection de déplacements `structural-v1`), `history` ; `watch` expérimental. `v0.4.0-rc1` en préparation, sans fonctionnalité nouvelle |
| Contrats v0.4 | Candidats au gel v0.4, pas encore figés ; le gel a lieu au commit de gel de la release candidate. Structural Watch v1 et Watch Event Schema v1 restent expérimentaux et hors gel. Les contrats Markdown, les fixtures golden et les tests sont la référence normative ; les fichiers JSON Schema sont reportés après v0.4 |
| Confiance des releases | Checksums SHA-256, SBOM SPDX et attestations GitHub (provenance SLSA) ; ce n'est pas une signature cryptographique indépendante des binaires |
| Desktop | Alpha/beta en `v1.x`, produit stable et intelligence multi-repositories en `v2.x` |
| Agents de code | Jalon `v0.9` : Context Compiler, receipts, MCP, skills et Context Firewall |
| Exports graphiques | Socle v0.2 livré : vue `structure`, `diagram.Document`, sources Mermaid/Graphviz/D2, alias `dot`, budget `maxNodes` illimité |
| Extensibilité | Analyzer SDK, Pack SDK, registries de packs et de thèmes, intégrations IDE/CI |
| Cloud et télémétrie | Aucun besoin pour le cœur ; local-first et aucune exfiltration par défaut |

## Cycle documentaire

Chaque fonctionnalité qui entre en réalisation doit produire ou mettre à jour :

1. son problème utilisateur et son résultat attendu ;
2. son contrat fonctionnel et ses cas limites ;
3. ses critères d'acceptation ;
4. ses implications de sécurité, de confidentialité et de compatibilité ;
5. sa place dans la roadmap et ses indicateurs d'adoption ;
6. la documentation utilisateur de la version qui la livre.

Une idée placée dans la roadmap n'est pas encore une promesse d'API. Les exemples de commandes des documents produit rendent l'expérience concrète ; ils restent exploratoires tant qu'une spécification de version ne les a pas figés.
