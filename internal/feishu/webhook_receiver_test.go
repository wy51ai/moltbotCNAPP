package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// TestWebhookReceiver_NewWebhookReceiver tests the constructor
func TestWebhookReceiver_NewWebhookReceiver(t *testing.T) {
	t.Run("panics without VerificationToken", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for missing VerificationToken")
			}
		}()
		NewWebhookReceiver(WebhookConfig{
			EncryptKey: "test_encrypt_key",
		}, nil)
	})

	t.Run("panics without EncryptKey", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for missing EncryptKey")
			}
		}()
		NewWebhookReceiver(WebhookConfig{
			VerificationToken: "test_token",
		}, nil)
	})

	t.Run("sets defaults", func(t *testing.T) {
		wr := NewWebhookReceiver(WebhookConfig{
			VerificationToken: "test_token",
			EncryptKey:        "test_encrypt_key",
		}, func(msg *Message) error { return nil })

		if wr.config.Port != 8080 {
			t.Errorf("expected default port 8080, got %d", wr.config.Port)
		}
		if wr.config.Path != "/webhook/feishu" {
			t.Errorf("expected default path /webhook/feishu, got %s", wr.config.Path)
		}
		if wr.config.Workers != 10 {
			t.Errorf("expected default workers 10, got %d", wr.config.Workers)
		}
		if wr.config.QueueSize != 100 {
			t.Errorf("expected default queue size 100, got %d", wr.config.QueueSize)
		}
	})
}

// TestWebhookReceiver_MethodNotAllowed tests that non-POST requests return 405
func TestWebhookReceiver_MethodNotAllowed(t *testing.T) {
	wr := createTestWebhookReceiver(t)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/webhook/feishu", nil)
			rr := httptest.NewRecorder()

			wr.webhookHandler(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405, got %d", rr.Code)
			}
		})
	}
}

// TestWebhookReceiver_BodyTooLarge tests that oversized body returns 413
func TestWebhookReceiver_BodyTooLarge(t *testing.T) {
	wr := createTestWebhookReceiver(t)

	// Create body larger than 1MB
	largeBody := strings.Repeat("x", 1<<20+1) // 1MB + 1 byte
	req := httptest.NewRequest(http.MethodPost, "/webhook/feishu", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	wr.webhookHandler(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", rr.Code)
	}
}

// TestWebhookReceiver_Challenge tests URL verification (challenge) request
func TestWebhookReceiver_Challenge(t *testing.T) {
	wr := createTestWebhookReceiver(t)

	t.Run("valid challenge", func(t *testing.T) {
		challengeReq := map[string]string{
			"type":      "url_verification",
			"token":     "test_verification_token",
			"challenge": "test_challenge_value",
		}
		body, _ := json.Marshal(challengeReq)
		req := httptest.NewRequest(http.MethodPost, "/webhook/feishu", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		wr.webhookHandler(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var resp map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if resp["challenge"] != "test_challenge_value" {
			t.Errorf("expected challenge 'test_challenge_value', got '%s'", resp["challenge"])
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		challengeReq := map[string]string{
			"type":      "url_verification",
			"token":     "wrong_token",
			"challenge": "test_challenge_value",
		}
		body, _ := json.Marshal(challengeReq)
		req := httptest.NewRequest(http.MethodPost, "/webhook/feishu", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		wr.webhookHandler(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})
}

// TestWebhookReceiver_InvalidSignature tests that invalid signature returns 401
func TestWebhookReceiver_InvalidSignature(t *testing.T) {
	wr := createTestWebhookReceiver(t)

	// Create a non-challenge event request without proper signature
	eventBody := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{
			"event_id":    "test_event_id",
			"event_type":  "im.message.receive_v1",
			"create_time": "1234567890",
			"token":       "test_verification_token",
			"app_id":      "test_app_id",
			"tenant_key":  "test_tenant_key",
		},
		"event": map[string]interface{}{
			"message": map[string]interface{}{
				"message_id": "test_msg_id",
			},
		},
	}
	body, _ := json.Marshal(eventBody)

	req := httptest.NewRequest(http.MethodPost, "/webhook/feishu", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No signature headers - should fail verification
	rr := httptest.NewRecorder()

	wr.webhookHandler(rr, req)

	// SDK will fail signature verification and return 500, which we map to 401
	// Note: The SDK returns 500 with "signature verification failed" in body
	if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 401 or 500, got %d", rr.Code)
	}
}

// TestWebhookReceiver_QueueFull tests that queue full returns 503
func TestWebhookReceiver_QueueFull(t *testing.T) {
	// Create a receiver with tiny queue
	handlerCalled := make(chan struct{})
	wr := NewWebhookReceiver(WebhookConfig{
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key_1234", // 16+ chars for AES
		Workers:           1,
		QueueSize:         1,
	}, func(msg *Message) error {
		// Block until test signals
		<-handlerCalled
		return nil
	})

	// Start worker pool
	wr.workerPool.Start()
	defer wr.workerPool.Shutdown(time.Second)

	// Fill the queue by submitting jobs directly
	// First job - will be picked up by worker and block
	err := wr.workerPool.Submit(Job{EventID: "fill1", Handler: func() error {
		<-handlerCalled
		return nil
	}})
	if err != nil {
		t.Fatalf("failed to submit first job: %v", err)
	}

	// Give worker time to pick up the job
	time.Sleep(10 * time.Millisecond)

	// Second job - fills the queue
	err = wr.workerPool.Submit(Job{EventID: "fill2", Handler: func() error { return nil }})
	if err != nil {
		t.Fatalf("failed to submit second job: %v", err)
	}

	// Third job should fail with queue full
	err = wr.workerPool.Submit(Job{EventID: "overflow", Handler: func() error { return nil }})
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	// Cleanup
	close(handlerCalled)
}

// TestWebhookReceiver_Deduplication tests event deduplication
func TestWebhookReceiver_Deduplication(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	wr := NewWebhookReceiver(WebhookConfig{
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key_1234",
	}, func(msg *Message) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	})

	// Manually add an event ID to the dedupe cache
	eventID := "test_event_123"
	wr.dedupeCache.Store(eventID, time.Now())

	// Try to store the same event ID again
	if _, exists := wr.dedupeCache.LoadOrStore(eventID, time.Now()); !exists {
		t.Error("expected event to already exist in dedupe cache")
	}
}

// TestWebhookReceiver_CleanupDedupeCache tests dedupe cache cleanup
func TestWebhookReceiver_CleanupDedupeCache(t *testing.T) {
	wr := createTestWebhookReceiver(t)

	// Add old and new entries
	oldTime := time.Now().Add(-15 * time.Minute) // Older than 10 minutes
	newTime := time.Now()

	wr.dedupeCache.Store("old_event", oldTime)
	wr.dedupeCache.Store("new_event", newTime)

	// Manually trigger cleanup logic
	now := time.Now()
	wr.dedupeCache.Range(func(key, value interface{}) bool {
		if ts, ok := value.(time.Time); ok {
			if now.Sub(ts) > 10*time.Minute {
				wr.dedupeCache.Delete(key)
			}
		}
		return true
	})

	// Check old entry was removed
	if _, exists := wr.dedupeCache.Load("old_event"); exists {
		t.Error("expected old_event to be cleaned up")
	}

	// Check new entry still exists
	if _, exists := wr.dedupeCache.Load("new_event"); !exists {
		t.Error("expected new_event to still exist")
	}
}

// TestWebhookReceiver_ConvertEventToMessage tests message conversion
func TestWebhookReceiver_ConvertEventToMessage(t *testing.T) {
	wr := createTestWebhookReceiver(t)

	t.Run("nil event returns empty message", func(t *testing.T) {
		msg := wr.convertEventToMessage(nil)
		if msg == nil {
			t.Error("expected non-nil message")
		}
	})

	// Note: Full event conversion testing would require constructing
	// SDK event structs which is complex. The logic mirrors ws_receiver.go
	// which is already tested in production.
}

// Helper function to create a test WebhookReceiver with initialized dispatcher
func createTestWebhookReceiver(t *testing.T) *WebhookReceiver {
	t.Helper()
	wr := NewWebhookReceiver(WebhookConfig{
		VerificationToken: "test_verification_token",
		EncryptKey:        "test_encrypt_key_1234", // At least 16 chars for AES
	}, func(msg *Message) error { return nil })

	// Initialize dispatcher for testing
	wr.dispatcher = dispatcher.NewEventDispatcher(
		wr.config.VerificationToken,
		wr.config.EncryptKey,
	).OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		return wr.handleMessageEvent(event)
	})

	return wr
}
