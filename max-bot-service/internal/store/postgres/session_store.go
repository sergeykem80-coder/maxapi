package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session represents a user session
type Session struct {
	Secret       string     `json:"secret"`
	UserID       int64      `json:"user_id"`
	ChatID       int64      `json:"chat_id"`
	Username     string     `json:"username"`
	FirstName    string     `json:"first_name"`
	StartedAt    time.Time  `json:"started_at"`
	LastActivity time.Time  `json:"last_activity"`
	IsActive     bool       `json:"is_active"`
}

// Store provides database operations for sessions
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new session store
func NewStore(ctx context.Context, connString string) (*Store, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	store := &Store{
		pool: pool,
	}

	// Initialize database schema
	if err := store.initSchema(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

// initSchema creates the database schema if it doesn't exist
func (s *Store) initSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS sessions (
		secret VARCHAR(64) PRIMARY KEY,
		user_id BIGINT NOT NULL,
		chat_id BIGINT NOT NULL,
		username VARCHAR(255),
		first_name VARCHAR(255),
		started_at TIMESTAMPTZ DEFAULT NOW(),
		last_activity TIMESTAMPTZ,
		is_active BOOLEAN DEFAULT TRUE
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_secret_active ON sessions(secret, is_active);
	CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_chat_id ON sessions(chat_id);
	`
	_, err := s.pool.Exec(ctx, query)
	return err
}

// CreateSession creates a new session or updates existing one
func (s *Store) CreateSession(ctx context.Context, session *Session) error {
	query := `
	INSERT INTO sessions (secret, user_id, chat_id, username, first_name, started_at, last_activity, is_active)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (secret) DO UPDATE SET
		user_id = EXCLUDED.user_id,
		chat_id = EXCLUDED.chat_id,
		username = EXCLUDED.username,
		first_name = EXCLUDED.first_name,
		started_at = EXCLUDED.started_at,
		last_activity = EXCLUDED.last_activity,
		is_active = EXCLUDED.is_active
	`
	
	now := time.Now()
	session.StartedAt = now
	session.LastActivity = now
	if !session.IsActive {
		session.IsActive = true
	}

	_, err := s.pool.Exec(ctx, query,
		session.Secret,
		session.UserID,
		session.ChatID,
		session.Username,
		session.FirstName,
		session.StartedAt,
		session.LastActivity,
		session.IsActive,
	)
	
	return err
}

// GetSessionBySecret retrieves a session by its secret
func (s *Store) GetSessionBySecret(ctx context.Context, secret string) (*Session, error) {
	query := `
	SELECT secret, user_id, chat_id, username, first_name, started_at, last_activity, is_active
	FROM sessions
	WHERE secret = $1 AND is_active = TRUE
	`
	
	row := s.pool.QueryRow(ctx, query, secret)
	
	session := &Session{}
	err := row.Scan(
		&session.Secret,
		&session.UserID,
		&session.ChatID,
		&session.Username,
		&session.FirstName,
		&session.StartedAt,
		&session.LastActivity,
		&session.IsActive,
	)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	
	return session, nil
}

// GetSessionByUserID retrieves a session by user ID
func (s *Store) GetSessionByUserID(ctx context.Context, userID int64) (*Session, error) {
	query := `
	SELECT secret, user_id, chat_id, username, first_name, started_at, last_activity, is_active
	FROM sessions
	WHERE user_id = $1 AND is_active = TRUE
	ORDER BY started_at DESC
	LIMIT 1
	`
	
	row := s.pool.QueryRow(ctx, query, userID)
	
	session := &Session{}
	err := row.Scan(
		&session.Secret,
		&session.UserID,
		&session.ChatID,
		&session.Username,
		&session.FirstName,
		&session.StartedAt,
		&session.LastActivity,
		&session.IsActive,
	)
	
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	
	return session, nil
}

// UpdateLastActivity updates the last activity timestamp for a session
func (s *Store) UpdateLastActivity(ctx context.Context, secret string) error {
	query := `
	UPDATE sessions
	SET last_activity = NOW()
	WHERE secret = $1
	`
	
	_, err := s.pool.Exec(ctx, query, secret)
	return err
}

// DeactivateSession deactivates a session
func (s *Store) DeactivateSession(ctx context.Context, secret string) error {
	query := `
	UPDATE sessions
	SET is_active = FALSE
	WHERE secret = $1
	`
	
	_, err := s.pool.Exec(ctx, query, secret)
	return err
}

// Close closes the database connection pool
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// ErrSessionNotFound is returned when a session is not found
var ErrSessionNotFound = errors.New("session not found")

// HealthCheck checks if the database connection is healthy
func (s *Store) HealthCheck(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
