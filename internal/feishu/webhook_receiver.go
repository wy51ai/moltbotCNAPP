package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ErrBadRequest is returned when the request payload is invalid
var ErrBadRequest = errors.New("bad request")

// Prometheus metrics for webhook receiver
var (
	webhookRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "feishu_webhook_requests_total",
			Help: "Total number of webhook requests by status",
		},
		[]string{"status"},
	)
	webhookRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "feishu_webhook_request_duration_seconds",
			Help:    "Histogram of webhook request durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)
	workerQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "feishu_worker_queue_depth",
			Help: "Current number of jobs waiting in the worker queue",
		},
	)
	workerQueueCapacity = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "feishu_worker_queue_capacity",
			Help: "Maximum capacity of the worker queue",
		},
	)
)

func init() {
	prometheus.MustRegister(webhookRequestsTotal)
	prometheus.MustRegister(webhookRequestDuration)
	prometheus.MustRegister(workerQueueDepth)
	prometheus.MustRegister(workerQueueCapacity)
}

// WebhookConfig defines configuration for the webhook receiver
type WebhookConfig struct {
	Port              int    // Server port, default 8080
	Path              string // Webhook path, default "/webhook/feishu"
	VerificationToken string // Required: token for challenge verification
	EncryptKey        string // Required: key for message decryption
	Workers           int    // Number of workers, default 10
	QueueSize         int    // Queue size, default 100
}

// WebhookReceiver receives messages via HTTP webhook and implements FeishuReceiver
type WebhookReceiver struct {
	config      WebhookConfig
	handler     MessageHandler
	workerPool  *WorkerPool
	dedupeCache sync.Map // event_id -> time.Time
	server      *http.Server
	dispatcher  *dispatcher.EventDispatcher
}

// Interface compliance check
var _ FeishuReceiver = (*WebhookReceiver)(nil)

// NewWebhookReceiver creates a new WebhookReceiver with the given config and handler.
// Panics if VerificationToken or EncryptKey is empty.
func NewWebhookReceiver(config WebhookConfig, handler MessageHandler) *WebhookReceiver {
	// Validate required fields
	if config.VerificationToken == "" {
		panic("WebhookConfig.VerificationToken is required")
	}
	if config.EncryptKey == "" {
		panic("WebhookConfig.EncryptKey is required")
	}

	// Set defaults
	if config.Port == 0 {
		config.Port = 8080
	}
	if config.Path == "" {
		config.Path = "/webhook/feishu"
	}
	if config.Workers == 0 {
		config.Workers = 10
	}
	if config.QueueSize == 0 {
		config.QueueSize = 100
	}

	// Set queue capacity metric
	workerQueueCapacity.Set(float64(config.QueueSize))

	return &WebhookReceiver{
		config:     config,
		handler:    handler,
		workerPool: NewWorkerPool(config.Workers, config.QueueSize),
	}
}

// Start starts the webhook receiver and blocks until ctx is done.
// It starts the worker pool, HTTP server, and dedupe cache cleanup goroutine.
func (wr *WebhookReceiver) Start(ctx context.Context) error {
	// Start worker pool
	wr.workerPool.Start()

	// Start dedupe cache cleanup goroutine
	go wr.cleanupDedupeCache(ctx)

	// Start queue depth metrics updater
	go wr.updateQueueDepthMetrics(ctx)

	// Create SDK EventDispatcher with VerificationToken and EncryptKey
	// SDK automatically handles challenge verification and message decryption
	wr.dispatcher = dispatcher.NewEventDispatcher(
		wr.config.VerificationToken,
		wr.config.EncryptKey,
	).OnP2MessageReceiveV1(func(eventCtx context.Context, event *larkim.P2MessageReceiveV1) error {
		return wr.handleMessageEvent(event)
	})

	// Create HTTP mux with custom handler for proper error code mapping
	mux := http.NewServeMux()
	mux.HandleFunc(wr.config.Path, wr.webhookHandler)
	mux.HandleFunc("/health", wr.healthHandler)
	mux.Handle("/metrics", promhttp.Handler())

	// Configure HTTP server with timeouts
	wr.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", wr.config.Port),
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start graceful shutdown goroutine
	go wr.gracefulShutdown(ctx)

	log.Printf("[Webhook] Starting server on :%d (webhook=%s, health=/health, metrics=/metrics, workers=%d, queue=%d)",
		wr.config.Port, wr.config.Path, wr.config.Workers, wr.config.QueueSize)

	// Start HTTP server (blocks)
	if err := wr.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("webhook server error: %w", err)
	}

	return nil
}

// handleMessageEvent processes a message event from the SDK dispatcher.
// It performs deduplication and submits the job to the worker pool.
func (wr *WebhookReceiver) handleMessageEvent(event *larkim.P2MessageReceiveV1) error {
	// Nil check for header
	if event.EventV2Base == nil || event.EventV2Base.Header == nil {
		log.Printf("[Webhook] Event header is nil")
		return ErrBadRequest
	}

	// EventID is a string (not *string) in EventHeader
	eventID := event.EventV2Base.Header.EventID
	if eventID == "" {
		log.Printf("[Webhook] Event ID is empty")
		return ErrBadRequest
	}

	// Check dedupe cache
	if _, exists := wr.dedupeCache.LoadOrStore(eventID, time.Now()); exists {
		log.Printf("[Webhook] Duplicate event ignored: %s", eventID)
		return nil
	}

	// Convert event to Message
	msg := wr.convertEventToMessage(event)

	// Create job with handler
	job := Job{
		EventID: eventID,
		Handler: func() error {
			return wr.handler(msg)
		},
	}

	// Submit to worker pool
	if err := wr.workerPool.Submit(job); err != nil {
		if errors.Is(err, ErrQueueFull) {
			webhookRequestsTotal.WithLabelValues("rejected").Inc()
		} else {
			webhookRequestsTotal.WithLabelValues("error").Inc()
		}
		log.Printf("[Webhook] Queue full, event %s will be retried", eventID)
		return err // Will be mapped to 503 in webhookHandler
	}

	webhookRequestsTotal.WithLabelValues("success").Inc()
	return nil
}

// webhookHandler is the HTTP handler that implements custom error code mapping.
// It does NOT use httpserverext for full control over response codes.
func (wr *WebhookReceiver) webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Record request duration
	start := time.Now()
	defer func() {
		webhookRequestDuration.WithLabelValues("webhook").Observe(time.Since(start).Seconds())
	}()

	// Method check
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Body size limit (1MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Construct EventReq with the read body
	eventReq := &larkevent.EventReq{
		Header:     r.Header,
		Body:       body,
		RequestURI: r.RequestURI,
	}

	// Handle challenge request (url_verification)
	// Check if this is a challenge request before signature verification
	var challengeReq struct {
		Type      string `json:"type"`
		Token     string `json:"token"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(body, &challengeReq); err == nil && challengeReq.Type == "url_verification" {
		// For challenge, verify token matches
		if challengeReq.Token != wr.config.VerificationToken {
			log.Printf("[Webhook] Challenge token mismatch from %s", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Return challenge response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]string{"challenge": challengeReq.Challenge}
		_ = json.NewEncoder(w).Encode(resp)
		log.Printf("[Webhook] Challenge verified successfully")
		return
	}

	// Use SDK's Handle method which does:
	// 1. ParseReq - extract encrypted message
	// 2. DecryptEvent - decrypt if needed
	// 3. VerifySign - verify signature (after parsing to check reqType)
	// 4. DoHandle - dispatch to registered handler
	resp := wr.dispatcher.Handle(r.Context(), eventReq)

	// The SDK's Handle method returns 500 for all handler errors.
	// We need to check if our handler set an error type in the last processed error.
	// Since we can't modify SDK behavior, we use a workaround:
	// - For queue full: handler returns ErrQueueFull, SDK returns 500, we map it
	// - For signature failure: SDK returns 500 with specific message

	// Check response status and body to determine error type
	if resp.StatusCode == http.StatusInternalServerError {
		bodyStr := string(resp.Body)
		// Check for signature verification failure
		if contains(bodyStr, "signature verification failed") {
			log.Printf("[Webhook] Signature verification failed from %s", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Check for queue full error (our custom error)
		if contains(bodyStr, ErrQueueFull.Error()) {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		// Check for bad request error
		if contains(bodyStr, ErrBadRequest.Error()) {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		// Other errors pass through as 500
		log.Printf("[Webhook] Event handling error: %s", bodyStr)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Success - return response from SDK
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}

// contains checks if s contains substr (case-insensitive would be better but keep simple)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// convertEventToMessage converts a Feishu SDK message event to our Message type.
// This mirrors the logic in ws_receiver.go handleMessage.
func (wr *WebhookReceiver) convertEventToMessage(event *larkim.P2MessageReceiveV1) *Message {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return &Message{}
	}

	msg := event.Event.Message

	// Only handle text messages
	if msg.MessageType == nil || *msg.MessageType != "text" {
		return &Message{}
	}

	if msg.Content == nil {
		return &Message{}
	}

	// Parse message content
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*msg.Content), &content); err != nil {
		log.Printf("[Webhook] Failed to parse message content: %v", err)
		return &Message{}
	}

	// Build message
	message := &Message{
		MessageID: getStringValue(msg.MessageId),
		ChatID:    getStringValue(msg.ChatId),
		ChatType:  getStringValue(msg.ChatType),
		Content:   content.Text,
	}

	// Parse mentions
	if msg.Mentions != nil {
		for _, mention := range msg.Mentions {
			mentionID := ""
			if mention.Id != nil && mention.Id.UserId != nil {
				mentionID = *mention.Id.UserId
			}
			message.Mentions = append(message.Mentions, Mention{
				Key:       getStringValue(mention.Key),
				ID:        mentionID,
				Name:      getStringValue(mention.Name),
				TenantKey: getStringValue(mention.TenantKey),
			})
		}
	}

	return message
}

// gracefulShutdown handles graceful shutdown when context is cancelled
func (wr *WebhookReceiver) gracefulShutdown(ctx context.Context) {
	<-ctx.Done()
	log.Printf("[Webhook] Shutting down gracefully...")

	// Shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := wr.server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Webhook] Server shutdown error: %v", err)
	}

	// Shutdown worker pool with timeout
	if err := wr.workerPool.Shutdown(30 * time.Second); err != nil {
		log.Printf("[Webhook] Worker pool shutdown error: %v", err)
	}

	log.Printf("[Webhook] Shutdown complete")
}

// cleanupDedupeCache periodically removes old entries from the dedupe cache
func (wr *WebhookReceiver) cleanupDedupeCache(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			wr.dedupeCache.Range(func(key, value interface{}) bool {
				if ts, ok := value.(time.Time); ok {
					if now.Sub(ts) > 10*time.Minute {
						wr.dedupeCache.Delete(key)
					}
				}
				return true
			})
		}
	}
}

// healthHandler returns the health status of the webhook receiver
func (wr *WebhookReceiver) healthHandler(w http.ResponseWriter, r *http.Request) {
	queueDepth := wr.workerPool.QueueLen()
	queueCapacity := wr.config.QueueSize

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string]interface{}{
		"status":         "ok",
		"queue_depth":    queueDepth,
		"queue_capacity": queueCapacity,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// updateQueueDepthMetrics periodically updates the queue depth gauge
func (wr *WebhookReceiver) updateQueueDepthMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			workerQueueDepth.Set(float64(wr.workerPool.QueueLen()))
		}
	}
}
