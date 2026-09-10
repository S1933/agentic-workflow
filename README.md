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

## CLI Agentic

`agentic` orchestre Claude Code, Codex et OpenCode via leurs CLI locaux.
Il charge le workflow global et les bindings de runtimes, utilise le dossier
courant comme cible et enregistre son état hors du repository applicatif.

Prérequis : Go 1.23+, macOS ou Linux, Git pour la review, et les runtimes
choisis installés, authentifiés et configurés pour les opérations souhaitées.

```bash
git clone git@github.com:S1933/agentic-workflow.git ~/.agentic-workflow
cd ~/.agentic-workflow
go build -o ~/.local/bin/agentic ./cmd/agentic
```

Créer `~/.local/bin` si nécessaire et l'ajouter au `PATH`. Depuis un projet :

```bash
agentic feature
agentic debug
agentic status
agentic resume
agentic reset
```

`feature` et `debug` démarrent un workflow ; il ne peut y en avoir qu'un actif
par projet. `resume` propose l'action suivante et exige une décision explicite.
Chaque invocation exécute au maximum une tâche agent. `accept` valide le
résultat sans lancer la suite ; `next`, dans le dialogue de `resume`, autorise
le passage à l'étape suivante. Il n'existe pas de commande `agentic next`.

`reset` demande confirmation puis archive l'exécution. Il ne restaure ni ne
supprime les modifications de code. `status` est utilisable sans terminal ;
les autres commandes requièrent une saisie humaine interactive.

La configuration globale est dans `~/.agentic-workflow/` :

```bash
agentic feature --config-dir /chemin/agentic-workflow
```

Les modèles de `runtime.yml` sont transmis tels quels, sauf `default`, qui
signifie « utiliser le modèle configuré dans ce runtime ». Adapter ces valeurs
à ses outils et à ses accès ; le CLI ne remplace jamais un modèle ou un runtime
indisponible. Les permissions et l'authentification des runtimes restent sous
leur contrôle ; aucun flag de contournement des permissions n'est ajouté.

L'état est conservé sous `${XDG_STATE_HOME:-~/.local/state}/agentic-workflow/`.
`--state-dir /chemin/externe` permet de remplacer cette racine. Un chemin
interne au projet est refusé. Chaque run conserve les configurations utilisées,
les prompts, les résultats et les décisions humaines. Une mise à jour des YAML
n'affecte pas les runs en cours.

Les textes demandés peuvent être saisis directement ou chargés avec `@/path/file.md`.
Le MVP utilise le contenu des tickets fourni par l'utilisateur ; il ne publie
pas de spec et ne modifie pas de tickets. La publication de la spec est une
attestation humaine obligatoire. Les tâches proposées sont saisies et validées
une par une, puis exécutées dans des sessions indépendantes.

Voir [le guide du CLI](docs/cli.md) pour les reprises, checkpoints, conditions
et limites, et [l'analyse et le plan](docs/cli-orchestration-analysis.md).

```bash
go test ./...
go test -race ./...
go vet ./...
```

Les tests utilisent des runtimes simulés et ne consomment pas de tokens IA.

## Utilisation sans le CLI

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
