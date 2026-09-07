# Feature Workflow

## Pré-requis

L'analyse d'une feature ne commence que lorsque deux éléments sont disponibles :

- une demande fonctionnelle ;
- des acceptance criteria.

Ce sont des pré-requis.

Le workflow technique ne doit pas tenter d'inventer des exigences produit manquantes.

## Interview

Pour les features simples, le workflow peut passer directement à l'analysis.

Pour les features complexes, ambiguës ou nécessitant beaucoup de contexte, une phase d'interview est ajoutée.

L'interview utilise un `expert`.

Son objectif est de permettre au développeur de fournir des informations qui ne sont pas forcément disponibles dans le ticket ou le codebase.

L'interview peut notamment clarifier :

- les contraintes métier ;
- l'historique technique ;
- les contraintes d'architecture ;
- le comportement attendu ;
- les edge cases non évidents ;
- les décisions précédentes ;
- les limites du changement demandé.

L'interview a lieu avant la création du design final.

## Analysis

L'`expert` analyse le repository existant.

L'analysis ne doit pas se limiter aux fichiers qui semblent nécessiter une modification.

L'`expert` doit inspecter le code environnant et les patterns voisins afin de comprendre comment des problèmes similaires sont déjà traités.

Les zones pertinentes peuvent inclure :

- les implementations existantes ;
- l'architecture ;
- les conventions locales ;
- les composants liés ;
- les data flows ;
- les dépendances ;
- les tests ;
- les integration boundaries.

L'objectif est de concevoir une solution qui s'intègre au codebase existant plutôt que de concevoir une solution isolée.

## Design

L'`expert` propose le design technique.

Le développeur et l'`expert` échangent autour du design proposé.

Le design n'est considéré comme final qu'après validation explicite du développeur.

La création des tasks ne commence pas avant cette validation.

## Spec

Après validation du design, une spec formelle est produite.

Une skill adaptée peut être utilisée pour la créer.

Le workflow ne définit pas quelle skill doit être utilisée.

La spec est ajoutée au parent ticket.

Une fois ajoutée, elle devient la source de vérité pour l'implementation.

Les futures sessions d'implementation doivent consommer la spec via le contexte du ticket.

Elles ne doivent pas dépendre de la conversation initiale de design.

## Découpage en tasks

L'`expert` propose un premier découpage en tasks à partir de la spec validée.

Le développeur et l'`expert` ajustent ensemble ce découpage.

Le découpage doit être validé par le développeur avant de commencer l'implementation.

Les tasks doivent être suffisamment petites pour qu'un agent d'implementation puisse se concentrer sur un seul objectif à la fois.

## Implementation

L'implementation est réalisée par un `worker` par défaut.

Les tasks sont exécutées une par une.

Chaque task démarre dans une nouvelle session agent.

La session reçoit un contexte riche comprenant généralement :

- le parent ticket ;
- la child task ;
- la spec ;
- les acceptance criteria ;
- les informations techniques disponibles.

L'objectif n'est pas de réduire le contexte utile.

L'objectif est d'éviter de conserver un historique de conversation sans rapport direct avec la task courante.

## Tasks backend

Lorsqu'une feature contient du travail backend et frontend, les tasks backend sont implémentées en premier.

Chaque task backend est traitée indépendamment.

Le `worker` exécute uniquement les tests liés aux changements qu'il a réalisés.

Il n'est pas nécessaire d'exécuter toute la test suite après chaque task.

À la fin d'une task, le rapport final standard produit par le modèle suffit.

Aucun format de reporting custom n'est nécessaire.

La task peut ensuite passer au statut `In Review`.

Le passage au statut `In Review` ne déclenche pas automatiquement une review IA.

La review est lancée manuellement par le développeur.

## Checkpoint backend

Le travail frontend ne doit pas commencer tant que le checkpoint backend n'est pas validé.

Ce checkpoint nécessite :

- toutes les tasks backend terminées ;
- les tests automatisés liés aux changements toujours au vert ;
- une première validation manuelle par le développeur.

Cela crée une séparation volontaire entre l'implementation backend et frontend.

## Tasks frontend

Les tasks frontend suivent les mêmes règles d'implementation que les tasks backend :

- `worker` par défaut ;
- une task à la fois ;
- nouvelle session pour chaque task ;
- contexte riche récupéré depuis les tickets ;
- tests liés aux changements uniquement.

L'agent frontend n'hérite pas de l'historique des sessions d'implementation backend.

La source persistante de contexte reste les tickets et la spec.

## Fallback d'implementation vers un expert

Le `worker` reste l'agent d'implementation par défaut.

Si le `worker` n'arrive pas à implémenter correctement une task, le développeur peut décider d'escalader la task vers un `expert`.

Cette escalade n'est jamais automatique.

Les déclencheurs possibles incluent :

- une implementation incorrecte ;
- une mauvaise compréhension répétée ;
- un échec lors de la validation manuelle ;
- une complexité inattendue ;
- un échec d'integration ou du comportement end-to-end.

Le développeur décide explicitement si l'`expert` doit reprendre l'implementation.

Dans ce cas, l'`expert` démarre une nouvelle session propre.

L'objectif reste d'utiliser les `expert` principalement pour le raisonnement plutôt que pour l'implementation courante.

## Validation end-to-end

Une fois l'implementation terminée, le développeur effectue une validation manuelle globale de la feature.

Pour une feature qui traverse plusieurs couches, cette validation comprend un test end-to-end du comportement complet.

La phase de review ne commence pas tant que cette validation n'est pas réussie.

Si le test end-to-end échoue, le développeur peut décider de demander à l'`expert` qui a analysé la task de reprendre le travail.

Cette décision reste humaine.

L'`expert` ne devient pas automatiquement l'agent d'implementation.

## Review

La review est lancée manuellement par le développeur.

Elle est généralement réalisée au niveau de la branch, car celle-ci contient l'ensemble du développement lié au parent ticket.

Le reviewer compare le diff complet de la branch avec la target branch.

Le reviewer n'analyse pas le commit history.

La review se concentre uniquement sur :

- les bugs ;
- les regressions.

La review n'a pas pour objectif d'être un audit général de qualité du code.

Elle ne doit pas consacrer de temps à :

- la code style ;
- les préférences d'architecture ;
- les améliorations de maintenabilité ;
- les performances sauf si elles provoquent un bug ;
- la sécurité sauf si elle provoque un bug ou une regression observable ;
- la test strategy.

Les tests font partie du diff et peuvent être analysés comme n'importe quel code modifié, mais le reviewer n'évalue pas la qualité globale de la stratégie de tests.

Le reviewer n'exécute pas les tests.

L'exécution des tests liés aux changements est de la responsabilité de l'agent d'implementation.

La review ne revalide pas non plus la spec fonctionnelle ou les acceptance criteria.

Son entrée est le diff de l'implementation.

## Findings

Les findings de review utilisent trois niveaux.

### Blocking

Un finding `blocking` empêche l'implementation d'être considérée comme valide.

Une task de correction est créée.

### Major

Un finding `major` correspond à un bug ou une regression importante.

Une task de correction est créée.

### Minor

Un finding `minor` est reporté mais ne crée pas automatiquement de task de correction.

Les findings `blocking` et `major` sont prioritaires par rapport aux findings `minor`.

## Corrections

Les findings `blocking` et `major` sont transformés en tasks directement exploitables par un `worker`.

Les tasks de correction sont exécutées :

- une par une ;
- par un `worker` par défaut ;
- dans de nouvelles sessions.

Le `worker` exécute les tests liés à chaque correction.

Un `expert` peut être utilisé comme fallback d'implementation uniquement après une décision humaine explicite.

## Correction review

Une correction ne déclenche pas automatiquement une nouvelle review complète de toute la branch.

L'`expert` review uniquement la zone corrigée.

Le reviewer peut élargir le scope lorsque la correction nécessite d'inspecter du code directement lié ou des dépendances.

Cela ne signifie pas qu'il doit relancer une recherche globale de regressions.

L'objectif est de valider la correction sans repayer systématiquement le coût d'une review complète de la branch.

## Validation finale

La validation humaine est toujours obligatoire.

Même lorsqu'aucun finding `blocking` ou `major` ne subsiste, la review IA ne prend pas la décision finale de merge.

Le développeur reste l'autorité finale.
