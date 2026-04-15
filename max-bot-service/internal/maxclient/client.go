package maxclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client wraps the MAX Bot API client with additional functionality
type Client struct {
	httpClient *http.Client
	token      string
	logger     *zap.Logger
	baseURL    string
}

// NewClient creates a new MAX Bot API client
func NewClient(token string, logger *zap.Logger) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:   token,
		logger:  logger,
		baseURL: "https://api.max.ru/v1",
	}, nil
}

// SendMessage sends a message to a chat
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) (string, error) {
	if chatID == 0 {
		return "", fmt.Errorf("chat_id is required")
	}
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	if len(text) > 4000 {
		return "", fmt.Errorf("text exceeds 4000 characters limit")
	}

	c.logger.Debug("sending message to MAX",
		zap.Int64("chat_id", chatID),
		zap.Int("text_length", len(text)))

	// Send with retry logic
	resp, err := c.sendWithRetry(ctx, chatID, text, 3)
	if err != nil {
		c.logger.Error("failed to send message after retries",
			zap.Int64("chat_id", chatID),
			zap.Error(err))
		return "", fmt.Errorf("send message: %w", err)
	}

	c.logger.Info("message sent successfully",
		zap.Int64("chat_id", chatID),
		zap.String("message_id", resp))

	return resp, nil
}

// SendMessageRequest represents a message request
type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

// SendMessageResponse represents a message response
type SendMessageResponse struct {
	ID int64 `json:"id"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
}

// sendWithRetry sends a message with exponential backoff retry
func (c *Client) sendWithRetry(ctx context.Context, chatID int64, text string, maxRetries int) (string, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Send message using HTTP POST
		result, err := c.sendMessage(chatID, text)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if it's a rate limit error
		if isRateLimitError(err) {
			waitTime := time.Duration(1<<uint(attempt)) * time.Second
			if waitTime > 10*time.Second {
				waitTime = 10 * time.Second
			}

			c.logger.Warn("rate limit hit, waiting before retry",
				zap.Int("attempt", attempt+1),
				zap.Duration("wait_time", waitTime))

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(waitTime):
			}
			continue
		}

		// For non-retryable errors, return immediately
		if !isRetryableError(err) {
			return "", err
		}

		// Exponential backoff
		waitTime := time.Duration(1<<uint(attempt)) * 100 * time.Millisecond
		c.logger.Debug("retrying after error",
			zap.Int("attempt", attempt+1),
			zap.Duration("wait_time", waitTime),
			zap.Error(err))

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// sendMessage sends a single message
func (c *Client) sendMessage(chatID int64, text string) (string, error) {
	url := fmt.Sprintf("%s/messages.sendMessage", c.baseURL)

	reqBody := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if !apiResp.OK {
		return "", fmt.Errorf("API returned error")
	}

	// Generate message ID
	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	return messageID, nil
}

// isRateLimitError checks if the error is a rate limit error
func isRateLimitError(err error) bool {
	// Check for rate limit specific errors
	// This is a placeholder - actual implementation depends on the MAX API client error types
	return false
}

// isRetryableError checks if the error is retryable
func isRetryableError(err error) bool {
	// Check for network errors, timeouts, etc.
	// This is a placeholder - actual implementation depends on the MAX API client error types
	return true
}

// GetMe returns information about the bot
func (c *Client) GetMe(ctx context.Context) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/users.getMe", c.baseURL)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	
	if !apiResp.OK {
		return nil, fmt.Errorf("API returned error")
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	
	return result, nil
}

// ValidateToken validates the bot token
func (c *Client) ValidateToken(ctx context.Context) error {
	_, err := c.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}
	return nil
}
