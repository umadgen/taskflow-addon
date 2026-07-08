# Synchro bidirectionnelle Taskflow ↔ Google Tasks

## Contexte et objectif

Permettre aux tâches Taskflow ayant une date d'échéance d'apparaître dans Google Tasks (visible depuis l'onglet "Tasks" de Google Calendar, mobile ou web), et inversement : ce qui se passe côté Google Tasks (complétion, nouvelles tâches) doit remonter dans Taskflow. Le but est de pouvoir gérer/cocher ses tâches depuis son téléphone via Google, sans dépendre de l'app Taskflow elle-même.

## Décisions de cadrage

- **API ciblée : Google Tasks**, pas Google Calendar (events). Google Tasks a une notion native de complétion et de récurrence-utilisateur, ce qui colle au modèle de Taskflow ; Google Calendar n'a pas de notion de "fait/pas fait".
- **Un seul compte Google pour tout le foyer**, lié une fois dans les réglages Taskflow. Une seule liste Google Tasks dédiée, nommée **"Taskflow"**, créée automatiquement au premier lien de compte.
- **OAuth direct dans l'addon** (pas via l'intégration Google Calendar de Home Assistant), avec un **Device Authorization Grant** (flow "TV et entrées limitées") : pas besoin d'URL de callback joignable depuis l'extérieur, fonctionne même purement derrière l'ingress HA.
- **Tâches concernées** : toute tâche avec une date d'échéance (`Due` non vide), ponctuelle ou récurrente. Les tâches sans échéance ne sont pas synchronisées (pas d'équivalent naturel côté Google Tasks).
- **Récurrence** : Google Tasks n'expose pas de règle de récurrence pilotable par API (fonctionnalité UI récente, non documentée dans l'API publique). Une tâche récurrente Taskflow est donc représentée par **une Google Task à la fois** : quand elle est complétée, la Google Task est marquée `completed`, et une **nouvelle** Google Task est créée pour la prochaine échéance (miroir exact du comportement interne `advanceDue()` de Taskflow).
- **Autorité des champs (titre, date, notes) : Taskflow gagne toujours.** Toute modification faite directement sur la Google Task (titre, date, description) est silencieusement écrasée au push suivant. Ce n'est pas une "négociation" — Taskflow est la source de vérité pour le contenu.
- **La complétion n'est pas soumise à cette règle** : c'est un évènement, pas un champ en conflit. Cocher une tâche dans Google Tasks la complète dans Taskflow au prochain pull (et réciproquement). Si les deux se produisent dans le même intervalle, le résultat (complété) est le même des deux côtés — pas de conflit réel.
- **Nouvelles tâches créées directement dans Google Tasks sont importées** dans Taskflow (sans membre assigné, catégorie par défaut), puis Taskflow devient autoritaire sur leur contenu à partir de ce moment-là.
- **Suppression : Taskflow → Google uniquement.** Supprimer une tâche dans Taskflow supprime la Google Task liée. Supprimer directement la Google Task ne supprime PAS la tâche Taskflow ; au push suivant, Taskflow recrée la Google Task manquante (le lien étant considéré "à réparer", pas "à respecter" comme une suppression volontaire côté Taskflow).
- **Attribution des complétions venues de Google** : Google Tasks n'a aucune notion de partage multi-comptes ni de "qui a coché" (contrainte dure de l'API, pas un choix de design). Toute complétion détectée côté Google est attribuée à une identité générique "Google Tasks" dans l'historique (`by: "google"`, rendu avec une icône dédiée côté frontend — pas un vrai membre du foyer, pour ne pas polluer le sélecteur de membres).
- **Fonctionnalité entièrement désactivable** (toggle dans les réglages) ; sans compte lié, aucun comportement de l'app n'est modifié.

## Architecture

```
hub.OnChange() ──immédiat──▶ push (créer/màj/compléter/supprimer Google Task)
ticker (2 min) ──périodique──▶ pull (lister les Google Tasks modifiées, importer/compléter côté Taskflow)
```

Nouvelle goroutine `runGoogleTasksSync(...)` démarrée dans `main.go`, sur le modèle exact de `runHASensorPublisher` (déjà existante) : réagit à `hub.OnChange()` et à un `ticker`, tolère les échecs sans jamais bloquer le reste de l'app.

Nouveau package `internal/googletasks/` : client REST (net/http, dans le même style que les appels à l'API Supervisor déjà présents dans `main.go`, pas de dépendance lourde type SDK Google) gérant l'échange de tokens et les appels `tasks.list` / `tasks.insert` / `tasks.patch` / `tasks.delete`.

## Modèle de données

- `model.Task` : ajout de `GoogleTaskID *string` (json:"googleTaskId,omitempty").
- `model.Settings` : ajout de `GoogleSyncEnabled bool`, `GoogleClientID string`, `GoogleClientSecret string`, `GoogleRefreshToken string`, `GoogleTaskListID string`, `GoogleLastSyncAt string`, `GoogleLastError string`. Stocké dans le blob JSON existant de la table `settings` — pas de nouvelle table nécessaire.
- Le lien Google→Taskflow (pour retrouver une tâche importée ou déjà syncée) passe par un tag caché dans le champ `notes` de la Google Task (ex: `\n\n[taskflow:id]`), seul champ libre exploitable côté API Google Tasks.

## Connexion du compte (device flow)

1. L'utilisateur crée son propre client OAuth ("TVs and Limited Input devices") dans Google Cloud Console (même démarche que pour l'intégration Google Calendar historique de Home Assistant — les utilisateurs HA connaissent déjà ce parcours), et colle Client ID + Secret dans les réglages Taskflow.
2. Clic "Connecter mon compte Google" → `POST /api/google/connect` démarre le device flow, l'addon affiche un code à 6 chiffres + le lien `google.com/device`.
3. L'utilisateur valide sur son téléphone/PC. Le backend poll en arrière-plan le token endpoint de Google jusqu'à obtenir `access_token` + `refresh_token`.
4. Le frontend poll `GET /api/google/status` jusqu'à `linked: true`. La liste "Taskflow" est créée automatiquement à ce moment (ou réutilisée si elle existe déjà).
5. `POST /api/google/disconnect` efface les tokens et désactive la synchro.

## Réglages / UI

Nouveau panneau "Google Tasks" dans `admin.html` : champs Client ID/Secret, bouton de connexion (avec affichage code + lien pendant le device flow), toggle marche/arrêt une fois lié, indicateur de dernière synchro et dernière erreur (pour debug sans avoir à lire les logs du conteneur).

## Gestion des erreurs

Échecs de refresh token, rate-limit ou erreurs 5xx de l'API Google : consignés dans `GoogleLastError`, retry au tick suivant, jamais de blocage du reste de l'application (goroutine isolée, même tolérance que le publisher MQTT/HA existant).

## Tests

Tests unitaires table-driven (suivant le style de `tasks_test.go` / `history_test.go`) sur la logique de décision du moteur de synchro (étant donné un état Taskflow + un état Google, quelle action en résulte), via une interface de client Google Tasks mockable. Pas d'appel réel à l'API Google en CI.

## Hors périmètre (v1)

- Multi-comptes Google (un compte par membre du foyer).
- Sous-tâches Google Tasks ↔ checklist Taskflow.
- Synchronisation des tâches sans date d'échéance.
