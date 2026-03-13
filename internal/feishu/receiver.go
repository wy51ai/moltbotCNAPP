package feishu

import (
	"context"
)

// FeishuReceiver defines the interface for receiving messages from Feishu
type FeishuReceiver interface {
	Start(ctx context.Context) error
}

// MessageHandler is called when a message is received.
// This type is shared between WebSocket and Webhook receivers.
type MessageHandler func(msg *Message) error

// Interface compliance will be verified after Client refactoring:
// var _ FeishuReceiver = (*Client)(nil)
