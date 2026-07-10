# 🏠 Taskflow

**Taskflow** est un gestionnaire de tâches ménagères pour la famille, conçu comme un **add-on Home Assistant**. Il centralise les corvées du foyer, le suivi des animaux de compagnie et un historique commun, avec une synchronisation en temps réel entre tous les membres de la famille et une intégration native à Home Assistant (capteur, cartes Lovelace, ingress).

Le nom de code interne du projet est *foyer* — vous le croiserez dans le code (module Go `foyer/taskflow`, topic MQTT `foyer/snapshot`, etc.).

## ✨ Fonctionnalités

### 📋 Gestion des tâches
- Création, édition, suppression de tâches avec catégorie, date/heure d'échéance et check-list de sous-étapes.
- Attribution des tâches à un membre du foyer.
- Tâches récurrentes : quotidiennes, hebdomadaires (jours de la semaine au choix) ou mensuelles (jour du mois).
- Marquage fait / non fait, avec traçabilité de qui a fait quoi et quand.
- **Import en masse (JSON)** avec un générateur de prompt prêt à copier-coller dans un assistant IA pour créer un planning de tâches automatiquement.

### 🐾 Suivi des animaux
- Fiche par animal (espèce, race, âge, sexe, emoji/avatar).
- Suivi du poids avec historique/graphique.
- Carnet de vaccins et rendez-vous vétérinaires.
- Traitements récurrents (antiparasitaires, etc.) avec prochaine échéance calculée.

### 👨‍👩‍👧‍👦 Foyer & réglages
- Gestion des membres du foyer (nom, couleur/« tone », avatar).
- Historique complet des actions (tâches complétées, par qui, quand).
- **Mode vacances** : mise en pause des échéances jusqu'à une date de retour.
- Thème clair/sombre/auto, couleur d'accent, notifications et sons configurables.
- **Export / restauration** des données en un fichier JSON de sauvegarde.

### 🔄 Temps réel & intégration Home Assistant
- Synchronisation instantanée entre tous les clients connectés via WebSocket.
- Publication MQTT optionnelle (topic `foyer/snapshot`) pour s'interfacer avec le reste de Home Assistant.
- Création automatique d'un capteur `sensor.foyer_snapshot` (tâches, membres, historique, mode vacances) via l'API Supervisor.
- Deux cartes Lovelace personnalisées (tâches et animaux) installées et mises à jour automatiquement au démarrage.
- Accès via **ingress** Home Assistant : pas de port à exposer, l'interface d'administration s'ouvre directement dans la barre latérale.

### 🚧 À venir
- Synchronisation bidirectionnelle avec **Google Tasks** (conception détaillée disponible dans [`docs/superpowers/specs/2026-07-08-google-tasks-sync-design.md`](docs/superpowers/specs/2026-07-08-google-tasks-sync-design.md)) — pas encore implémentée.

## 🧱 Architecture

- **Backend** : Go (routeur [chi](https://github.com/go-chi/chi)), base de données SQLite embarquée (sans CGO, via `modernc.org/sqlite`), WebSocket ([gorilla/websocket](https://github.com/gorilla/websocket)) et client MQTT ([paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang)).
- **Frontend** : une interface d'administration statique (`web/admin.html`) servie par le backend, plus deux cartes Lovelace autonomes (`web/foyer-tasks-card.js`, `web/foyer-pets-card.js`).
- **Distribution** : image Docker multi-architecture (`amd64`, `aarch64`, `armv7`, `armhf`, `i386`) publiée sur GitHub Container Registry via GitHub Actions.

## 📦 Installation

### En tant qu'add-on Home Assistant (recommandé)

1. Dans Home Assistant, allez dans **Paramètres → Add-ons → Boutique d'add-ons**.
2. Ouvrez le menu (⋮) en haut à droite → **Dépôts**, puis ajoutez :
   ```
   https://github.com/umadgen/taskflow-addon
   ```
3. Rafraîchissez la boutique, l'add-on **Taskflow** apparaît dans la liste. Cliquez dessus puis **Installer**.
4. Dans l'onglet **Configuration**, renseignez :
   - `household_name` : le nom de votre foyer.
   - `members` : liste des membres (`name`, `tone`, `avatar` optionnel).
   - `pets` : liste des animaux (`name`, `species`, `emoji` optionnel).
5. Démarrez l'add-on. L'interface est accessible directement depuis la barre latérale de Home Assistant grâce à l'ingress (port interne `8787`).
6. Les cartes Lovelace `foyer-tasks-card` et `foyer-pets-card` sont installées et enregistrées automatiquement ; vous pouvez les ajouter à vos tableaux de bord.

### En développement local / hors Home Assistant

Prérequis : Go 1.24+.

```bash
cd taskflow-backend
go run .
```

Variables d'environnement disponibles :

| Variable | Défaut | Description |
|---|---|---|
| `PORT` | `8787` | Port d'écoute HTTP |
| `FOYER_DB` | `./foyer.sqlite` | Chemin de la base SQLite |
| `FOYER_MQTT_URL` | `mqtt://localhost:1883` | Broker MQTT (optionnel, l'app fonctionne sans) |
| `FOYER_STATIC` | `./public` | Dossier de fichiers statiques additionnels à servir |
| `FOYER_SECRET` | *(vide)* | Secret requis pour l'endpoint `/api/import` |
| `SUPERVISOR_TOKEN` | *(vide)* | Token fourni automatiquement par Home Assistant Supervisor |
| `FOYER_OPTIONS` | `/data/options.json` | Fichier d'options utilisé pour la première initialisation (membres/animaux/nom du foyer) |

### Avec Docker

```bash
cd taskflow-backend
docker build -t taskflow .
docker run -p 8787:8787 -v taskflow-data:/data taskflow
```

L'interface est alors disponible sur `http://localhost:8787`.

## 🧪 Tests

```bash
cd taskflow-backend
go test ./...
```

## 📄 Licence

Ce projet est distribué sous licence **MIT** — voir le fichier [LICENSE](LICENSE) pour le texte complet.
