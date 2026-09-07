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
