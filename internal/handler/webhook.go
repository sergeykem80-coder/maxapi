package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/sergeykem80-maxapi/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WebhookRequest represents the incoming webhook request from MAX Bot API
type WebhookRequest struct {
	UpdateID int64  `json:"update_id"`
	Type     string `json:"type"`
	Sender   struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
	} `json:"sender"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Payload struct {
		Start string `json:"start"`
	} `json:"payload"`
}

// WebhookHandler handles MAX Bot webhook requests
type WebhookHandler struct {
	sessionService *service.SessionService
	webhookSecret  string
	logger         *zap.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(sessionService *service.SessionService, webhookSecret string, logger *zap.Logger) *WebhookHandler {
	return &WebhookHandler{
		sessionService: sessionService,
		webhookSecret:  webhookSecret,
		logger:         logger,
	}
}

// HandleWebhook processes incoming webhook requests from MAX
// POST /webhook
// Headers: X-Max-Bot-Api-Secret: {WEBHOOK_SECRET}
func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	startTime := time.Now()
	
	// Validate webhook secret from header
	secret := c.GetHeader("X-Max-Bot-Api-Secret")
	if secret == "" {
		h.logger.Warn("webhook request missing secret header",
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "missing X-Max-Bot-Api-Secret header",
		})
		return
	}

	if !h.validateSecret(secret) {
		h.logger.Warn("webhook request with invalid secret",
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "invalid secret",
		})
		return
	}

	// Parse request body
	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("failed to parse webhook request",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request body",
		})
		return
	}

	// Only process bot_started events
	if req.Type != "bot_started" {
		h.logger.Debug("ignoring non-bot_started event",
			zap.String("type", req.Type))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "event ignored",
		})
		return
	}

	// Extract secret from payload.start (format: "secret_{VALUE}")
	if req.Payload.Start == "" {
		h.logger.Warn("bot_started event missing payload.start")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "missing payload.start",
		})
		return
	}

	extractedSecret := h.extractSecret(req.Payload.Start)
	if extractedSecret == "" {
		h.logger.Warn("invalid secret format in payload.start",
			zap.String("payload_start", req.Payload.Start))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid secret format",
		})
		return
	}

	h.logger.Info("processing bot_started event",
		zap.Int64("user_id", req.Sender.ID),
		zap.Int64("chat_id", req.Chat.ID),
		zap.String("username", req.Sender.Username),
		zap.String("secret", extractedSecret))

	// Create session in database
	ctx := c.Request.Context()
	if err := h.sessionService.CreateSession(
		ctx,
		extractedSecret,
		req.Sender.ID,
		req.Chat.ID,
		req.Sender.Username,
		req.Sender.FirstName,
	); err != nil {
		h.logger.Error("failed to create session",
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "failed to save session",
		})
		return
	}

	// Log processing time
	processingTime := time.Since(startTime)
	h.logger.Debug("webhook processed successfully",
		zap.Duration("processing_time", processingTime),
		zap.String("secret", extractedSecret))

	// Respond with 200 OK (must be <30 sec)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "session created",
	})
}

// validateSecret checks if the provided secret matches the configured webhook secret
func (h *WebhookHandler) validateSecret(secret string) bool {
	return secret == h.webhookSecret
}

// extractSecret extracts the secret value from the payload.start field
// Expected format: "secret_{VALUE}" -> returns "{VALUE}"
func (h *WebhookHandler) extractSecret(payloadStart string) string {
	const prefix = "secret_"
	if strings.HasPrefix(payloadStart, prefix) {
		return strings.TrimPrefix(payloadStart, prefix)
	}
	// If no prefix, return as-is (fallback)
	return payloadStart
}

// HealthCheck returns health status of the service
func (h *WebhookHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
