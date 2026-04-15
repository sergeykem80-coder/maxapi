package handler

import (
	"net/http"
	"time"

	"github.com/example/max-bot-service/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OneCAPIHandler handles API requests from 1C
type OneCAPIHandler struct {
	sessionService *service.SessionService
	messageService *service.MessageService
	oneCAPIKey     string
	logger         *zap.Logger
}

// NewOneCAPIHandler creates a new 1C API handler
func NewOneCAPIHandler(
	sessionService *service.SessionService,
	messageService *service.MessageService,
	oneCAPIKey string,
	logger *zap.Logger,
) *OneCAPIHandler {
	return &OneCAPIHandler{
		sessionService: sessionService,
		messageService: messageService,
		oneCAPIKey:     oneCAPIKey,
		logger:         logger,
	}
}

// SessionResponse represents the response for GET /api/v1/session
type SessionResponse struct {
	Success bool              `json:"success"`
	Data    *SessionData      `json:"data,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// SessionData represents session data in the response
type SessionData struct {
	Secret    string    `json:"secret"`
	UserID    int64     `json:"user_id"`
	ChatID    int64     `json:"chat_id"`
	Username  string    `json:"username"`
	FirstName string    `json:"first_name"`
	StartedAt time.Time `json:"started_at"`
	IsActive  bool      `json:"is_active"`
}

// NotifyRequest represents the request for POST /api/v1/notify
type NotifyRequest struct {
	Secret   string `json:"secret"`
	Text     string `json:"text"`
	Priority string `json:"priority"` // "normal" or "urgent"
}

// NotifyResponse represents the response for POST /api/v1/notify
type NotifyResponse struct {
	Success    bool      `json:"success"`
	MessageID  string    `json:"message_id,omitempty"`
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// GetSession retrieves session data by secret
// GET /api/v1/session?secret={SECRET}
// Headers: Authorization: Bearer {ONEC_API_KEY}
func (h *OneCAPIHandler) GetSession(c *gin.Context) {
	// Validate Bearer token
	if !h.validateBearerToken(c) {
		h.logger.Warn("unauthorized session request",
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusUnauthorized, SessionResponse{
			Success: false,
			Error:   "unauthorized",
		})
		return
	}

	// Get secret from query parameter
	secret := c.Query("secret")
	if secret == "" {
		h.logger.Warn("session request missing secret parameter")
		c.JSON(http.StatusBadRequest, SessionResponse{
			Success: false,
			Error:   "missing secret parameter",
		})
		return
	}

	h.logger.Debug("getting session",
		zap.String("secret", secret))

	// Get session from service
	ctx := c.Request.Context()
	session, err := h.sessionService.GetSession(ctx, secret)
	if err != nil {
		h.logger.Error("failed to get session",
			zap.String("secret", secret),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, SessionResponse{
			Success: false,
			Error:   "internal server error",
		})
		return
	}

	if session == nil {
		h.logger.Info("session not found",
			zap.String("secret", secret))
		c.JSON(http.StatusNotFound, SessionResponse{
			Success: false,
			Error:   "session not found",
		})
		return
	}

	response := SessionResponse{
		Success: true,
		Data: &SessionData{
			Secret:    session.Secret,
			UserID:    session.UserID,
			ChatID:    session.ChatID,
			Username:  session.Username,
			FirstName: session.FirstName,
			StartedAt: session.StartedAt,
			IsActive:  session.IsActive,
		},
	}

	c.JSON(http.StatusOK, response)
}

// SendNotification sends a notification to a user
// POST /api/v1/notify
// Headers: Authorization: Bearer {ONEC_API_KEY}
func (h *OneCAPIHandler) SendNotification(c *gin.Context) {
	// Validate Bearer token
	if !h.validateBearerToken(c) {
		h.logger.Warn("unauthorized notify request",
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusUnauthorized, NotifyResponse{
			Success: false,
			Error:   "unauthorized",
		})
		return
	}

	// Parse request body
	var req NotifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("failed to parse notify request",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, NotifyResponse{
			Success: false,
			Error:   "invalid request body",
		})
		return
	}

	// Validate request
	if req.Secret == "" {
		h.logger.Warn("notify request missing secret")
		c.JSON(http.StatusBadRequest, NotifyResponse{
			Success: false,
			Error:   "missing secret",
		})
		return
	}

	if req.Text == "" {
		h.logger.Warn("notify request missing text")
		c.JSON(http.StatusBadRequest, NotifyResponse{
			Success: false,
			Error:   "missing text",
		})
		return
	}

	// Validate priority (optional, default to "normal")
	if req.Priority != "" && req.Priority != "normal" && req.Priority != "urgent" {
		h.logger.Warn("notify request with invalid priority",
			zap.String("priority", req.Priority))
		c.JSON(http.StatusBadRequest, NotifyResponse{
			Success: false,
			Error:   "invalid priority, must be 'normal' or 'urgent'",
		})
		return
	}

	h.logger.Info("sending notification",
		zap.String("secret", req.Secret),
		zap.Int("text_length", len(req.Text)),
		zap.String("priority", req.Priority))

	// Validate message text
	if err := h.messageService.ValidateMessage(req.Text); err != nil {
		h.logger.Warn("invalid message text",
			zap.Error(err))
		c.JSON(http.StatusBadRequest, NotifyResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Send message via service
	ctx := c.Request.Context()
	messageID, deliveredAt, err := h.messageService.SendToUser(ctx, req.Secret, req.Text)
	if err != nil {
		h.logger.Error("failed to send notification",
			zap.String("secret", req.Secret),
			zap.Error(err))
		
		errorMsg := err.Error()
		statusCode := http.StatusInternalServerError
		
		if errorMsg == "session not found" {
			statusCode = http.StatusNotFound
		} else if errorMsg == "session is not active" {
			statusCode = http.StatusForbidden
		}
		
		c.JSON(statusCode, NotifyResponse{
			Success: false,
			Error:   errorMsg,
		})
		return
	}

	response := NotifyResponse{
		Success:    true,
		MessageID:  messageID,
		DeliveredAt: deliveredAt,
	}

	h.logger.Info("notification sent successfully",
		zap.String("secret", req.Secret),
		zap.String("message_id", messageID))

	c.JSON(http.StatusOK, response)
}

// validateBearerToken checks if the Authorization header contains a valid Bearer token
func (h *OneCAPIHandler) validateBearerToken(c *gin.Context) bool {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return false
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) <= len(bearerPrefix) {
		return false
	}

	token := authHeader[len(bearerPrefix):]
	return token == h.oneCAPIKey
}
