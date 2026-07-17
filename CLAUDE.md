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

**3. Convention de commit et de PR — Conventional Commits, en anglais**
Les messages de commit et les titres de PR doivent suivre [Conventional Commits](https://www.conventionalcommits.org/), toujours en anglais :

```
<type>(<scope>): <description>
```

- Types : `feat`, `fix`, `chore`, `docs`, `refactor`, `perf`, `test`, `ci`, `build`.
- `scope` optionnel entre parenthèses (ex: `feat(weekly-tasks): ...`).
- Description en anglais, à l'impératif, minuscule, sans point final.
- Le numéro de version va dans le corps du commit (ex: `Bumps version to 1.6.22.`), pas dans le sujet.

Ceci remplace l'ancienne convention `type: vX.Y.Z - description` en français visible plus tôt dans l'historique — ne pas la reproduire pour les nouveaux commits.

Le bump de `config.yaml` et l'entrée `CHANGELOG.md` restent inclus dans le même commit que le changement qu'ils documentent : code, version et changelog doivent toujours être commités ensemble, jamais le code seul.
