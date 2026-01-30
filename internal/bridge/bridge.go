package bridge

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wy51ai/moltbotCNAPP/internal/clawdbot"
	"github.com/wy51ai/moltbotCNAPP/internal/feishu"
)

// Bridge connects Feishu and ClawdBot
type Bridge struct {
	feishuClient   *feishu.Client
	clawdbotClient *clawdbot.Client
	thinkingMs     int
	seenMessages   *messageCache
}

// messageCache stores seen message IDs to prevent duplicate processing
type messageCache struct {
	cache map[string]time.Time
	mu    sync.RWMutex
	ttl   time.Duration
}

func newMessageCache(ttl time.Duration) *messageCache {
	mc := &messageCache{
		cache: make(map[string]time.Time),
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go mc.cleanup()

	return mc
}

func (mc *messageCache) has(messageID string) bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	_, exists := mc.cache[messageID]
	return exists
}

func (mc *messageCache) add(messageID string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cache[messageID] = time.Now()
}

func (mc *messageCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		mc.mu.Lock()
		now := time.Now()
		for id, timestamp := range mc.cache {
			if now.Sub(timestamp) > mc.ttl {
				delete(mc.cache, id)
			}
		}
		mc.mu.Unlock()
	}
}

// NewBridge creates a new bridge
func NewBridge(feishuClient *feishu.Client, clawdbotClient *clawdbot.Client, thinkingMs int) *Bridge {
	return &Bridge{
		feishuClient:   feishuClient,
		clawdbotClient: clawdbotClient,
		thinkingMs:     thinkingMs,
		seenMessages:   newMessageCache(10 * time.Minute),
	}
}

// SetFeishuClient sets the Feishu client after construction
func (b *Bridge) SetFeishuClient(client *feishu.Client) {
	b.feishuClient = client
}

// HandleMessage processes a message from Feishu
func (b *Bridge) HandleMessage(msg *feishu.Message) error {
	// Check for duplicates
	if msg.MessageID != "" && b.seenMessages.has(msg.MessageID) {
		log.Printf("[Bridge] Skipping duplicate message: %s", msg.MessageID)
		return nil
	}

	// Mark as seen
	if msg.MessageID != "" {
		b.seenMessages.add(msg.MessageID)
	}

	// Clean up message text
	text := msg.Content
	text = removeMentions(text)
	text = strings.TrimSpace(text)

	if text == "" {
		return nil
	}

	// For group chats, check if we should respond
	if msg.ChatType == "group" {
		if !shouldRespondInGroup(text, msg.Mentions) {
			log.Printf("[Bridge] Skipping group message (no trigger): %s", text)
			return nil
		}
	}

	log.Printf("[Bridge] Processing message from %s: %s", msg.ChatID, text)

	// Handle reset command
	if text == "重置" || text == "/reset" {
		// 1. Reset Session Memory
		sessionKey := fmt.Sprintf("feishu:%s", msg.ChatID)
		resetErr := b.clawdbotClient.ResetSession(sessionKey)
		if resetErr != nil {
			log.Printf("[Bridge] Failed to reset session: %v", resetErr)
		}

		// 2. Restart Clawdbot Gateway
		log.Printf("[Bridge] Restarting Clawdbot Gateway...")
		cmd := exec.Command("clawdbot", "gateway", "restart")
		// Ideally set Dir if needed, but default should work if clawdbot is in PATH
		// cmd.Dir = "/home/pk/.clawdbot"
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("[Bridge] Failed to restart gateway: %v, Output: %s", err, string(output))
			b.feishuClient.SendMessage(msg.ChatID, fmt.Sprintf("会话重置失败 (Gateway重启错误): %v", err))
		} else {
			log.Printf("[Bridge] Gateway restarted successfully")
			if resetErr != nil {
				b.feishuClient.SendMessage(msg.ChatID, fmt.Sprintf("Gateway已重启，但会话清除失败: %v。请再试一次。", resetErr))
			} else {
				b.feishuClient.SendMessage(msg.ChatID, "会话已重置，Gateway已重启。您可以重新开始对话。")
			}
		}
		return nil
	}

	// Process asynchronously
	go b.processMessage(msg.ChatID, text)

	return nil
}

func (b *Bridge) processMessage(chatID, text string) {
	var placeholderID string
	var done bool
	var mu sync.Mutex
	var lastUpdate time.Time
	var currentResponse string

	// Show "thinking..." if response takes too long
	var timer *time.Timer
	if b.thinkingMs > 0 {
		timer = time.AfterFunc(time.Duration(b.thinkingMs)*time.Millisecond, func() {
			mu.Lock()
			if done || placeholderID != "" {
				mu.Unlock()
				return
			}
			mu.Unlock()

			msgID, err := b.feishuClient.SendMessage(chatID, "正在思考…")

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				log.Printf("[Bridge] Failed to send thinking message: %v", err)
				return
			}
			// Check if done or placeholder set while we were sending
			if !done && placeholderID == "" {
				placeholderID = msgID
			}
		})
	}

	// Progress handler
	onProgress := func(stream, data string) {
		mu.Lock()

		// Parse data for assistant stream to build response
		if stream == "assistant" {
			var sd clawdbot.StreamData
			if err := json.Unmarshal([]byte(data), &sd); err == nil {
				if sd.Text != "" {
					currentResponse = sd.Text
				} else if sd.Delta != "" {
					currentResponse += sd.Delta
				}
			} else {
				log.Printf("[Bridge] Failed to unmarshal stream data: %v. Data: %s", err, data)
			}
		}

		// Update at most once per second
		if done || time.Since(lastUpdate) < 1000*time.Millisecond {
			mu.Unlock()
			return
		}
		lastUpdate = time.Now()
		pid := placeholderID
		mu.Unlock()

		var statusText string
		switch stream {
		case "assistant":
			if currentResponse != "" {
				statusText = currentResponse + " ▌"
			} else {
				statusText = "正在回复…"
			}
		case "thought":
			statusText = "正在思考…"
		case "tool_call":
			log.Printf("[Bridge] Tool call data: %s", data)
			var tc map[string]interface{}
			if err := json.Unmarshal([]byte(data), &tc); err == nil {
				if name, ok := tc["tool"].(string); ok {
					statusText = fmt.Sprintf("正在使用工具: %s", name)
				} else if name, ok := tc["name"].(string); ok {
					statusText = fmt.Sprintf("正在使用工具: %s", name)
				} else if function, ok := tc["function"].(map[string]interface{}); ok {
					if name, ok := function["name"].(string); ok {
						statusText = fmt.Sprintf("正在使用工具: %s", name)
					}
				} else {
					statusText = "正在调用工具…"
				}
			} else {
				statusText = "正在调用工具…"
			}
		case "tool_result":
			log.Printf("[Bridge] Tool result data: %s", data)
			var tr map[string]interface{}
			if err := json.Unmarshal([]byte(data), &tr); err == nil {
				if name, ok := tr["tool"].(string); ok {
					statusText = fmt.Sprintf("工具 %s 调用完成，正在分析…", name)
				} else if name, ok := tr["name"].(string); ok {
					statusText = fmt.Sprintf("工具 %s 调用完成，正在分析…", name)
				} else {
					statusText = "工具调用完成，正在分析…"
				}
			} else {
				statusText = "工具调用完成，正在分析…"
			}
		}

		if statusText != "" {
			if pid != "" {
				if err := b.feishuClient.UpdateMessage(pid, statusText); err != nil {
					log.Printf("[Bridge] Failed to update message %s: %v", pid, err)
				}
			} else {
				// Create placeholder if not exists
				mu.Lock()
				if placeholderID == "" && !done {
					mu.Unlock()
					msgID, err := b.feishuClient.SendMessage(chatID, statusText)
					mu.Lock()
					if err == nil && placeholderID == "" {
						placeholderID = msgID
					}
					mu.Unlock()
				} else {
					mu.Unlock()
				}
			}
		}
	}

	// Ask ClawdBot
	sessionKey := fmt.Sprintf("feishu:%s", chatID)
	reply, err := b.clawdbotClient.AskClawdbot(text, sessionKey, onProgress)

	// Mark as done
	mu.Lock()
	done = true
	mu.Unlock()

	if timer != nil {
		timer.Stop()
	}

	if err != nil {
		reply = fmt.Sprintf("（系统出错）%v", err)
		log.Printf("[Bridge] Error from ClawdBot: %v", err)
	}

	// Clean up reply
	reply = strings.TrimSpace(reply)
	log.Printf("[Bridge] ClawdBot raw reply: %q", reply)

	// Check for NO_REPLY
	if reply == "" || reply == "NO_REPLY" {
		log.Printf("[Bridge] Received NO_REPLY, sending error message")

		errorMsg := "（AI 服务未返回任何内容，请重试或检查后台日志）"

		// Update placeholder if it exists
		if placeholderID != "" {
			if err := b.feishuClient.UpdateMessage(placeholderID, errorMsg); err != nil {
				log.Printf("[Bridge] Failed to update placeholder with error: %v", err)
			}
		} else {
			// Send new error message
			if _, err := b.feishuClient.SendMessage(chatID, errorMsg); err != nil {
				log.Printf("[Bridge] Failed to send error message: %v", err)
			}
		}
		return
	}

	// Send or update message
	mu.Lock()
	currentPlaceholder := placeholderID
	mu.Unlock()

	if currentPlaceholder != "" {
		// Update existing "thinking..." or streaming message with final reply
		if err := b.feishuClient.UpdateMessage(currentPlaceholder, reply); err != nil {
			log.Printf("[Bridge] Failed to update message, sending new: %v", err)
			// Fall back to sending new message
			if _, err := b.feishuClient.SendMessage(chatID, reply); err != nil {
				log.Printf("[Bridge] Failed to send message: %v", err)
			}
		} else {
			log.Printf("[Bridge] Updated message in %s", chatID)
		}
	} else {
		// Send new message
		if _, err := b.feishuClient.SendMessage(chatID, reply); err != nil {
			log.Printf("[Bridge] Failed to send message: %v", err)
		} else {
			log.Printf("[Bridge] Sent message to %s", chatID)
		}
	}
}

// shouldRespondInGroup determines if the bot should respond in a group chat
func shouldRespondInGroup(text string, mentions []feishu.Mention) bool {
	// Always respond if mentioned
	if len(mentions) > 0 {
		return true
	}

	lowerText := strings.ToLower(text)

	// Question marks
	if strings.HasSuffix(text, "?") || strings.HasSuffix(text, "？") {
		return true
	}

	// English question words
	questionWords := []string{"why", "how", "what", "when", "where", "who", "help"}
	for _, word := range questionWords {
		if regexp.MustCompile(`\b` + word + `\b`).MatchString(lowerText) {
			return true
		}
	}

	// Chinese action verbs
	actionVerbs := []string{
		"帮", "麻烦", "请", "能否", "可以", "解释", "看看",
		"排查", "分析", "总结", "写", "改", "修", "查", "对比", "翻译",
	}
	for _, verb := range actionVerbs {
		if strings.Contains(text, verb) {
			return true
		}
	}

	// Bot names/triggers
	botTriggers := []string{"alen", "clawdbot", "bot", "助手", "智能体"}
	for _, trigger := range botTriggers {
		pattern := fmt.Sprintf(`^%s[\s,:，：]`, trigger)
		if matched, _ := regexp.MatchString(pattern, lowerText); matched {
			return true
		}
	}

	return false
}

// removeMentions removes @mention patterns from text
func removeMentions(text string) string {
	re := regexp.MustCompile(`@_user_\d+\s*`)
	return re.ReplaceAllString(text, "")
}
