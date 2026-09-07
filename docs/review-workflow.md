# Review Workflow

## Objectif

Le Review Workflow constitue un point d'entrée commun utilisé après l'implementation.

Il est volontairement indépendant du workflow ayant produit le changement.

Le reviewer n'a pas besoin de savoir si la branch contient :

- une nouvelle feature ;
- un bug fix.

Sa responsabilité est volontairement limitée.

## Déclenchement

La review est lancée manuellement par le développeur.

Le passage des tickets d'implementation au statut `In Review` ne déclenche pas automatiquement une review.

Cela permet de conserver un contrôle humain explicite sur le moment où les ressources de review sont utilisées.

## Entrée

La review travaille sur le diff final complet entre :

- la working branch ;
- la target branch.

Le commit history est ignoré.

Le reviewer évalue le code final obtenu plutôt que la suite de commits qui a permis de le produire.

## Scope

Le reviewer recherche :

- les bugs ;
- les regressions.

La review n'est pas un audit général de qualité du code.

Elle n'a pas pour objectif de produire des suggestions simplement parce que le code pourrait être :

- plus propre ;
- plus élégant ;
- plus maintenable ;
- architecturalement différent ;
- mieux formaté.

Un finding doit correspondre à un bug concret ou à un risque réel de regression.

## Tests

Les tests modifiés font partie du diff et peuvent être analysés comme n'importe quel autre code modifié.

Le reviewer n'exécute pas les tests.

L'exécution des tests relève de la responsabilité du `worker` ayant réalisé l'implementation.

Le reviewer n'effectue pas non plus d'audit détaillé de la test strategy ou de la qualité des tests.

## Profondeur de review

La review possède un seul point d'entrée logique.

Le développeur peut décider d'utiliser un ou plusieurs `expert` reviewers selon la difficulté du changement.

Le mécanisme exact reste volontairement ouvert.

Il pourra plus tard être :

- choisi explicitement par le développeur ;
- passé comme option dans le prompt de review ;
- partiellement décidé par un review orchestrator.

Ce comportement n'est pas automatisé dans la version 1 du workflow.

Lorsque plusieurs reviewers sont utilisés, il est préférable d'utiliser des providers différents lorsque cela est possible.

Cela permet d'obtenir des points de vue plus indépendants.

## Sévérité

Les findings sont classés selon trois niveaux.

### Blocking

L'implementation ne peut pas être acceptée tant que le problème n'est pas corrigé.

Une task de correction doit être créée.

### Major

Le finding représente un bug ou une regression significative.

Une task de correction doit être créée.

### Minor

Le finding est réel mais son impact est limité.

Il est uniquement reporté.

Il ne devient pas automatiquement une task de correction.

## Priorisation

Le travail de correction est priorisé ainsi :

1. findings `blocking` ;
2. findings `major`.

Les findings `minor` restent visibles dans le rapport mais ne sont pas automatiquement inclus dans la boucle de correction.

## Tasks de correction

Les findings `blocking` et `major` doivent être exprimés sous forme de tasks d'implementation directement exploitables par un `worker`.

Le reviewer doit rendre chaque correction suffisamment ciblée pour permettre une exécution indépendante.

Plusieurs corrections ne sont pas envoyées au `worker` dans une seule grosse demande d'implementation.

Elles sont traitées une par une.

Chaque correction démarre dans une nouvelle session.

## Re-review

Après une correction, la review cible uniquement la zone corrigée.

Une review complète de toute la branch n'est pas automatiquement relancée.

L'`expert` peut inspecter le code adjacent lorsque cela est nécessaire pour comprendre ou valider la correction.

Le reviewer ne doit pas utiliser cette étape pour relancer une review globale sur du code sans rapport avec la correction.

## Autorité humaine

Le résultat de la review reste consultatif.

Une review IA réussie n'autorise pas automatiquement un merge.

Une review IA en échec ne déclenche pas automatiquement une implementation.

Le développeur contrôle chaque boucle :

`Review → décision → correction → décision → re-review`

Le workflow se termine uniquement après validation humaine.
