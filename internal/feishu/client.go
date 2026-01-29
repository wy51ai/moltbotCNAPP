package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// MessageHandler is called when a message is received
type MessageHandler func(msg *Message) error

// Message represents a received message
type Message struct {
	MessageID   string
	ChatID      string
	ChatType    string
	Content     string
	Mentions    []Mention
}

// Mention represents a user mention
type Mention struct {
	Key       string
	ID        string
	Name      string
	TenantKey string
}

// Client is a Feishu WebSocket client
type Client struct {
	appID     string
	appSecret string
	client    *lark.Client
	wsClient  *larkws.Client
	handler   MessageHandler
}

// NewClient creates a new Feishu client
func NewClient(appID, appSecret string, handler MessageHandler) *Client {
	client := lark.NewClient(appID, appSecret,
		lark.WithLogLevel(larkcore.LogLevelInfo),
	)

	return &Client{
		appID:     appID,
		appSecret: appSecret,
		client:    client,
		handler:   handler,
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

// SendMessage sends a text message to a chat
func (c *Client) SendMessage(chatID, text string) (string, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
			Content(fmt.Sprintf(`{"text":"%s"}`, escapeJSON(text))).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("failed to send message: %s", resp.Msg)
	}

	messageID := ""
	if resp.Data != nil && resp.Data.MessageId != nil {
		messageID = *resp.Data.MessageId
	}

	return messageID, nil
}

// UpdateMessage updates an existing message
func (c *Client) UpdateMessage(messageID, text string) error {
	req := larkim.NewUpdateMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewUpdateMessageReqBodyBuilder().
			MsgType("text").
			Content(fmt.Sprintf(`{"text":"%s"}`, escapeJSON(text))).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Update(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to update message: %s", resp.Msg)
	}

	return nil
}

// DeleteMessage deletes a message
func (c *Client) DeleteMessage(messageID string) error {
	req := larkim.NewDeleteMessageReqBuilder().
		MessageId(messageID).
		Build()

	resp, err := c.client.Im.Message.Delete(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to delete message: %s", resp.Msg)
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

// escapeJSON is defined in sender.go
