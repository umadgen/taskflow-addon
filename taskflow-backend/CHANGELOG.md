# Changelog

## 1.6.23

- Retire l'entrée "non faite" (?) que la réinitialisation hebdomadaire ajoutait automatiquement à l'historique : les tâches hebdomadaires non cochées à temps se réinitialisent maintenant silencieusement, sans polluer l'historique.

## 1.6.22

- Documente dans CLAUDE.md que les commits et titres de PR doivent suivre Conventional Commits, en anglais.

## 1.6.21

- Introduit ce changelog, visible directement dans l'interface de l'add-on Home Assistant.

## 1.6.20

- Harmonise le modal "qui a fait ça" de la carte hebdomadaire avec celui utilisé pour les tâches classiques (pilules à bordure colorée, poignée de glisser, mêmes tailles).

## 1.6.19

- Corrige l'enregistrement automatique des cartes Lovelace : Home Assistant ne gère les ressources Lovelace que via son API WebSocket, pas via REST (l'ancien endpoint utilisé répondait 404 depuis toujours).

## 1.6.18

- Les échecs d'enregistrement des ressources Lovelace étaient silencieux ; ils sont désormais loggués avec le code d'erreur et le corps de la réponse.

## 1.6.17

- Ajoute les tâches hebdomadaires "libre service" : à faire une fois par semaine, n'importe quel jour, avec objectif configurable (N fois/semaine), compteur de progression, dernière date de complétion, et une nouvelle carte Lovelace dédiée.

## 1.6.16

- Corrige l'indicateur "en retard" qui restait affiché après la complétion d'une tâche récurrente.

## 1.6.15

- Ajoute une bordure interne fine pour distinguer visuellement les tâches critiques.

## 1.6.14

- Permet de marquer une tâche comme critique depuis le panneau admin.

## 1.6.13

- Ajoute les tâches critiques avec alerte visuelle à l'échéance.

## 1.6.12

- Permet de décaler une tâche au lendemain, avec trace dans l'historique (décalage/ignorance d'occurrence).

## 1.6.11

- Corrige la carte de progression du foyer qui ne se rafraîchissait pas au changement de jour.

## 1.6.10

- Corrige à nouveau le rafraîchissement au changement de jour (toujours cassé après le 1.6.9).

## 1.6.9

- Corrige un décalage de 2h persistant sur les anciennes entrées d'historique.

## 1.6.8

- Corrige les tâches qui ne s'actualisaient pas au changement de jour.

## 1.6.7

- Corrige une heure incorrecte affichée dans l'historique.

## 1.6.6

- Corrige la date des tâches ponctuelles, retire le swipe, corrige la couleur rose.

## 1.6.5

- Corrige le numéro de version qui n'avait pas été mis à jour au commit précédent (empêchait Supervisor de détecter la mise à jour et la CI de publier l'image taguée correctement).

## 1.6.4

- Ajoute le mode vacances : mise en pause des échéances jusqu'à une date de retour.

## 1.6.3

- Ajoute le thème sombre et les checklists de sous-tâches.

## 1.6.2

- Ajoute les statistiques par membre et le calendrier mensuel.

## 1.6.1

- Ajoute l'export/import de sauvegarde au format JSON.

## 1.6.0

- Ajoute la gestion des animaux (fiche santé dans l'admin + carte Lovelace dédiée).

## 1.5.0

- Ajoute l'interface d'administration embarquée, accessible via l'ingress Home Assistant.

## 1.4.3

- Ajoute un redémarrage automatique de HA Core au premier install, pour activer la route statique `/local/`.

## 1.4.2

- La carte de progression utilise désormais l'historique pour compter les tâches récurrentes complétées.

## 1.4.1

- Corrige la complétion de tâche (passage par l'API directe) et un bug d'état "fait" indéfini.

## 1.4.0

- Sert la carte Lovelace JS directement depuis le backend et ajoute l'endpoint `/api/ops`.

## 1.3.0

- Ajoute le capteur Home Assistant `sensor.foyer_snapshot`, expose le port 8787, corrige l'URL d'API utilisée par la carte.

## 1.2.0

- Corrige une erreur "exec format error" en utilisant un build QEMU natif.

## 1.1.0

- Pré-construit les images Docker via GitHub Actions.

## 1.0.0

- Version initiale de l'add-on Taskflow.
- Installation et enregistrement automatique de la carte Lovelace au démarrage.
