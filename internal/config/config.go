package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configuration for the bridge
type Config struct {
	Mode     string         // "websocket" | "webhook", default "websocket"
	Feishu   FeishuConfig
	Webhook  WebhookConfig  // Webhook mode configuration
	Clawdbot ClawdbotConfig
}

// FeishuConfig contains Feishu-specific configuration
type FeishuConfig struct {
	AppID               string
	AppSecret           string
	ThinkingThresholdMs int
}

// WebhookConfig contains Webhook mode configuration
type WebhookConfig struct {
	Port              int    // Server port, default 9090
	Path              string // Webhook path, default "/webhook/feishu"
	VerificationToken string // Required in webhook mode
	EncryptKey        string // Required in webhook mode
	Workers           int    // Number of workers, default 10
	QueueSize         int    // Queue size, default 100
}

// ClawdbotConfig contains Clawdbot Gateway configuration
type ClawdbotConfig struct {
	GatewayPort  int
	GatewayToken string
	AgentID      string
}

// clawdbotJSON matches ~/.clawdbot/clawdbot.json (managed by ClawdBot)
type clawdbotJSON struct {
	Gateway struct {
		Port int `json:"port"`
		Auth struct {
			Token string `json:"token"`
		} `json:"auth"`
	} `json:"gateway"`
}

// bridgeJSON matches ~/.clawdbot/bridge.json
type bridgeJSON struct {
	Mode   string `json:"mode,omitempty"`
	Feishu struct {
		AppID     string `json:"app_id"`
		AppSecret string `json:"app_secret"`
	} `json:"feishu"`
	Webhook struct {
		Port              int    `json:"port,omitempty"`
		Path              string `json:"path,omitempty"`
		VerificationToken string `json:"verification_token,omitempty"`
		EncryptKey        string `json:"encrypt_key,omitempty"`
		Workers           int    `json:"workers,omitempty"`
		QueueSize         int    `json:"queue_size,omitempty"`
	} `json:"webhook,omitempty"`
	ThinkingThresholdMs *int   `json:"thinking_threshold_ms,omitempty"`
	AgentID             string `json:"agent_id,omitempty"`
}

// Dir returns the config directory path
// Tries ~/.clawdbot first, falls back to ~/.openclaw
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Priority order: .clawdbot, .openclaw
	candidates := []string{
		filepath.Join(home, ".clawdbot"),
		filepath.Join(home, ".openclaw"),
	}

	// Return first existing directory, or default to .clawdbot
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}

	// Default to .clawdbot if none exist
	return candidates[0], nil
}

// findConfigFile searches for a config file with multiple possible names
// Returns the first file found, or error if none exist
func findConfigFile(dir string, candidates ...string) (string, error) {
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	// Return error with all attempted paths
	return "", fmt.Errorf("config file not found, tried: %v", candidates)
}

// Load reads configuration from config files
// Supports both ~/.clawdbot/ and ~/.openclaw/ directories
// Gateway config: clawdbot.json or openclaw.json
// Bridge config: bridge.json
func Load() (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	// Find gateway config file: clawdbot.json or openclaw.json
	gwPath, err := findConfigFile(dir, "clawdbot.json", "openclaw.json")
	if err != nil {
		return nil, fmt.Errorf("failed to find gateway config (clawdbot.json or openclaw.json) in %s: %w", dir, err)
	}
	gwData, err := os.ReadFile(gwPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", gwPath, err)
	}
	var gwCfg clawdbotJSON
	if err := json.Unmarshal(gwData, &gwCfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", gwPath, err)
	}

	// Find bridge config file: bridge.json
	brPath, err := findConfigFile(dir, "bridge.json")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to find bridge.json in %s: %w\n\nCreate it with:\n  {\n    \"feishu\": {\n      \"app_id\": \"cli_xxx\",\n      \"app_secret\": \"xxx\"\n    }\n  }", dir, err)
	}
	brData, err := os.ReadFile(brPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", brPath, err)
	}
	var brCfg bridgeJSON
	if err := json.Unmarshal(brData, &brCfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", brPath, err)
	}

	// Validate required fields
	if brCfg.Feishu.AppID == "" {
		return nil, fmt.Errorf("feishu.app_id is required in ~/.clawdbot/bridge.json")
	}
	if brCfg.Feishu.AppSecret == "" {
		return nil, fmt.Errorf("feishu.app_secret is required in ~/.clawdbot/bridge.json")
	}

	// Parse and validate mode
	mode := brCfg.Mode
	if mode == "" {
		mode = "websocket" // Default mode
	}
	if mode != "websocket" && mode != "webhook" {
		return nil, fmt.Errorf("~/.clawdbot/bridge.json: invalid mode %q (must be \"websocket\" or \"webhook\")", mode)
	}

	// Validate webhook mode required fields
	if mode == "webhook" {
		if brCfg.Webhook.VerificationToken == "" {
			return nil, fmt.Errorf(
				"~/.clawdbot/bridge.json: webhook.verification_token is required when mode is \"webhook\"\n\n"+
					"Add to your config:\n"+
					"  \"webhook\": {\n"+
					"    \"verification_token\": \"your-token-from-feishu-console\",\n"+
					"    \"encrypt_key\": \"your-encrypt-key\"\n"+
					"  }",
			)
		}
		if brCfg.Webhook.EncryptKey == "" {
			return nil, fmt.Errorf(
				"~/.clawdbot/bridge.json: webhook.encrypt_key is required when mode is \"webhook\"\n\n"+
					"Add to your config:\n"+
					"  \"webhook\": {\n"+
					"    \"verification_token\": \"your-token-from-feishu-console\",\n"+
					"    \"encrypt_key\": \"your-encrypt-key\"\n"+
					"  }",
			)
		}
	}

	// Build config with defaults
	cfg := &Config{
		Mode: mode,
		Feishu: FeishuConfig{
			AppID:               brCfg.Feishu.AppID,
			AppSecret:           brCfg.Feishu.AppSecret,
			ThinkingThresholdMs: 0,
		},
		Webhook: WebhookConfig{
			Port:              9090,
			Path:              "/webhook/feishu",
			VerificationToken: brCfg.Webhook.VerificationToken,
			EncryptKey:        brCfg.Webhook.EncryptKey,
			Workers:           10,
			QueueSize:         100,
		},
		Clawdbot: ClawdbotConfig{
			GatewayPort:  gwCfg.Gateway.Port,
			GatewayToken: gwCfg.Gateway.Auth.Token,
			AgentID:      "main",
		},
	}

	// Override webhook defaults if set in config
	if brCfg.Webhook.Port != 0 {
		cfg.Webhook.Port = brCfg.Webhook.Port
	}
	if brCfg.Webhook.Path != "" {
		cfg.Webhook.Path = brCfg.Webhook.Path
	}
	if brCfg.Webhook.Workers != 0 {
		cfg.Webhook.Workers = brCfg.Webhook.Workers
	}
	if brCfg.Webhook.QueueSize != 0 {
		cfg.Webhook.QueueSize = brCfg.Webhook.QueueSize
	}

	if brCfg.ThinkingThresholdMs != nil {
		cfg.Feishu.ThinkingThresholdMs = *brCfg.ThinkingThresholdMs
	}
	if brCfg.AgentID != "" {
		cfg.Clawdbot.AgentID = brCfg.AgentID
	}
	if cfg.Clawdbot.GatewayPort == 0 {
		cfg.Clawdbot.GatewayPort = 18789
	}

	return cfg, nil
}
