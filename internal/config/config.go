package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configuration for the bridge
type Config struct {
	Feishu   FeishuConfig
	Clawdbot ClawdbotConfig
}

// FeishuConfig contains Feishu-specific configuration
type FeishuConfig struct {
	AppID               string
	AppSecret           string
	ThinkingThresholdMs int
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
	Feishu struct {
		AppID     string `json:"app_id"`
		AppSecret string `json:"app_secret"`
	} `json:"feishu"`
	ThinkingThresholdMs *int   `json:"thinking_threshold_ms,omitempty"`
	AgentID             string `json:"agent_id"`
}

// Dir returns the ~/.clawdbot directory path
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".clawdbot"), nil
}

// Load reads configuration from ~/.clawdbot/ config files
func Load() (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	// Read ~/.clawdbot/clawdbot.json for gateway config
	gwData, err := os.ReadFile(filepath.Join(dir, "clawdbot.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read ~/.clawdbot/clawdbot.json: %w", err)
	}
	var gwCfg clawdbotJSON
	if err := json.Unmarshal(gwData, &gwCfg); err != nil {
		return nil, fmt.Errorf("failed to parse ~/.clawdbot/clawdbot.json: %w", err)
	}

	// Read ~/.clawdbot/bridge.json for feishu/bridge config
	brData, err := os.ReadFile(filepath.Join(dir, "bridge.json"))
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read ~/.clawdbot/bridge.json: %w\n\nCreate it with:\n  {\n    \"feishu\": {\n      \"app_id\": \"cli_xxx\",\n      \"app_secret\": \"xxx\"\n    }\n  }", err)
	}
	var brCfg bridgeJSON
	if err := json.Unmarshal(brData, &brCfg); err != nil {
		return nil, fmt.Errorf("failed to parse ~/.clawdbot/bridge.json: %w", err)
	}

	// Validate required fields
	if brCfg.Feishu.AppID == "" {
		return nil, fmt.Errorf("feishu.app_id is required in ~/.clawdbot/bridge.json")
	}
	if brCfg.Feishu.AppSecret == "" {
		return nil, fmt.Errorf("feishu.app_secret is required in ~/.clawdbot/bridge.json")
	}

	// Build config with defaults
	cfg := &Config{
		Feishu: FeishuConfig{
			AppID:               brCfg.Feishu.AppID,
			AppSecret:           brCfg.Feishu.AppSecret,
			ThinkingThresholdMs: 0,
		},
		Clawdbot: ClawdbotConfig{
			GatewayPort:  gwCfg.Gateway.Port,
			GatewayToken: gwCfg.Gateway.Auth.Token,
			AgentID:      "main",
		},
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
