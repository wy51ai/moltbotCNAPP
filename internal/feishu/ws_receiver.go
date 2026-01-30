package feishu

import (
	"context"
	"encoding/json"
	"log"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// Message represents a received message
type Message struct {
	MessageID string
	ChatID    string
	ChatType  string
	Content   string
	Mentions  []Mention
}

// Mention represents a user mention
type Mention struct {
	Key       string
	ID        string
	Name      string
	TenantKey string
}

// Client is a Feishu WebSocket client that implements both FeishuSender and FeishuReceiver
type Client struct {
	*RESTSender          // Embedded RESTSender provides SendMessage/UpdateMessage/DeleteMessage
	appID     string
	appSecret string
	wsClient  *larkws.Client
	handler   MessageHandler
}

// Interface compliance checks
var _ FeishuSender = (*Client)(nil)
var _ FeishuReceiver = (*Client)(nil)

// NewClient creates a new Feishu WebSocket client
func NewClient(appID, appSecret string, handler MessageHandler) *Client {
	return &Client{
		RESTSender: NewRESTSender(appID, appSecret),
		appID:      appID,
		appSecret:  appSecret,
		handler:    handler,
	}
}

// Start starts the WebSocket client
func (c *Client) Start(ctx context.Context) error {
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(c.handleMessage)

	wsClient := larkws.NewClient(c.appID, c.appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	c.wsClient = wsClient

	log.Printf("[Feishu] Starting WebSocket client (appId=%s)", c.appID)
	return wsClient.Start(ctx)
}

// handleMessage handles incoming messages
func (c *Client) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	msg := event.Event.Message

	// Only handle text messages
	if msg.MessageType == nil || *msg.MessageType != "text" {
		return nil
	}

	if msg.Content == nil {
		return nil
	}

	// Parse message content
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*msg.Content), &content); err != nil {
		log.Printf("[Feishu] Failed to parse message content: %v", err)
		return nil
	}

	// Build message
	message := &Message{
		MessageID: getStringValue(msg.MessageId),
		ChatID:    getStringValue(msg.ChatId),
		ChatType:  getStringValue(msg.ChatType),
		Content:   content.Text,
	}

	// Parse mentions
	if msg.Mentions != nil {
		for _, mention := range msg.Mentions {
			mentionID := ""
			if mention.Id != nil && mention.Id.UserId != nil {
				mentionID = *mention.Id.UserId
			}
			message.Mentions = append(message.Mentions, Mention{
				Key:       getStringValue(mention.Key),
				ID:        mentionID,
				Name:      getStringValue(mention.Name),
				TenantKey: getStringValue(mention.TenantKey),
			})
		}
	}

	// Call handler
	if c.handler != nil {
		return c.handler(message)
	}

	return nil
}

// Helper functions

func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
