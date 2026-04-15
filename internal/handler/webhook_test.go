package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sergeykem80-maxapi/internal/service"
	"github.com/sergeykem80-maxapi/internal/store/postgres"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupTestWebhookHandler() (*WebhookHandler, *zap.Logger) {
	logger, _ := zap.NewDevelopment()
	
	// Create mock session service (nil for now, will be tested separately)
	sessionService := &service.SessionService{}
	webhookSecret := "test_secret_123"
	
	return NewWebhookHandler(sessionService, webhookSecret, logger), logger
}

func TestWebhookHandler_ValidateSecret(t *testing.T) {
	handler, _ := setupTestWebhookHandler()
	
	tests := []struct {
		name     string
		secret   string
		expected bool
	}{
		{"valid secret", "test_secret_123", true},
		{"invalid secret", "wrong_secret", false},
		{"empty secret", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateSecret(tt.secret)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookHandler_ExtractSecret(t *testing.T) {
	handler, _ := setupTestWebhookHandler()
	
	tests := []struct {
		name         string
		payloadStart string
		expected     string
	}{
		{"with prefix", "secret_abc123", "abc123"},
		{"without prefix", "just_value", "just_value"},
		{"empty", "", ""},
		{"prefix only", "secret_", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.extractSecret(tt.payloadStart)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookHandler_HandleWebhook_MissingSecret(t *testing.T) {
	handler, _ := setupTestWebhookHandler()
	
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	body := WebhookRequest{
		Type: "bot_started",
	}
	jsonBody, _ := json.Marshal(body)
	
	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{},
		Body:   io.NopCloser(bytes.NewReader(jsonBody)),
	}
	
	handler.HandleWebhook(c)
	
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebhookHandler_HandleWebhook_InvalidSecret(t *testing.T) {
	handler, _ := setupTestWebhookHandler()
	
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	body := WebhookRequest{
		Type: "bot_started",
	}
	jsonBody, _ := json.Marshal(body)
	
	c.Request = &http.Request{
		Method: "POST",
		Header: http.Header{
			"X-Max-Bot-Api-Secret": []string{"wrong_secret"},
			"Content-Type":         []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(jsonBody)),
	}
	
	handler.HandleWebhook(c)
	
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWebhookHandler_HealthCheck(t *testing.T) {
	handler, _ := setupTestWebhookHandler()
	
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	c.Request = &http.Request{
		Method: "GET",
		URL:    nil,
	}
	
	handler.HealthCheck(c)
	
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.NotEmpty(t, response["timestamp"])
}

func TestSessionData_JSON(t *testing.T) {
	sessionData := SessionData{
		Secret:    "test_secret",
		UserID:    123456,
		ChatID:    123456,
		Username:  "test_user",
		FirstName: "Тест",
		StartedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		IsActive:  true,
	}
	
	data, err := json.Marshal(sessionData)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	
	var unmarshaled SessionData
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, sessionData.Secret, unmarshaled.Secret)
	assert.Equal(t, sessionData.UserID, unmarshaled.UserID)
}

func TestNotifyRequest_Validation(t *testing.T) {
	tests := []struct {
		name        string
		request     NotifyRequest
		expectError bool
	}{
		{
			name: "valid request",
			request: NotifyRequest{
				Secret:   "test_secret",
				Text:     "Hello",
				Priority: "normal",
			},
			expectError: false,
		},
		{
			name: "missing secret",
			request: NotifyRequest{
				Text:     "Hello",
				Priority: "normal",
			},
			expectError: true,
		},
		{
			name: "missing text",
			request: NotifyRequest{
				Secret:   "test_secret",
				Priority: "normal",
			},
			expectError: true,
		},
		{
			name: "invalid priority",
			request: NotifyRequest{
				Secret:   "test_secret",
				Text:     "Hello",
				Priority: "high",
			},
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, data)
			}
		})
	}
}

func TestWebhookRequest_Parse(t *testing.T) {
	jsonData := `{
		"update_id": 1,
		"type": "bot_started",
		"sender": {
			"id": 123456,
			"username": "test_user",
			"first_name": "Тест"
		},
		"chat": {
			"id": 123456
		},
		"payload": {
			"start": "secret_test123"
		}
	}`
	
	var req WebhookRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), req.UpdateID)
	assert.Equal(t, "bot_started", req.Type)
	assert.Equal(t, int64(123456), req.Sender.ID)
	assert.Equal(t, "test_user", req.Sender.Username)
	assert.Equal(t, "Тест", req.Sender.FirstName)
	assert.Equal(t, int64(123456), req.Chat.ID)
	assert.Equal(t, "secret_test123", req.Payload.Start)
}

func TestSessionResponse_JSON(t *testing.T) {
	response := SessionResponse{
		Success: true,
		Data: &SessionData{
			Secret:    "test_secret",
			UserID:    123456,
			ChatID:    123456,
			Username:  "test_user",
			FirstName: "Тест",
			StartedAt: time.Now(),
			IsActive:  true,
		},
	}
	
	data, err := json.Marshal(response)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	
	var unmarshaled SessionResponse
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.True(t, unmarshaled.Success)
	assert.NotNil(t, unmarshaled.Data)
}

func TestNotifyResponse_JSON(t *testing.T) {
	response := NotifyResponse{
		Success:     true,
		MessageID:   "msg_abc123",
		DeliveredAt: time.Now(),
	}
	
	data, err := json.Marshal(response)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	
	var unmarshaled NotifyResponse
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.True(t, unmarshaled.Success)
	assert.Equal(t, "msg_abc123", unmarshaled.MessageID)
}

func TestOneCAPIHandler_ValidateBearerToken(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handler := NewOneCAPIHandler(nil, nil, "test_api_key", logger)
	
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{"valid token", "Bearer test_api_key", true},
		{"invalid token", "Bearer wrong_key", false},
		{"no bearer prefix", "test_api_key", false},
		{"empty", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			
			if tt.token != "" {
				c.Request = &http.Request{
					Header: http.Header{
						"Authorization": []string{tt.token},
					},
				}
			} else {
				c.Request = &http.Request{
					Header: http.Header{},
				}
			}
			
			result := handler.validateBearerToken(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgresSession_Struct(t *testing.T) {
	session := postgres.Session{
		Secret:       "test_secret",
		UserID:       123456,
		ChatID:       123456,
		Username:     "test_user",
		FirstName:    "Тест",
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
		IsActive:     true,
	}
	
	data, err := json.Marshal(session)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
	
	var unmarshaled postgres.Session
	err = json.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, session.Secret, unmarshaled.Secret)
	assert.Equal(t, session.UserID, unmarshaled.UserID)
}
