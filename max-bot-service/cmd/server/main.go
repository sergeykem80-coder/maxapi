package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/max-bot-service/internal/config"
	"github.com/example/max-bot-service/internal/handler"
	"github.com/example/max-bot-service/internal/maxclient"
	"github.com/example/max-bot-service/internal/service"
	"github.com/example/max-bot-service/internal/store/postgres"
	"github.com/example/max-bot-service/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	if err := logger.Init(cfg.GetLogLevel()); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync(logger.Logger)

	log := logger.L()
	log.Info("starting MAX Bot integration service",
		zap.String("version", "1.0.0"),
		zap.String("log_level", cfg.GetLogLevel()))

	// Initialize database connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := postgres.NewStore(ctx, cfg.GetDatabaseURL())
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer store.Close()

	log.Info("connected to PostgreSQL database")

	// Initialize MAX Bot API client
	maxClient, err := maxclient.NewClient(cfg.GetMaxBotToken(), log)
	if err != nil {
		log.Fatal("failed to create MAX Bot API client", zap.Error(err))
	}

	log.Info("initialized MAX Bot API client")

	// Initialize services
	sessionService := service.NewSessionService(store, log, cfg.GetSessionTTL())
	messageService := service.NewMessageService(maxClient, store, log)

	// Initialize handlers
	webhookHandler := handler.NewWebhookHandler(sessionService, cfg.GetWebhookSecret(), log)
	api1cHandler := handler.NewOneCAPIHandler(sessionService, messageService, cfg.GetOneCAPIKey(), log)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware(log))

	// Register routes
	// Health check endpoint
	router.GET("/health", webhookHandler.HealthCheck)

	// MAX Bot webhook endpoint
	router.POST("/webhook", webhookHandler.HandleWebhook)

	// 1C API endpoints
	apiV1 := router.Group("/api/v1")
	{
		apiV1.GET("/session", api1cHandler.GetSession)
		apiV1.POST("/notify", api1cHandler.SendNotification)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.GetServerPort(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("starting HTTP server",
			zap.String("port", cfg.GetServerPort()))
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}

	log.Info("server stopped")
}

// loggingMiddleware creates a middleware for logging HTTP requests
func loggingMiddleware(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		log.Info("HTTP request",
			zap.Int("status", statusCode),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("client_ip", clientIP),
			zap.Duration("latency", latency),
		)
	}
}

// corsMiddleware creates a middleware for CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Max-Bot-Api-Secret")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}
