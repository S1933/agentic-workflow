# AI Development Workflow

Ce repository définit un workflow piloté par l'humain pour utiliser des agents IA pendant le développement logiciel.

Il sépare deux responsabilités :

- `expert` : analysis, interview, design, diagnosis, planning et review ;
- `worker` : implementation, exécution d'un plan, corrections ciblées et tests liés aux changements.

Le workflow prend en charge deux types de développement :

- nouvelle feature ;
- debug.

Le workflow déclaratif est défini dans `workflow.yaml`.

La documentation détaillée est disponible dans :

- `docs/philosophy.md`
- `docs/feature-workflow.md`
- `docs/debug-workflow.md`
- `docs/review-workflow.md`

Le workflow est volontairement indépendant :

- des providers IA ;
- des produits et coding agents IA ;
- des langages de programmation ;
- des frameworks ;
- des issue trackers ;
- de la structure des repositories ;
- des skills individuelles des agents.

Toutes les transitions du workflow restent contrôlées manuellement par le développeur.

## Setup recommandé

Le workflow est conçu pour être installé une seule fois au niveau utilisateur et partagé entre tous les projets locaux.

Il n'est pas nécessaire de copier `workflow.yaml` ni d'ajouter une configuration spécifique dans chaque repository applicatif.

### 1. Cloner le workflow dans le home

```bash
git clone git@github.com:S1933/agentic-workflow.git ~/.agentic-workflow
```

Le workflow devient alors disponible depuis :

```text
~/.agentic-workflow/workflow.yaml
```

### 2. Le référencer depuis les instructions globales de l'agent

Ajouter une instruction minimale dans le fichier global chargé par le coding agent.

Exemple :

```md
## Development workflow

For feature implementation and debugging tasks, read and follow:

~/.agentic-workflow/workflow.yaml

The workflow file is the source of truth for roles, steps, prerequisites,
human validations and transitions.

Load it only when a feature or debug workflow is needed.
```

Quelques emplacements courants :

- OpenCode : `~/.config/opencode/AGENTS.md`
- Codex : `$CODEX_HOME/AGENTS.md`, généralement `~/.codex/AGENTS.md`
- autres agents : utiliser leur mécanisme équivalent d'instructions globales

Le fichier global reste volontairement minimal : il indique uniquement à l'agent où trouver le workflow. La logique du workflow reste centralisée dans `workflow.yaml`.

### 3. Utilisation

Une fois le setup effectué, les agents peuvent utiliser le même workflow depuis n'importe quel repository local.

Exemples :

```text
Use the feature workflow for this implementation.
```

```text
Use the debug workflow to investigate this bug.
```

L'agent doit alors charger `~/.agentic-workflow/workflow.yaml`, sélectionner le workflow correspondant et respecter les rôles, préconditions, transitions manuelles et validations humaines définis dans le fichier.

### 4. Mise à jour

Le workflow reste versionné dans ce repository. Pour récupérer les changements :

```bash
cd ~/.agentic-workflow
git pull
```

Tous les coding agents utilisant cette installation locale bénéficient ensuite de la même version du workflow, sans modification des repositories applicatifs.
