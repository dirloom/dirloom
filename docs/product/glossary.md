# Glossaire produit

> **Statut :** vocabulaire canonique<br>
> **Règle :** si une spécification de version donne une définition plus précise, elle prévaut pour cette version

## Artefact structurel

Représentation versionnée et déterministe d'une structure logicielle. Il contient la hiérarchie, les types, l'ordre canonique et les options pertinentes, mais exclut par défaut les métadonnées non déterministes ou sensibles.

## Observation

Donnée lue dans l'état courant d'un environnement et qui n'appartient pas nécessairement à l'artefact canonique : taille, date de modification, permissions, propriétaire, statut Git ou information runtime.

## Connaissance déclarée

Information fournie volontairement par le projet : Architecture Pack, contract, annotation, topologie voulue ou référence d'ADR. Elle exprime une intention ou un contexte, pas une observation automatique.

## Analyse dérivée

Résultat calculé à partir d'un ou plusieurs inputs : métrique, conformité, impact, dérive, recommandation ou sélection de contexte. Sa méthode et ses sources doivent être identifiables.

## Fingerprint

Empreinte calculée à partir d'une représentation canonique et d'un algorithme versionné. Les namespaces distinguent notamment structure, contenu, dépendances, architecture et contexte.

## Snapshot

Artefact persistant décrivant un état structurel précis et sa portée. `verify` compare une observation à cet état.

## Verify

Opération qui répond : « l'état observé correspond-il au snapshot de référence selon le mode demandé ? » Elle ne vérifie pas à elle seule qu'une architecture est bien conçue ou conforme à des règles générales.

## Diff structurel

Comparaison entre deux artefacts ou entre un artefact et une structure observée. Il décrit les ajouts, suppressions, changements et déplacements prouvés ou probables.

## Structural Version Control

Ensemble formé par fingerprint, snapshot, verify, diff, history et détection de déplacements. Il traite l'évolution de la structure comme une donnée de premier ordre ; il ne remplace pas Git pour le contenu.

## Structural Event Stream

Flux local d'événements structurels, notamment produit par `watch`, destiné au TUI, à Desktop, aux IDE et aux calculs incrémentaux. Il ne remplace pas une reconstruction complète lorsqu'un événement peut avoir été perdu.

## Query

Interrogation structurée de l'artefact ou, lorsqu'elle est explicitement activée, d'une couche d'observation. Le langage de requête est partagé entre les surfaces.

## Structural Metrics

Mesures descriptives telles que profondeur, fan-out, concentration, distribution ou croissance. Une métrique n'est pas automatiquement un jugement de qualité.

## Forme structurelle

Organisation relative attendue sous un nœud : dossiers, fichiers, cardinalités, motifs et niveaux. Deux modules peuvent avoir des noms différents tout en partageant la même forme.

## Structural Shape Diff

Comparaison de plusieurs structures contre une référence explicite — Shape Signature, Architecture Pack, contract, nœud choisi ou forme dominante — afin de révéler leurs divergences de forme. La commande cible est `dirloom shape compare`.

## Shape Signature

Représentation versionnée d'une forme structurelle réutilisable par `shape compare`, contracts, scaffold, conform, drift et Architecture Packs.

## Template

Unité matérialisable décrivant inputs, prompts, variantes, chemins, dossiers, fichiers, conditions, répétitions contrôlées et composition. Un template appartient généralement à un Architecture Pack.

## Architecture Pack

Unité versionnée qui réunit une convention d'architecture et ses moyens d'exécution : templates, variantes, contracts, règles de nommage, annotations par défaut, Shape Signatures, query presets, règles de contexte, métadonnées visuelles, migrations et skills d'agents.

## Pack manifest

Contrat machine-readable décrivant l'identité, la version, la compatibilité, la provenance, l'intégrité, les permissions et les hooks éventuels d'un Architecture Pack.

## Scaffold

Opération mutante qui matérialise un template ou un artefact dans une destination. Elle comprend un plan ou dry-run et applique le contrat des opérations `write controlled`.

## Architecture Contract

Ensemble de règles qui décrit les structures autorisées : éléments requis ou interdits, forme, nommage, profondeur, cardinalité et, plus tard, dépendances. `check` évalue une structure contre ces règles.

## Check

Opération qui répond : « la structure respecte-t-elle les contracts applicables ? » Contrairement à `verify`, plusieurs structures différentes peuvent toutes être conformes.

## Conformance

Comparaison d'une structure avec une cible déclarée, généralement issue d'un Architecture Pack. `conform` produit un plan reproductible pour corriger les éléments manquants, mal placés ou non conformes ; aucune mutation n'est appliquée sans `--apply` explicite.

## Drift

Évolution notable ou non conforme détectée sur une période à partir de snapshots, métriques, formes, contracts ou dépendances. Tout changement n'est pas une dérive fautive ; le diagnostic doit qualifier le signal.

## Persistent Structural Annotation

Métadonnée versionnée attachée à une zone structurelle : description, owner, statut, ADR, dépréciation ou échéance. Elle ne nécessite pas de modifier les fichiers métiers annotés.

## Reconciliation

Comparaison entre architecture voulue et architecture observée. Elle peut conclure `conforme`, `divergent`, `partiellement observable` ou `inconnu`.

## Context Compiler

Capacité qui sélectionne et organise un contexte pour un humain ou un agent sous un budget donné. La compilation peut être purement structurelle et déterministe ou orientée tâche avec des analyses supplémentaires déclarées.

## Context Receipt

Artefact qui enregistre ce qui a été inclus ou exclu d'un contexte, les raisons, le budget, les fingerprints et l'algorithme de sélection. Il permet de vérifier la fraîcheur du contexte.

## Progressive Context

Parcours dans lequel un consommateur reçoit d'abord une vue compacte, puis demande des expansions ciblées — branches, dépendances, symboles et fichiers — au lieu de charger tout le repository.

## Context Firewall

Couche de sécurité qui classe et filtre les sources destinées à un agent : confiance, contenu généré ou distant, dépendance externe, secret probable, instruction suspecte et configuration exécutable.

## Impact Lens

Vue du rayon d'impact potentiel d'un élément ou d'un changement, fondée sur les relations disponibles : structure, code, tests, configuration, runtime et infrastructure.

## Architecture Simulator

Moteur qui applique une transformation hypothétique à un modèle en mémoire et en évalue les conséquences sans modifier le filesystem.

## System Topology

Extension du graphe Dirloom au-delà du filesystem : packages, services, conteneurs, infrastructure et relations runtime, éventuellement sur plusieurs repositories.

## Workspace

Déclaration `dirloom.workspace.yaml` regroupant plusieurs repositories dans une représentation structurelle commune, puis dans un graphe de relations cross-repo.

## Architecture Twin

Représentation conjointe de l'architecture voulue — packs, contracts, annotations, ADR et topologie déclarée — et observée — filesystem, dépendances, conteneurs, configuration et runtime. Elle alimente drift, reconciliation, impact, simulation et conformance.

## Surface

Mode d'accès au produit : CLI, TUI, Desktop, CI, skill, MCP ou export. Une surface ne doit pas redéfinir le comportement métier du cœur.

## TUI

Interface utilisateur en terminal, explicitement lancée par `dirloom browse`. Elle explore l'artefact et ses analyses ; ce n'est pas un gestionnaire de fichiers généraliste.

## Dirloom Desktop

Application graphique future, locale par défaut, destinée à l'exploration, aux Architecture Packs, aux diffs, à la gouvernance, au contexte et à la simulation. Elle repose sur les mêmes services que la CLI.

## Skill d'agent

Workflow documenté qui permet à un agent de code d'utiliser Dirloom de manière sûre. Un skill adapte les commandes publiques sans élargir leurs permissions.

## MCP

Canal standardisé permettant à des agents de demander les capacités Dirloom. MCP transporte l'intelligence structurelle ; il ne constitue pas à lui seul la proposition de valeur.

## Adaptateur

Composant optionnel qui observe un domaine particulier — langage, dépendances, framework, infrastructure ou runtime — et traduit ses relations dans les modèles Dirloom avec une provenance explicite.

## Analyzer SDK

Contrat d'extension permettant à des analyzers spécialisés de produire des relations versionnées pour Dart/Flutter, TypeScript/JavaScript, Go puis d'autres domaines sans modifier le core.

## Pack SDK

Outillage et contrats permettant de créer, valider, tester et distribuer templates, contracts, queries, annotations, migrations, context policies et skills d'un Architecture Pack.

## Pack Registry

Service explicite de découverte et de distribution d'Architecture Packs. Il distingue `official`, `verified`, `community` et `private` et conserve compatibilité, provenance, intégrité, permissions et hooks visibles.

## Theme Registry

Service explicite de découverte et de distribution de thèmes purement présentationnels. L'installation est demandée par l'utilisateur et ne modifie jamais l'artefact canonique.

## Structural Artifact Format

Famille de formats versionnés issue du JSON schema v1 : artefact, snapshot, diff, Context Receipt et plan de mutation ou simulation. Les suffixes exacts restent expérimentaux jusqu'à stabilisation par l'usage.

## Architecture Pack FSD-like de référence

Premier pack officiel, identifié provisoirement par `reference-fsd`. Il encode une philosophie commune et des variantes Flutter, Next.js et Hono.js. Son nom public et sa forme exacte restent à documenter à partir des projets réels de référence.
