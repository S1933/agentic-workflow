# Debug Workflow

## Pré-requis

Un workflow de debug peut démarrer avec moins d'informations qu'un Feature Workflow.

Une spec complète n'est pas nécessaire.

Un bug report contenant suffisamment d'informations pour commencer l'investigation est suffisant.

## Reproduction

Le premier objectif est de reproduire le bug.

Aucune modification du code de production ne doit être effectuée avant que le problème ait été reproduit.

La reproduction fournit un point de départ fiable pour l'investigation et évite les fixes spéculatifs.

## Analysis

La root cause analysis est réalisée par un `expert`.

L'`expert` ne doit pas proposer immédiatement un patch.

Sa première responsabilité est de comprendre pourquoi le problème se produit.

L'analysis doit produire une root cause crédible, appuyée par des éléments concrets.

Ces éléments peuvent notamment inclure :

- le comportement observé à l'exécution ;
- les logs ;
- les tests en échec ;
- les transitions d'état ;
- des données incorrectes ;
- les code paths ;
- le comportement d'une dépendance ;
- la configuration ;
- les résultats de reproduction.

## Confidence

L'`expert` doit avoir un niveau de confiance suffisant dans la root cause avant que l'implementation ne commence.

Si la confidence est faible, l'investigation continue.

Le workflow ne doit pas lancer une implementation expérimentale simplement parce qu'une hypothèse semble plausible.

L'`expert` doit collecter davantage d'éléments, tester ses hypothèses et éliminer les explications alternatives jusqu'à obtenir une confidence suffisante.

Cette approche est volontairement différente d'un debug orienté implementation en premier.

## Plan de correction

Une fois la root cause suffisamment comprise, l'`expert` produit un plan de correction détaillé.

Le plan décrit ce qui doit être modifié.

Le `worker` est ensuite responsable de l'exécution de ce plan.

Le `worker` ne doit pas avoir à redécouvrir la stratégie de debug pendant l'implementation.

## Implementation

Le `worker` exécute le plan de correction.

Lorsque le plan contient plusieurs tasks d'implementation, elles sont exécutées une par une.

Chaque task démarre dans une nouvelle session.

La session d'implementation reçoit le contexte utile disponible, notamment :

- le bug report ;
- le contexte du ticket lorsqu'il existe ;
- la root cause ;
- le plan de correction ;
- le contexte technique pertinent.

Le `worker` exécute uniquement les tests liés à ses changements.

## Fallback d'implementation vers un expert

Comme pour le Feature Workflow, l'`expert` n'est pas l'agent d'implementation par défaut.

Si le `worker` n'arrive pas à implémenter correctement le fix, le développeur peut décider explicitement de confier l'implementation à un `expert`.

Il s'agit d'une escalade manuelle.

L'`expert` démarre une nouvelle session propre dédiée à l'implementation.

## Regression coverage

Une fois le fix implémenté, un regression test est ajouté afin de couvrir le bug corrigé.

Le regression test est ajouté après le fix plutôt que d'être un pré-requis à son implementation.

Les tests concernés doivent passer avant la review.

## Review

Le Debug Workflow utilise le même point d'entrée de review que le Feature Workflow.

Le processus de review n'a pas besoin de savoir si le changement provient d'une feature ou d'un bug fix.

Il reçoit le diff final de la branch par rapport à la target branch.

Sa responsabilité est de rechercher :

- les bugs ;
- les regressions.

Il ne relance pas la root cause analysis.

Il ne revalide pas le ticket.

Il n'exécute pas les tests.

## Corrections

Les findings `blocking` et `major` deviennent des tasks de correction.

Les findings `minor` sont uniquement reportés.

Les tasks de correction sont exécutées une par une dans de nouvelles sessions `worker`.

La zone corrigée est ensuite review par un `expert`.

Une review complète de la branch n'est pas automatiquement relancée.

## Validation finale

Le développeur effectue la validation finale.

Le Debug Workflow est terminé uniquement lorsque le développeur considère le fix comme acceptable.
