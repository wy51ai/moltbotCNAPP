package clawdbot

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client is a ClawdBot Gateway WebSocket client
type Client struct {
	port     int
	token    string
	agentID  string
	mu       sync.Mutex
}

// NewClient creates a new ClawdBot Gateway client
func NewClient(port int, token, agentID string) *Client {
	return &Client{
		port:    port,
		token:   token,
		agentID: agentID,
	}
}

// Request represents a request to the gateway
type Request struct {
	Type   string      `json:"type"`
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// Response represents a response from the gateway
type Response struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *ErrorInfo      `json:"error,omitempty"`
	Event   string          `json:"event,omitempty"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Message string `json:"message"`
}

// ConnectParams contains connection parameters
type ConnectParams struct {
	MinProtocol int         `json:"minProtocol"`
	MaxProtocol int         `json:"maxProtocol"`
	Client      ClientInfo  `json:"client"`
	Role        string      `json:"role"`
	Scopes      []string    `json:"scopes"`
	Auth        AuthInfo    `json:"auth"`
	Locale      string      `json:"locale"`
	UserAgent   string      `json:"userAgent"`
}

// ClientInfo contains client information
type ClientInfo struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Mode     string `json:"mode"`
}

// AuthInfo contains authentication information
type AuthInfo struct {
	Token string `json:"token"`
}

// AgentParams contains agent request parameters
type AgentParams struct {
	Message        string `json:"message"`
	AgentID        string `json:"agentId"`
	SessionKey     string `json:"sessionKey"`
	Deliver        bool   `json:"deliver"`
	IdempotencyKey string `json:"idempotencyKey"`
}

// AgentPayload contains the agent response payload
type AgentPayload struct {
	RunID string `json:"runId,omitempty"`
}

// EventPayload contains event data
type EventPayload struct {
	RunID  string          `json:"runId,omitempty"`
	Stream string          `json:"stream,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// StreamData contains stream data
type StreamData struct {
	Text  string `json:"text,omitempty"`
	Delta string `json:"delta,omitempty"`
	Phase string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

// AskClawdbot sends a message to ClawdBot and returns the response
func (c *Client) AskClawdbot(text, sessionKey string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	url := fmt.Sprintf("ws://127.0.0.1:%d", c.port)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to connect to gateway: %w", err)
	}
	defer conn.Close()

	var runID string
	var buffer string
	responseChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	// Message reader goroutine
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var resp Response
			if err := json.Unmarshal(message, &resp); err != nil {
				continue
			}

			// Step 1: Handle connect challenge
			if resp.Type == "event" && resp.Event == "connect.challenge" {
				connectReq := Request{
					Type:   "req",
					ID:     "connect",
					Method: "connect",
					Params: ConnectParams{
						MinProtocol: 3,
						MaxProtocol: 3,
						Client: ClientInfo{
							ID:       "gateway-client",
							Version:  "0.2.0",
							Platform: "linux",
							Mode:     "backend",
						},
						Role:   "operator",
						Scopes: []string{"operator.read", "operator.write"},
						Auth: AuthInfo{
							Token: c.token,
						},
						Locale:    "zh-CN",
						UserAgent: "clawdbot-bridge-go",
					},
				}

				if err := conn.WriteJSON(connectReq); err != nil {
					errorChan <- fmt.Errorf("failed to send connect request: %w", err)
					return
				}
				continue
			}

			// Step 2: Handle connect response
			if resp.Type == "res" && resp.ID == "connect" {
				if !resp.OK {
					errMsg := "connect failed"
					if resp.Error != nil {
						errMsg = resp.Error.Message
					}
					errorChan <- fmt.Errorf(errMsg)
					return
				}

				// Send agent request
				agentReq := Request{
					Type:   "req",
					ID:     "agent",
					Method: "agent",
					Params: AgentParams{
						Message:        text,
						AgentID:        c.agentID,
						SessionKey:     sessionKey,
						Deliver:        false,
						IdempotencyKey: uuid.New().String(),
					},
				}

				if err := conn.WriteJSON(agentReq); err != nil {
					errorChan <- fmt.Errorf("failed to send agent request: %w", err)
					return
				}
				continue
			}

			// Step 3: Handle agent response
			if resp.Type == "res" && resp.ID == "agent" {
				if !resp.OK {
					errMsg := "agent error"
					if resp.Error != nil {
						errMsg = resp.Error.Message
					}
					errorChan <- fmt.Errorf(errMsg)
					return
				}

				var payload AgentPayload
				if err := json.Unmarshal(resp.Payload, &payload); err == nil {
					runID = payload.RunID
				}
				continue
			}

			// Step 4: Handle agent events
			if resp.Type == "event" && resp.Event == "agent" {
				var eventPayload EventPayload
				if err := json.Unmarshal(resp.Payload, &eventPayload); err != nil {
					continue
				}

				// Check runID matches if we have one
				if runID != "" && eventPayload.RunID != runID {
					continue
				}

				// Handle assistant stream
				if eventPayload.Stream == "assistant" {
					var streamData StreamData
					if err := json.Unmarshal(eventPayload.Data, &streamData); err == nil {
						if streamData.Text != "" {
							buffer = streamData.Text
						} else if streamData.Delta != "" {
							buffer += streamData.Delta
						}
					}
					continue
				}

				// Handle lifecycle stream
				if eventPayload.Stream == "lifecycle" {
					var streamData StreamData
					if err := json.Unmarshal(eventPayload.Data, &streamData); err == nil {
						if streamData.Phase == "end" {
							responseChan <- buffer
							return
						}
						if streamData.Phase == "error" {
							errMsg := "agent error"
							if streamData.Message != "" {
								errMsg = streamData.Message
							}
							errorChan <- fmt.Errorf(errMsg)
							return
						}
					}
				}
			}
		}
	}()

	// Wait for response or timeout
	select {
	case result := <-responseChan:
		return result, nil
	case err := <-errorChan:
		return "", err
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("timeout waiting for response")
	}
}
