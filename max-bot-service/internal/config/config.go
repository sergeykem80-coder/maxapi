package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	MAX         MAXConfig
	OneC        OneCConfig
	Session     SessionConfig
	Log         LogConfig
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port string
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	URL string
}

// MAXConfig holds MAX Bot API configuration
type MAXConfig struct {
	BotToken     string
	WebhookSecret string
}

// OneCConfig holds 1C integration configuration
type OneCConfig struct {
	APIKey string
}

// SessionConfig holds session-related configuration
type SessionConfig struct {
	TTL time.Duration
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Set defaults
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("SESSION_TTL", "24h")

	// Bind environment variables
	viper.AutomaticEnv()

	// Read config file if exists (optional)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")
	
	// Try to read config file but don't fail if it doesn't exist
	_ = viper.ReadInConfig()

	config := &Config{
		Server: ServerConfig{
			Port: viper.GetString("PORT"),
		},
		Database: DatabaseConfig{
			URL: viper.GetString("DATABASE_URL"),
		},
		MAX: MAXConfig{
			BotToken:     viper.GetString("MAX_BOT_TOKEN"),
			WebhookSecret: viper.GetString("WEBHOOK_SECRET"),
		},
		OneC: OneCConfig{
			APIKey: viper.GetString("ONEC_API_KEY"),
		},
		Session: SessionConfig{
			TTL: viper.GetDuration("SESSION_TTL"),
		},
		Log: LogConfig{
			Level: viper.GetString("LOG_LEVEL"),
		},
	}

	// Validate required configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate checks if required configuration values are set
func (c *Config) Validate() error {
	if c.MAX.BotToken == "" {
		return fmt.Errorf("MAX_BOT_TOKEN is required")
	}
	if c.MAX.WebhookSecret == "" {
		return fmt.Errorf("WEBHOOK_SECRET is required")
	}
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.OneC.APIKey == "" {
		return fmt.Errorf("ONEC_API_KEY is required")
	}
	if c.Session.TTL <= 0 {
		return fmt.Errorf("SESSION_TTL must be positive")
	}
	return nil
}

// GetMaxBotToken returns the MAX bot token
func (c *Config) GetMaxBotToken() string {
	return c.MAX.BotToken
}

// GetWebhookSecret returns the webhook secret
func (c *Config) GetWebhookSecret() string {
	return c.MAX.WebhookSecret
}

// GetDatabaseURL returns the database URL
func (c *Config) GetDatabaseURL() string {
	return c.Database.URL
}

// GetOneCAPIKey returns the 1C API key
func (c *Config) GetOneCAPIKey() string {
	return c.OneC.APIKey
}

// GetSessionTTL returns the session TTL
func (c *Config) GetSessionTTL() time.Duration {
	return c.Session.TTL
}

// GetLogLevel returns the log level
func (c *Config) GetLogLevel() string {
	return c.Log.Level
}

// GetServerPort returns the server port
func (c *Config) GetServerPort() string {
	return c.Server.Port
}
