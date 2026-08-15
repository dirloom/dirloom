# Documentation produit de Dirloom

> **Statut :** corpus aligné sur la roadmap stratégique votée<br>
> **Dernière consolidation :** 11 août 2026<br>
> **Périmètre :** évolution post-v0.1 de Dirloom

Dirloom transforme la structure logicielle en un artefact que l'on peut capturer, comparer, interroger, gouverner, matérialiser et fournir aux outils. La version `v0.1.1` livre déjà le socle : une représentation locale, déterministe, filtrable, portable et exploitable par des machines. Les documents de ce dossier décrivent le produit que ce socle permet de construire.

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
| Couleurs et icônes | Système thématique sémantique puissant, sans contaminer l'artefact canonique |
| Configuration | Socle livré : `.dirloom.yaml`, configuration utilisateur, presets intégrés et résolution inspectable |
| TUI | Jalon `v0.3` : surface d'exploration de l'artefact, pas gestionnaire de fichiers |
| Desktop | Alpha/beta en `v1.x`, produit stable et intelligence multi-repositories en `v2.x` |
| Agents de code | Jalon `v0.9` : Context Compiler, receipts, MCP, skills et Context Firewall |
| Exports graphiques | Mermaid, Graphviz et D2 dans le jalon d'identité visuelle `v0.2` |
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
