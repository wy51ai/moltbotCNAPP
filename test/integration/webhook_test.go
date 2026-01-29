//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/wy51ai/moltbotCNAPP/internal/feishu"
)

// TestWebhook_SignatureVerification_Contract 测试 SDK 签名验证契约
func TestWebhook_SignatureVerification_Contract(t *testing.T) {
	port := 19090 // 使用非标准端口避免冲突

	wr := feishu.NewWebhookReceiver(feishu.WebhookConfig{
		Port:              port,
		Path:              "/webhook/feishu",
		VerificationToken: "integration_test_token",
		EncryptKey:        "integration_key_16ch",
		Workers:           1,
		QueueSize:         10,
	}, func(msg *feishu.Message) error {
		return nil
	})

	// 启动服务器 (后台)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- wr.Start(ctx)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	t.Run("invalid signature returns 401", func(t *testing.T) {
		// 构造无签名的事件请求
		eventBody := map[string]interface{}{
			"schema": "2.0",
			"header": map[string]interface{}{
				"event_id":    "int_test_event_1",
				"event_type":  "im.message.receive_v1",
				"create_time": "1234567890000",
				"token":       "integration_test_token",
				"app_id":      "cli_test",
				"tenant_key":  "tenant_test",
			},
			"event": map[string]interface{}{
				"message": map[string]interface{}{
					"message_id": "msg_int_test",
				},
			},
		}
		body, _ := json.Marshal(eventBody)

		resp, err := http.Post(
			baseURL+"/webhook/feishu",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// 契约断言: 无效签名必须返回 401
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for invalid signature, got %d", resp.StatusCode)
		}
	})

	// 停止服务器
	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Logf("server error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Log("server shutdown timeout")
	}
}
