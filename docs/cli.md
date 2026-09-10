# Guide du CLI

## Contrat d'exécution

Le moteur charge `workflow.yaml` et `runtime.yml`, valide leurs schémas,
sélectionne le workflow et conserve une copie des deux fichiers pour le run.
Il résout le rôle via la capability, puis propose les runtimes dans un ordre
stable. Avec plusieurs candidats, le choix humain est obligatoire. Un candidat
unique est utilisé directement. Aucun échec ne déclenche un autre runtime.

La sortie du sous-processus est affichée et sauvegardée avec ses diagnostics,
son modèle, son code de sortie et le prompt. Un code zéro laisse le résultat
en attente d'acceptation. Les rapports restent du texte naturel.

## Actions de resume

| Action | Effet |
| --- | --- |
| `run` | Préparer et autoriser une tâche agent. |
| `accept` | Accepter le résultat courant ; aucune nouvelle tâche lancée. |
| `next` | Autoriser le passage d'une étape acceptée/sautée à la suivante. |
| `validate` | Attester un checkpoint manuel, dont E2E et validation finale. |
| `skip` | Sauter une étape optionnelle non applicable, sans la marquer terminée. |
| `retry` | Nouvelle invocation avec les instructions correctives fournies. |
| `fallback` | Autoriser explicitement l'expert à reprendre la tâche worker. |
| `rewind` | Revenir à une étape antérieure, invalider son résultat et la suite. |
| `edit` | Remplacer une entrée initiale et invalider tout le workflow. |
| `pause` | Sauvegarder et arrêter. |

Seules les actions applicables sont proposées. Les confirmations demandent
`yes` ou `no` ; Entrée ne vaut jamais approbation. Le CLI ne lit pas une file
d'approbations sur un pipe. Un EOF interrompt le dialogue en conservant l'état.

Pour une interview, `retry` permet d'envoyer les réponses dans une nouvelle
invocation. Le contexte explicite remplace la continuation d'une session native.

Pour une étape à plusieurs tâches, `accept` valide la tâche courante ; un
nouveau `resume` est nécessaire pour autoriser la suivante. La liste est saisie
avec un nombre de tâches, puis un texte ou fichier par tâche. Chaque liste
doit être approuvée. Pour les corrections, saisir les blocking avant les major.

Les copies des tickets, de la spec et du plan sont fournies explicitement.
Les rapports acceptés des étapes de raisonnement sont aussi injectés ; les
rapports des tâches worker voisines ne constituent pas un historique de session.
Un rewind conserve les anciens prompts/résultats mais efface les listes de
tâches et le contexte dérivé pour demander leur nouvelle validation.

## Conditions

Les conditions déclarées ont quatre origines :

- `input` : contenu fourni par l'utilisateur ;
- `human` : attestation explicite avec une preuve ou explication ; `after`
  impose qu'une étape préalable soit acceptée ;
- `completion` : acceptation de l'étape désignée par `step` ;
- `aggregate` : toutes les étapes de `steps` sont acceptées.

`allow_skipped` permet explicitement à une dépendance de ne pas s'appliquer.
Il est utilisé pour le checkpoint backend et l'agrégation backend/frontend.
Une étape optionnelle avec `when` ne peut pas être sautée quand cette condition
est vraie. Une condition inconnue, une dépendance future/cyclique, une clé YAML
inconnue ou une version non supportée provoquent une erreur.

L'interview est accessible avant de compléter les prérequis du workflow ;
l'analyse feature exige la demande fonctionnelle et les critères d'acceptation.
Les observations mutables (tests au vert, résolution des findings, etc.)
portent `invalidate_on_execution: true` : une nouvelle invocation agent les
invalide et elles seront redemandées si une gate en dépend. Les faits
historiques, comme une publication de spec ou les findings de la review
acceptée, restent disponibles. Les acceptations d'étapes sont distinctes de
ces attestations. Un retour à une étape invalide toutes les attestations et
les acceptations à partir de cette étape.

Le CLI applique ces règles à ses propres transitions. Le comportement interne
du coding agent dépend de ses instructions et de ses permissions ; le moteur
ne prétend pas vérifier sémantiquement le code ou le rapport généré.

## État et reprise

```text
~/.local/state/agentic-workflow/projects/<sha256-du-chemin-canonique>/
  lock
  state.json
  runs/<run-id>/
    workflow.yaml
    runtime.yml
    attempts/<step-attempt>/
      prompt.txt
      stdout.txt
      stderr.txt
    archived-state.json  (après reset)
```

Le chemin canonique de la racine du worktree Git identifie le projet ; hors
Git, c'est le dossier de lancement. Les sous-dossiers d'un même worktree
partagent l'état, mais le cwd initial est conservé pour les runtimes. Deux
clones ou worktrees ont des états distincts. Un déplacement du projet change
son identité ; il n'y a pas de migration automatique.

Les écritures d'état utilisent un fichier temporaire suivi d'un renommage
atomique. Les répertoires créés sont privés (`0700`) et les fichiers `0600`.
Un verrou système empêche deux commandes mutantes simultanées pour un projet.
`status` lit la dernière sauvegarde sans prendre le verrou.

Ctrl-C interrompt le groupe de processus du runtime et conserve une tentative
interrompue. Après un arrêt brutal du processus Agentic, un état `running`
est traité comme interrompu à la reprise. Inspecter le projet et vérifier
qu'aucun ancien runtime ne tourne encore avant de relancer une tentative.
Il n'y a ni rollback automatique du code ni nouvelle tentative automatique.

## Adapters et limites du MVP

Les adapters utilisent `claude -p`, `codex exec` et `opencode run`.
Claude et Codex reçoivent le prompt sur stdin ; OpenCode reçoit un argument
positionnel unique, séparé des options. Ce dernier reste soumis à la limite
de taille des arguments du système : un contexte excessif produit une erreur
d'exécution, sans troncature silencieuse.

Les arguments ont été vérifiés dans les aides des CLI installés. La
[documentation officielle OpenCode](https://opencode.ai/docs/cli/) décrit
également son mode `run`. Les tests de contrat utilisent de faux exécutables ;
ils ne valident pas l'accès réel aux modèles de l'utilisateur. Les versions
des outils doivent supporter les options utilisées, dont `--ephemeral` pour
Codex et `--no-session-persistence` pour Claude.

Le CLI conserve les permissions natives. Un runtime qui ne dispose pas des
autorisations nécessaires peut refuser un outil ou attendre sa propre
validation ; configurer ses permissions en dehors d'Agentic et interrompre
si nécessaire. Aucun mode `bypass`, `--auto` ou équivalent n'est ajouté.

La review demande une branche/référence cible et un worktree propre. Elle
utilise le diff complet entre le merge-base de cette cible et HEAD ; elle
ne lit pas l'historique des commits et ne lance pas de tests. Les corrections
font l'objet d'une review ciblée. Les branches contenant des changements non
commités doivent être préparées par l'utilisateur avant la review.

Les commandes ne créent pas de daemon, serveur ou MCP. Les intégrations de
tickets, les workflows simultanés dans un même projet, Windows et la sélection
de plusieurs reviewers dans une seule étape restent hors du MVP.
