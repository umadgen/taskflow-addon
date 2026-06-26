package mqtt

import (
	"encoding/json"
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"foyer/taskflow/internal/model"
)

type Client struct {
	client paho.Client
}

func NewClient(brokerURL string) *Client {
	opts := paho.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID("foyer-server-go").
		SetAutoReconnect(true).
		SetConnectRetryInterval(5 * time.Second).
		SetConnectTimeout(3 * time.Second)
	return &Client{client: paho.NewClient(opts)}
}

func (c *Client) Connect() error {
	token := c.client.Connect()
	token.WaitTimeout(5 * time.Second)
	return token.Error()
}

func (c *Client) PublishSnapshot(snap model.MQTTSnapshot) error {
	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	token := c.client.Publish("foyer/snapshot", 1, true, payload)
	token.Wait()
	return token.Error()
}
