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

// TestWebhook_SignatureVerification_Contract 测试 SDK 身份验证契约
// 通过 challenge 验证测试 token 验证逻辑（SDK 契约保护）
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

	t.Run("challenge with invalid token returns 401", func(t *testing.T) {
		// 测试 challenge 验证失败场景（token 不匹配）
		// 这是明确的签名/token 验证契约
		challengeBody := map[string]interface{}{
			"type":      "url_verification",
			"token":     "wrong_token", // 错误的 token
			"challenge": "test_challenge_string",
		}
		body, _ := json.Marshal(challengeBody)

		resp, err := http.Post(
			baseURL+"/webhook/feishu",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// 契约断言: token 不匹配必须返回 401
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for invalid token, got %d", resp.StatusCode)
		}
	})

	t.Run("challenge with valid token returns 200", func(t *testing.T) {
		// 验证正确的 token 能通过
		challengeBody := map[string]interface{}{
			"type":      "url_verification",
			"token":     "integration_test_token", // 正确的 token
			"challenge": "test_challenge_string",
		}
		body, _ := json.Marshal(challengeBody)

		resp, err := http.Post(
			baseURL+"/webhook/feishu",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// 契约断言: 正确的 token 返回 200
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK for valid token, got %d", resp.StatusCode)
		}

		// 验证响应体包含 challenge
		var respBody map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if respBody["challenge"] != "test_challenge_string" {
			t.Errorf("expected challenge in response, got %v", respBody)
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
