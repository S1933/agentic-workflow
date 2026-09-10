# Analyse et plan du CLI Agentic

Le dépôt initial contient uniquement `workflow.yaml`, `runtime.yml` et la
documentation. Le CLI utilise les agents locaux comme sous-processus : il ne
réimplémente pas leur boucle d'exécution.

## Architecture retenue

- Go : binaire unique, indépendant du langage des projets ciblés.
- `internal/config` : YAML, schémas et validation.
- `internal/orchestrator` : progression déterministe et décisions humaines.
- `internal/state` : identité canonique du projet, stockage externe atomique.
- `internal/context` : instructions et contexte de la tâche courante.
- `internal/runtime` : adapters Claude Code, Codex et OpenCode.
- `internal/cli` et `cmd/agentic` : commandes et interactions terminal.

`workflow.yaml` décrit le travail ; `runtime.yml` associe rôles, capabilities,
runtimes et modèles. Les arguments des exécutables restent dans les adapters.

## Décisions de sémantique

Les conditions doivent avoir une origine explicite : saisie utilisateur,
attestation humaine, acceptation d'une étape, ou agrégation d'étapes terminées.
Un exit code nul n'est jamais une validation métier. Les résultats restent du
texte naturel ; leur acceptation et les transitions sont distinctes.

Les étapes optionnelles sautées ne sont pas terminées. Un checkpoint peut
explicitement accepter une dépendance sautée. L'interview peut précéder la
collecte des critères d'acceptation. Le design et les tâches nécessitent une
validation explicite. Les tickets sont fournis manuellement au MVP : aucune
intégration ni publication implicite dans un issue tracker.

Une invocation du CLI effectue au maximum une tâche agent. `resume` recueille
les décisions en attente ; `next` est inutile. Chaque tâche lance une nouvelle
session. Un fallback exige une décision humaine spécifique. Les reprises et
retours invalident les validations devenues obsolètes.

Un seul workflow est actif par projet. L'identité dérive du chemin canonique
de la racine du worktree Git, sinon du dossier courant. Le cwd de lancement
reste le cwd du runtime. Les configurations sont figées par exécution. L'état,
le contexte et les résultats résident sous XDG_STATE_HOME/agentic-workflow.
Reset archive l'état et ne modifie pas le projet applicatif.

## Plan et vérifications

1. Formaliser le contrat YAML : conditions, validations, étapes optionnelles.
2. Créer le CLI et le chargeur : rejeter versions, rôles et bindings invalides.
3. Ajouter l'état : isolation projets, verrou, sauvegarde atomique, archives.
4. Implémenter le moteur : gates bloquantes, sauts tracés, pas d'automatisme.
5. Assembler le contexte et les tâches : un objectif par invocation.
6. Implémenter les trois adapters : arguments, cwd, stdin, sorties et signaux.
7. Relier sélection humaine, resume, retry et fallback explicite.
8. Tester les parcours feature/debug avec faux runtimes et documenter l'usage.

Les tests ne doivent pas appeler de modèles réels. Les permissions et
l'authentification restent celles des outils installés, sans bypass ajouté.
