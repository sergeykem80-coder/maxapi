package service

import (
	"context"
	"fmt"
	"time"

	"github.com/example/max-bot-service/internal/maxclient"
	"github.com/example/max-bot-service/internal/store/postgres"
	"go.uber.org/zap"
)

// SessionService handles session business logic
type SessionService struct {
	store  *postgres.Store
	logger *zap.Logger
	ttl    time.Duration
}

// NewSessionService creates a new session service
func NewSessionService(store *postgres.Store, logger *zap.Logger, ttl time.Duration) *SessionService {
	return &SessionService{
		store:  store,
		logger: logger,
		ttl:    ttl,
	}
}

// CreateSession creates a new session from webhook data
func (s *SessionService) CreateSession(ctx context.Context, secret string, userID, chatID int64, username, firstName string) error {
	session := &postgres.Session{
		Secret:    secret,
		UserID:    userID,
		ChatID:    chatID,
		Username:  username,
		FirstName: firstName,
		IsActive:  true,
	}

	if err := s.store.CreateSession(ctx, session); err != nil {
		s.logger.Error("failed to create session",
			zap.String("secret", secret),
			zap.Int64("user_id", userID),
			zap.Error(err))
		return fmt.Errorf("create session: %w", err)
	}

	s.logger.Info("session created successfully",
		zap.String("secret", secret),
		zap.Int64("user_id", userID),
		zap.Int64("chat_id", chatID))

	return nil
}

// GetSession retrieves a session by secret
func (s *SessionService) GetSession(ctx context.Context, secret string) (*postgres.Session, error) {
	session, err := s.store.GetSessionBySecret(ctx, secret)
	if err != nil {
		if err == postgres.ErrSessionNotFound {
			return nil, nil
		}
		return nil, err
	}

	// Check if session has expired
	if time.Since(session.StartedAt) > s.ttl {
		s.logger.Debug("session expired",
			zap.String("secret", secret),
			zap.Time("started_at", session.StartedAt))
		
		// Deactivate expired session
		if err := s.store.DeactivateSession(ctx, secret); err != nil {
			s.logger.Warn("failed to deactivate expired session",
				zap.String("secret", secret),
				zap.Error(err))
		}
		
		return nil, nil
	}

	return session, nil
}

// UpdateActivity updates the last activity timestamp
func (s *SessionService) UpdateActivity(ctx context.Context, secret string) error {
	return s.store.UpdateLastActivity(ctx, secret)
}

// MessageService handles message sending logic
type MessageService struct {
	maxClient *maxclient.Client
	store     *postgres.Store
	logger    *zap.Logger
}

// NewMessageService creates a new message service
func NewMessageService(maxClient *maxclient.Client, store *postgres.Store, logger *zap.Logger) *MessageService {
	return &MessageService{
		maxClient: maxClient,
		store:     store,
		logger:    logger,
	}
}

// SendToUser sends a message to a user identified by secret
func (m *MessageService) SendToUser(ctx context.Context, secret, text string) (string, time.Time, error) {
	// Get session
	session, err := m.store.GetSessionBySecret(ctx, secret)
	if err != nil {
		if err == postgres.ErrSessionNotFound {
			return "", time.Time{}, fmt.Errorf("session not found")
		}
		return "", time.Time{}, fmt.Errorf("get session: %w", err)
	}

	if !session.IsActive {
		return "", time.Time{}, fmt.Errorf("session is not active")
	}

	// Send message via MAX API
	messageID, err := m.maxClient.SendMessage(ctx, session.ChatID, text)
	if err != nil {
		m.logger.Error("failed to send message",
			zap.String("secret", secret),
			zap.Int64("chat_id", session.ChatID),
			zap.Error(err))
		return "", time.Time{}, fmt.Errorf("send message: %w", err)
	}

	deliveredAt := time.Now()

	// Update last activity
	if err := m.store.UpdateLastActivity(ctx, secret); err != nil {
		m.logger.Warn("failed to update last activity",
			zap.String("secret", secret),
			zap.Error(err))
	}

	m.logger.Info("message sent successfully",
		zap.String("secret", secret),
		zap.String("message_id", messageID),
		zap.Time("delivered_at", deliveredAt))

	return messageID, deliveredAt, nil
}

// ValidateMessage validates message text
func (m *MessageService) ValidateMessage(text string) error {
	if text == "" {
		return fmt.Errorf("message text cannot be empty")
	}
	if len(text) > 4000 {
		return fmt.Errorf("message text exceeds 4000 characters limit")
	}
	return nil
}
