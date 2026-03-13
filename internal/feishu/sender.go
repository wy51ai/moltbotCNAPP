package feishu

import (
	"context"
	"encoding/json"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuSender defines the interface for sending messages to Feishu
type FeishuSender interface {
	SendMessage(chatID, text string) (messageID string, err error)
	UpdateMessage(messageID, text string) error
	DeleteMessage(messageID string) error
}

// RESTSender implements FeishuSender using Feishu REST API
type RESTSender struct {
	client *lark.Client
}

// Interface compliance check
var _ FeishuSender = (*RESTSender)(nil)

// NewRESTSender creates a new RESTSender with the given app credentials
func NewRESTSender(appID, appSecret string) *RESTSender {
	client := lark.NewClient(appID, appSecret,
		lark.WithLogLevel(larkcore.LogLevelInfo),
	)
	return &RESTSender{
		client: client,
	}
}

// SendMessage sends a text message to a chat
func (s *RESTSender) SendMessage(chatID, text string) (string, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
			Content(fmt.Sprintf(`{"text":"%s"}`, escapeJSON(text))).
			Build()).
		Build()

	resp, err := s.client.Im.Message.Create(context.Background(), req)
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
func (s *RESTSender) UpdateMessage(messageID, text string) error {
	req := larkim.NewUpdateMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewUpdateMessageReqBodyBuilder().
			MsgType("text").
			Content(fmt.Sprintf(`{"text":"%s"}`, escapeJSON(text))).
			Build()).
		Build()

	resp, err := s.client.Im.Message.Update(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to update message: %s", resp.Msg)
	}

	return nil
}

// DeleteMessage deletes a message
func (s *RESTSender) DeleteMessage(messageID string) error {
	req := larkim.NewDeleteMessageReqBuilder().
		MessageId(messageID).
		Build()

	resp, err := s.client.Im.Message.Delete(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("failed to delete message: %s", resp.Msg)
	}

	return nil
}

// escapeJSON escapes a string for use in JSON
func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return string(b[1 : len(b)-1])
	}
	return string(b)
}
