# Philosophie

## Objectif

L'objectif de ce workflow n'est pas d'automatiser le développement logiciel de bout en bout.

L'objectif est d'affecter le bon type de modèle IA au bon type de travail, tout en laissant au développeur le contrôle de chaque transition importante.

Le workflow sépare les tâches qui demandent beaucoup de raisonnement des tâches principalement orientées exécution.

## Rôles `expert` et `worker`

Deux rôles sont utilisés.

### Expert

L'`expert` est responsable de :

- l'analysis ;
- l'interview ;
- le design ;
- le diagnosis ;
- le planning ;
- la review.

L'`expert` doit principalement raisonner sur le problème.

Il doit comprendre le système, identifier les contraintes, prendre les décisions techniques et préparer le travail à exécuter.

L'`expert` n'est pas l'agent d'implementation par défaut.

Un `expert` peut implémenter du code uniquement lorsque le développeur décide explicitement de l'utiliser comme fallback.

Les raisons peuvent notamment être :

- le `worker` ne comprend pas correctement la task ;
- le `worker` produit une implementation incorrecte ;
- un test manuel révèle un problème non trivial ;
- la task est plus complexe que prévu ;
- le développeur estime qu'une capacité de raisonnement supérieure est nécessaire.

Lorsqu'un `expert` prend en charge l'implementation, il démarre dans une nouvelle session.

### Worker

Le `worker` est responsable de :

- l'implementation ;
- l'exécution d'un plan existant ;
- les corrections ciblées ;
- les tests liés à ses changements.

Le `worker` doit normalement recevoir un problème bien défini ou un plan d'implementation.

Le `worker` est l'agent d'implementation par défaut.

## Utiliser l'intelligence pour les décisions

Le workflow utilise volontairement les modèles disposant des meilleures capacités de raisonnement pour les décisions, et des modèles plus rapides et moins coûteux pour l'exécution.

Le schéma général est :

`Expert → Worker → Expert`

Le premier `expert` comprend le problème et conçoit la solution.

Le `worker` l'implémente.

Le dernier `expert` effectue la review du code obtenu.

Cette approche permet d'utiliser les modèles les plus coûteux là où leur intelligence apporte réellement de la valeur, sans les utiliser pour chaque modification de code.

## Workflow contrôlé par l'humain

Le workflow n'est pas autonome.

Chaque transition est contrôlée manuellement par le développeur.

Il n'existe pas de boucle automatique du type :

`Worker → Review → Worker → Review jusqu'au succès`

Le système peut identifier l'étape suivante recommandée, mais le développeur décide quand démarrer la phase suivante.

Cela concerne notamment :

- le démarrage de l'implementation ;
- le passage à la task suivante ;
- l'escalade d'un `worker` vers un `expert` ;
- le passage du backend au frontend ;
- le lancement d'une review ;
- le démarrage des tasks de correction ;
- l'acceptation finale de l'implementation.

L'IA assiste le workflow.

Elle ne le contrôle pas.

## Une task à la fois

Les tasks d'implementation sont exécutées une par une.

Un `worker` ne reçoit pas plusieurs tasks d'implementation à réaliser dans une seule session.

Cela permet de maintenir le modèle concentré sur un seul objectif et de limiter le contexte non pertinent.

La même règle s'applique aux corrections issues de review.

Si une review identifie plusieurs problèmes `blocking` ou `major`, chaque correction est exécutée séparément.

## Une nouvelle session par task

Chaque task d'implementation démarre dans une nouvelle session.

Le contexte de conversation de la task précédente n'est pas réutilisé.

À la place, le contexte utile est réinjecté au démarrage de la nouvelle session.

Le principe est simple :

**Maximum de contexte utile, minimum de contexte historique.**

L'agent d'implementation peut recevoir :

- le parent ticket ;
- la child task ;
- la spec ;
- les acceptance criteria ;
- le contexte technique ;
- toute information supplémentaire disponible via l'issue tracker.

Cela permet au `worker` de comprendre l'objectif global sans conserver le raisonnement et le bruit des tasks précédentes.

## Tickets comme contexte persistant

Un parent ticket peut contenir la demande fonctionnelle, les acceptance criteria et la spec.

Les tasks de développement peuvent être représentées par des child tickets.

Une CLI ou une autre intégration peut récupérer le contenu des tickets via une API et l'injecter dans une session agent.

L'issue tracker joue donc le rôle de contexte persistant entre des sessions IA indépendantes.

## Spec comme source de vérité

Pour le développement d'une feature, une spec formelle est créée après validation du design.

La spec est ajoutée au parent ticket.

Une fois publiée, elle devient la source de vérité pour l'implementation.

Les sessions d'implementation consomment la spec via le contexte du ticket plutôt que de dépendre d'une conversation IA précédente.

## Skills

Le workflow ne mappe pas explicitement les phases du workflow vers des skills précises.

Les agents doivent sélectionner naturellement les skills disponibles adaptées au travail en cours.

Par exemple, un agent peut sélectionner de lui-même des skills liées à :

- l'interview ;
- l'analyse du codebase ;
- la rédaction d'une spec ;
- la création de tasks ;
- l'exécution d'un plan ;
- le debug ;
- la vérification ;
- la code review.

Le workflow doit rester indépendant du contenu exact du skill registry.

Les skills décrivent des capacités réutilisables.

Ce repository décrit la manière dont le travail progresse.

## Indépendance vis-à-vis des providers

Le workflow ne définit aucun modèle IA concret.

Il définit uniquement des responsabilités à travers les rôles `expert` et `worker`.

Le modèle réel affecté à un rôle peut évoluer à tout moment sans modifier le workflow.

Par exemple, le modèle utilisé derrière le rôle `expert` peut être remplacé lorsqu'un meilleur modèle de raisonnement devient disponible.

La même logique s'applique au rôle `worker`.

## Plusieurs providers

Une review peut utiliser plusieurs `expert` lorsque davantage de confiance est nécessaire.

Le nombre exact de reviewers n'est volontairement pas figé dans le workflow.

Le développeur peut demander des reviewers supplémentaires selon :

- la complexité du ticket ;
- la taille du diff ;
- le niveau de risque ;
- le niveau d'incertitude.

Lorsque plusieurs reviewers sont utilisés, il est préférable d'utiliser des providers différents afin d'obtenir une plus grande diversité de points de vue.

Il s'agit d'une stratégie de review, pas d'une règle d'orchestration automatique.

## Validation

Une review IA ne remplace jamais la validation humaine.

Le développeur reste responsable de l'acceptation de l'implementation.

La validation humaine intervient tout au long du workflow et reste obligatoire avant de considérer le travail comme terminé.
