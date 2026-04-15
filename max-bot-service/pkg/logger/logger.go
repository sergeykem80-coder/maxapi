package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a new zap logger based on the provided level
func New(level string) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.MessageKey = "message"

	return config.Build(zap.AddStacktrace(zapcore.PanicLevel))
}

// NewDevelopment creates a development logger with human-readable output
func NewDevelopment(level string) (*zap.Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.DebugLevel
	}

	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return config.Build(zap.AddStacktrace(zapcore.PanicLevel))
}

// Sync closes the logger and flushes any buffered log entries
func Sync(logger *zap.Logger) {
	if logger != nil {
		_ = logger.Sync()
	}
}

// Logger is a global logger instance
var Logger *zap.Logger

// Init initializes the global logger
func Init(level string) error {
	var err error
	Logger, err = New(level)
	return err
}

// L returns the global logger
func L() *zap.Logger {
	if Logger == nil {
		Logger, _ = zap.NewProduction()
	}
	return Logger
}

// SugaredLogger returns a sugared version of the global logger
func SL() *zap.SugaredLogger {
	return L().Sugar()
}

// HTTPLogger returns a logger suitable for HTTP request logging
func HTTPLogger() *zap.Logger {
	return L().WithOptions(zap.AddCallerSkip(1))
}

// WithFields adds fields to the logger
func WithFields(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	L().Fatal(msg, fields...)
}

// Recover handles panics gracefully
func Recover() {
	if r := recover(); r != nil {
		L().Fatal("Recovered from panic", zap.Any("panic", r))
	}
}

// NewHTTPMiddlewareLogger creates a logger for HTTP middleware
func NewHTTPMiddlewareLogger() *zap.Logger {
	return L().With(zap.String("component", "http"))
}

// RequestContextFields creates common fields for request context
func RequestContextFields(method, path, clientIP string) []zap.Field {
	return []zap.Field{
		zap.String("method", method),
		zap.String("path", path),
		zap.String("client_ip", clientIP),
	}
}
