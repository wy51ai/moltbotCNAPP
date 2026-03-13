package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestConfig_WebhookModeValidation tests webhook mode validation
func TestConfig_WebhookModeValidation(t *testing.T) {
	// Setup test directory
	testDir := t.TempDir()
	originalDir := os.Getenv("HOME")
	t.Setenv("HOME", testDir)
	defer os.Setenv("HOME", originalDir)

	clawdbotDir := filepath.Join(testDir, ".clawdbot")
	if err := os.MkdirAll(clawdbotDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create clawdbot.json (required for Load())
	clawdbotJSON := map[string]interface{}{
		"gateway": map[string]interface{}{
			"port": 18789,
			"auth": map[string]string{
				"token": "test-token",
			},
		},
	}
	clawdbotData, _ := json.Marshal(clawdbotJSON)
	if err := os.WriteFile(filepath.Join(clawdbotDir, "clawdbot.json"), clawdbotData, 0644); err != nil {
		t.Fatalf("failed to write clawdbot.json: %v", err)
	}

	t.Run("webhook mode missing verification_token", func(t *testing.T) {
		bridgeJSON := map[string]interface{}{
			"mode": "webhook",
			"feishu": map[string]string{
				"app_id":     "test_app_id",
				"app_secret": "test_app_secret",
			},
			"webhook": map[string]string{
				"encrypt_key": "test_encrypt_key",
				// Missing verification_token
			},
		}
		bridgeData, _ := json.Marshal(bridgeJSON)
		if err := os.WriteFile(filepath.Join(clawdbotDir, "bridge.json"), bridgeData, 0644); err != nil {
			t.Fatalf("failed to write bridge.json: %v", err)
		}

		_, err := Load()
		if err == nil {
			t.Error("expected error for missing verification_token, got nil")
		}
		if err != nil && err.Error() != "" {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("webhook mode missing encrypt_key", func(t *testing.T) {
		bridgeJSON := map[string]interface{}{
			"mode": "webhook",
			"feishu": map[string]string{
				"app_id":     "test_app_id",
				"app_secret": "test_app_secret",
			},
			"webhook": map[string]string{
				"verification_token": "test_verification_token",
				// Missing encrypt_key
			},
		}
		bridgeData, _ := json.Marshal(bridgeJSON)
		if err := os.WriteFile(filepath.Join(clawdbotDir, "bridge.json"), bridgeData, 0644); err != nil {
			t.Fatalf("failed to write bridge.json: %v", err)
		}

		_, err := Load()
		if err == nil {
			t.Error("expected error for missing encrypt_key, got nil")
		}
		if err != nil && err.Error() != "" {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("invalid mode value", func(t *testing.T) {
		bridgeJSON := map[string]interface{}{
			"mode": "invalid_mode",
			"feishu": map[string]string{
				"app_id":     "test_app_id",
				"app_secret": "test_app_secret",
			},
		}
		bridgeData, _ := json.Marshal(bridgeJSON)
		if err := os.WriteFile(filepath.Join(clawdbotDir, "bridge.json"), bridgeData, 0644); err != nil {
			t.Fatalf("failed to write bridge.json: %v", err)
		}

		_, err := Load()
		if err == nil {
			t.Error("expected error for invalid mode, got nil")
		}
		if err != nil && err.Error() != "" {
			t.Logf("Got expected error: %v", err)
		}
	})

	t.Run("webhook mode with all required fields", func(t *testing.T) {
		bridgeJSON := map[string]interface{}{
			"mode": "webhook",
			"feishu": map[string]string{
				"app_id":     "test_app_id",
				"app_secret": "test_app_secret",
			},
			"webhook": map[string]string{
				"verification_token": "test_verification_token",
				"encrypt_key":        "test_encrypt_key",
			},
		}
		bridgeData, _ := json.Marshal(bridgeJSON)
		if err := os.WriteFile(filepath.Join(clawdbotDir, "bridge.json"), bridgeData, 0644); err != nil {
			t.Fatalf("failed to write bridge.json: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Errorf("expected no error with valid webhook config, got %v", err)
		}
		if cfg.Mode != "webhook" {
			t.Errorf("expected mode 'webhook', got '%s'", cfg.Mode)
		}
		if cfg.Webhook.VerificationToken != "test_verification_token" {
			t.Errorf("expected verification_token 'test_verification_token', got '%s'", cfg.Webhook.VerificationToken)
		}
		if cfg.Webhook.EncryptKey != "test_encrypt_key" {
			t.Errorf("expected encrypt_key 'test_encrypt_key', got '%s'", cfg.Webhook.EncryptKey)
		}
	})

	t.Run("websocket mode does not require webhook fields", func(t *testing.T) {
		bridgeJSON := map[string]interface{}{
			"mode": "websocket",
			"feishu": map[string]string{
				"app_id":     "test_app_id",
				"app_secret": "test_app_secret",
			},
			// No webhook config needed for websocket mode
		}
		bridgeData, _ := json.Marshal(bridgeJSON)
		if err := os.WriteFile(filepath.Join(clawdbotDir, "bridge.json"), bridgeData, 0644); err != nil {
			t.Fatalf("failed to write bridge.json: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Errorf("expected no error with websocket mode, got %v", err)
		}
		if cfg.Mode != "websocket" {
			t.Errorf("expected mode 'websocket', got '%s'", cfg.Mode)
		}
	})

	t.Run("default mode is websocket", func(t *testing.T) {
		bridgeJSON := map[string]interface{}{
			"feishu": map[string]string{
				"app_id":     "test_app_id",
				"app_secret": "test_app_secret",
			},
			// No mode specified
		}
		bridgeData, _ := json.Marshal(bridgeJSON)
		if err := os.WriteFile(filepath.Join(clawdbotDir, "bridge.json"), bridgeData, 0644); err != nil {
			t.Fatalf("failed to write bridge.json: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Errorf("expected no error with default mode, got %v", err)
		}
		if cfg.Mode != "websocket" {
			t.Errorf("expected default mode 'websocket', got '%s'", cfg.Mode)
		}
	})
}
