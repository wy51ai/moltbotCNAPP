package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wy51ai/moltbotCNAPP/internal/bridge"
	"github.com/wy51ai/moltbotCNAPP/internal/clawdbot"
	"github.com/wy51ai/moltbotCNAPP/internal/config"
	"github.com/wy51ai/moltbotCNAPP/internal/feishu"
)

func main() {
	cmd := "run"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "start":
		applyConfigArgs(os.Args[2:])
		cmdStart()
	case "stop":
		cmdStop()
	case "status":
		cmdStatus()
	case "restart":
		applyConfigArgs(os.Args[2:])
		dir, _ := config.Dir()
		pidPath := filepath.Join(dir, "bridge.pid")
		if pid, err := readPID(pidPath); err == nil {
			stopProcess(pid)
			for i := 0; i < 10; i++ {
				time.Sleep(200 * time.Millisecond)
				if !isProcessRunning(pid) {
					break
				}
			}
			os.Remove(pidPath)
		}
		cmdStart()
	case "run":
		if len(os.Args) > 2 {
			applyConfigArgs(os.Args[2:])
		}
		cmdRun()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\nUsage:\n  clawdbot-bridge start [fs_app_id=xxx fs_app_secret=yyy]\n  clawdbot-bridge stop\n  clawdbot-bridge status\n  clawdbot-bridge restart\n  clawdbot-bridge run\n")
		os.Exit(1)
	}
}

func cmdStart() {
	dir, err := config.Dir()
	if err != nil {
		log.Fatal(err)
	}

	pidPath := filepath.Join(dir, "bridge.pid")
	logPath := filepath.Join(dir, "bridge.log")

	// Check if already running
	if isRunning(pidPath) {
		fmt.Println("Already running")
		os.Exit(1)
	}

	// Validate config before daemonizing so errors are visible
	if _, err := config.Load(); err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// Open log file
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file %s: %v", logPath, err)
	}
	defer logFile.Close()

	// Use /dev/null for stdin
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

	// Re-exec self as daemon
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	p, err := os.StartProcess(exe, []string{exe, "run"}, &os.ProcAttr{
		Files: []*os.File{devNull, logFile, logFile},
		Sys:   daemonSysProcAttr(),
	})
	if err != nil {
		log.Fatalf("Failed to start daemon: %v", err)
	}

	pid := p.Pid

	// Write PID file
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		p.Kill()
		log.Fatalf("Failed to write PID file: %v", err)
	}

	p.Release()
	fmt.Printf("Started (PID %d), log: %s\n", pid, logPath)
}

func cmdStop() {
	dir, err := config.Dir()
	if err != nil {
		log.Fatal(err)
	}

	pidPath := filepath.Join(dir, "bridge.pid")
	pid, err := readPID(pidPath)
	if err != nil {
		fmt.Println("Not running")
		os.Exit(1)
	}

	if err := stopProcess(pid); err != nil {
		fmt.Println("Not running")
		os.Remove(pidPath)
		os.Exit(1)
	}

	for i := 0; i < 10; i++ {
		time.Sleep(200 * time.Millisecond)
		if !isProcessRunning(pid) {
			break
		}
	}

	os.Remove(pidPath)
	fmt.Println("Stopped")
}

func cmdStatus() {
	dir, err := config.Dir()
	if err != nil {
		log.Fatal(err)
	}

	pidPath := filepath.Join(dir, "bridge.pid")
	if isRunning(pidPath) {
		pid, _ := readPID(pidPath)
		fmt.Printf("Running (PID %d)\n", pid)
	} else {
		fmt.Println("Not running")
		os.Exit(1)
	}
}

func cmdRun() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[Main] Starting ClawdBot Bridge...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[Main] Failed to load config: %v", err)
	}

	log.Printf("[Main] Loaded config: AppID=%s, Gateway=127.0.0.1:%d, AgentID=%s",
		cfg.Feishu.AppID, cfg.Clawdbot.GatewayPort, cfg.Clawdbot.AgentID)

	clawdbotClient := clawdbot.NewClient(
		cfg.Clawdbot.GatewayPort,
		cfg.Clawdbot.GatewayToken,
		cfg.Clawdbot.AgentID,
	)

	bridgeInstance := bridge.NewBridge(nil, clawdbotClient, cfg.Feishu.ThinkingThresholdMs, cfg.Feishu.AllowUsers, cfg.Feishu.AllowChats)

	feishuClient := feishu.NewClient(
		cfg.Feishu.AppID,
		cfg.Feishu.AppSecret,
		bridgeInstance.HandleMessage,
	)

	bridgeInstance.SetFeishuClient(feishuClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		if err := feishuClient.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	log.Println("[Main] ClawdBot Bridge started successfully")
	log.Println("[Main] Press Ctrl+C to stop")

	select {
	case <-sigChan:
		log.Println("[Main] Received shutdown signal, stopping...")
		cancel()
	case err := <-errChan:
		log.Printf("[Main] Error: %v", err)
		cancel()
	}

	log.Println("[Main] ClawdBot Bridge stopped")
}

func isRunning(pidPath string) bool {
	pid, err := readPID(pidPath)
	if err != nil {
		return false
	}
	return isProcessRunning(pid)
}

func readPID(pidPath string) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// applyConfigArgs parses key=value args and saves to bridge.json
func applyConfigArgs(args []string) {
	kv := parseKeyValue(args)
	appID := kv["fs_app_id"]
	appSecret := kv["fs_app_secret"]

	if appID == "" && appSecret == "" {
		return
	}

	dir, err := config.Dir()
	if err != nil {
		log.Fatal(err)
	}

	// Read existing config if present (try bridge.json first)
	var cfg bridgeConfigJSON
	if data, err := os.ReadFile(filepath.Join(dir, "bridge.json")); err == nil {
		json.Unmarshal(data, &cfg)
	}

	if appID != "" {
		cfg.Feishu.AppID = appID
	}
	if appSecret != "" {
		cfg.Feishu.AppSecret = appSecret
	}
	if v, ok := kv["agent_id"]; ok {
		cfg.AgentID = v
	}
	if v, ok := kv["thinking_ms"]; ok {
		if ms, err := strconv.Atoi(v); err == nil {
			cfg.ThinkingThresholdMs = ms
		}
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	path := filepath.Join(dir, "bridge.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}
	fmt.Printf("Saved config to %s\n", path)
}

type bridgeConfigJSON struct {
	Feishu struct {
		AppID     string `json:"app_id"`
		AppSecret string `json:"app_secret"`
	} `json:"feishu"`
	ThinkingThresholdMs int    `json:"thinking_threshold_ms,omitempty"`
	AgentID             string `json:"agent_id,omitempty"`
}

func parseKeyValue(args []string) map[string]string {
	result := make(map[string]string)
	for _, arg := range args {
		if parts := strings.SplitN(arg, "=", 2); len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
