package bridge

import (
	"encoding/base64"
	"fmt"
	"log"
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

	// Prepare attachments
	var attachments []*clawdbot.Attachment

	// Handle image messages
	if msg.MsgType == "image" && len(msg.ImageData) > 0 {
		// If it's just an image without text, add a default prompt
		if text == "" || text == "[图片]" {
			text = "请描述这张图片"
		}

		// Create image attachment
		attachment := &clawdbot.Attachment{
			MimeType: "image/png", // Feishu images are typically PNG
			Content:  base64.StdEncoding.EncodeToString(msg.ImageData),
		}
		attachments = append(attachments, attachment)

		log.Printf("[Bridge] Processing image message from %s (size: %d bytes)", msg.ChatID, len(msg.ImageData))
	} else {
		// Text message
		if text == "" {
			return nil
		}
		log.Printf("[Bridge] Processing message from %s: %s", msg.ChatID, text)
	}

	// For group chats, check if we should respond
	if msg.ChatType == "group" {
		// Always respond to images
		if msg.MsgType != "image" && !shouldRespondInGroup(text, msg.Mentions) {
			log.Printf("[Bridge] Skipping group message (no trigger): %s", text)
			return nil
		}
	}

	// Process asynchronously
	go b.processMessageWithAttachments(msg.ChatID, text, attachments)

	return nil
}

func (b *Bridge) processMessageWithAttachments(chatID, text string, attachments []*clawdbot.Attachment) {
	var placeholderID string
	var done bool
	var mu sync.Mutex

	// Show "thinking..." if response takes too long
	var timer *time.Timer
	if b.thinkingMs > 0 {
		timer = time.AfterFunc(time.Duration(b.thinkingMs)*time.Millisecond, func() {
			mu.Lock()
			defer mu.Unlock()

			if done {
				return
			}

			msgID, err := b.feishuClient.SendMessage(chatID, "正在思考…")
			if err != nil {
				log.Printf("[Bridge] Failed to send thinking message: %v", err)
				return
			}
			placeholderID = msgID
		})
	}

	// Ask ClawdBot
	sessionKey := fmt.Sprintf("feishu:%s", chatID)
	var reply string
	var err error

	if len(attachments) > 0 {
		reply, err = b.clawdbotClient.AskClawdbotWithAttachments(text, sessionKey, attachments)
	} else {
		reply, err = b.clawdbotClient.AskClawdbot(text, sessionKey)
	}

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
		log.Printf("[Bridge] Received NO_REPLY, not sending message")

		// Delete thinking placeholder if it exists
		if placeholderID != "" {
			if err := b.feishuClient.DeleteMessage(placeholderID); err != nil {
				log.Printf("[Bridge] Failed to delete placeholder: %v", err)
			}
		}
		return
	}

	// Send or update message
	mu.Lock()
	currentPlaceholder := placeholderID
	mu.Unlock()

	if currentPlaceholder != "" {
		// Update existing "thinking..." message
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
