# Taskflow — instructions pour Claude

Ce repo est un add-on Home Assistant (racine de l'add-on : `taskflow-backend/`, à côté de `config.yaml`).

## Règles obligatoires à chaque changement

**1. Bump de version obligatoire**
Tout changement au code, à la config, ou au comportement de l'add-on doit s'accompagner d'un incrément du champ `version` dans `taskflow-backend/config.yaml`. Home Assistant Supervisor détecte les mises à jour disponibles en comparant ce champ, et la CI (`.github/workflows/build.yml`) tague l'image Docker publiée avec cette même valeur — sans bump, l'utilisateur ne verra jamais la mise à jour proposée dans Home Assistant.

**2. Changelog Home Assistant obligatoire**
Chaque bump de version doit ajouter une entrée en tête de `taskflow-backend/CHANGELOG.md` (même dossier que `config.yaml` — c'est l'emplacement que Supervisor lit pour afficher le changelog dans l'interface d'add-on de Home Assistant). Format à respecter, le plus récent en premier :

```markdown
## X.Y.Z

- Description concise et orientée utilisateur du changement.
```

Une ligne par changement notable ; plusieurs lignes si le bump groupe plusieurs changements.

**3. Convention de commit**
Suivre le style déjà en place dans l'historique : `type: vX.Y.Z - description courte` (`feat:`, `fix:`, ou `chore:` selon la nature du changement), avec le bump de `config.yaml` et l'entrée `CHANGELOG.md` inclus dans le même commit que le changement qu'ils documentent.

Ces trois éléments (code, version, changelog) doivent toujours être commités ensemble — jamais le code seul.
