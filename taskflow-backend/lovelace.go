package main

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

// haWSClient est un client minimal pour l'API WebSocket de Home Assistant
// Core, accédée via le proxy Supervisor (ws://supervisor/core/websocket).
// Les ressources Lovelace ne sont gérables que par cette API — il n'existe
// pas d'équivalent REST (voir registerLovelaceResources).
type haWSClient struct {
	conn   *websocket.Conn
	nextID int
}

func dialHAWebsocket(token string) (*haWSClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial("ws://supervisor/core/websocket", nil)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	var hello map[string]any
	if err := conn.ReadJSON(&hello); err != nil {
		conn.Close()
		return nil, fmt.Errorf("lecture auth_required: %w", err)
	}
	if hello["type"] != "auth_required" {
		conn.Close()
		return nil, fmt.Errorf("message inattendu (attendu auth_required): %v", hello)
	}

	if err := conn.WriteJSON(map[string]string{"type": "auth", "access_token": token}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("envoi auth: %w", err)
	}
	var authResp map[string]any
	if err := conn.ReadJSON(&authResp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("lecture réponse auth: %w", err)
	}
	if authResp["type"] != "auth_ok" {
		conn.Close()
		return nil, fmt.Errorf("authentification refusée: %v", authResp)
	}

	return &haWSClient{conn: conn}, nil
}

func (c *haWSClient) Close() error {
	return c.conn.Close()
}

// call envoie une commande (avec son "id" assigné automatiquement) et
// attend la réponse correspondante, en ignorant les autres messages
// (événements, etc.) qui pourraient arriver entre-temps sur la connexion.
func (c *haWSClient) call(payload map[string]any) (map[string]any, error) {
	c.nextID++
	id := c.nextID
	payload["id"] = id
	if err := c.conn.WriteJSON(payload); err != nil {
		return nil, err
	}
	for {
		var resp map[string]any
		if err := c.conn.ReadJSON(&resp); err != nil {
			return nil, err
		}
		respID, _ := resp["id"].(float64)
		if int(respID) != id {
			continue
		}
		if success, _ := resp["success"].(bool); !success {
			return resp, fmt.Errorf("commande refusée: %v", resp["error"])
		}
		return resp, nil
	}
}

// registerLovelaceResources enregistre (ou met à jour) chacune des URLs de
// ressources Lovelace données, via une unique connexion WebSocket vers
// Home Assistant Core. Une ressource déjà enregistrée sous une URL de base
// identique mais une version différente (paramètre "?v=") est supprimée au
// préalable, pour forcer le navigateur à recharger le module au lieu de
// garder en cache l'ancien fichier indéfiniment.
func registerLovelaceResources(token string, urls []string) {
	client, err := dialHAWebsocket(token)
	if err != nil {
		log.Printf("bootstrap: connexion WebSocket HA: %v", err)
		return
	}
	defer client.Close()

	listResp, err := client.call(map[string]any{"type": "lovelace/resources/list"})
	if err != nil {
		log.Printf("bootstrap: lovelace/resources/list: %v", err)
		return
	}
	existing, _ := listResp["result"].([]any)

	for _, url := range urls {
		path := resourcePath(url)
		upToDate := false
		for _, raw := range existing {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			itemURL, _ := item["url"].(string)
			itemID, _ := item["id"].(string)
			if itemURL == url {
				upToDate = true
				continue
			}
			if resourcePath(itemURL) == path {
				if _, err := client.call(map[string]any{
					"type":        "lovelace/resources/delete",
					"resource_id": itemID,
				}); err != nil {
					log.Printf("bootstrap: suppression ressource %s: %v", itemURL, err)
				} else {
					log.Printf("bootstrap: ancienne ressource Lovelace supprimée (%s)", itemURL)
				}
			}
		}
		if upToDate {
			log.Printf("bootstrap: ressource Lovelace déjà à jour (%s)", url)
			continue
		}
		if _, err := client.call(map[string]any{
			"type":     "lovelace/resources/create",
			"res_type": "module",
			"url":      url,
		}); err != nil {
			log.Printf("bootstrap: création ressource %s: %v", url, err)
			continue
		}
		log.Printf("bootstrap: ressource Lovelace enregistrée (%s)", url)
	}
}
