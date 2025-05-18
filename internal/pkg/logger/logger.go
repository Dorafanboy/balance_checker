package logger

import (
	"log/slog"
	"os"
	"strings"
)

var globalLogger *slog.Logger // Возвращаем один глобальный логгер

// InitSlog initializes the global slog logger with a specified log level and JSON format.
func InitSlog(levelStr string) {
	var parsedLevel slog.Level
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		parsedLevel = slog.LevelDebug
	case "INFO":
		parsedLevel = slog.LevelInfo
	case "WARN":
		parsedLevel = slog.LevelWarn
	case "ERROR":
		parsedLevel = slog.LevelError
	default:
		parsedLevel = slog.LevelInfo
		// Используем slog.Default(), так как globalLogger еще не инициализирован
		slog.Warn("Invalid log level string, defaulting to INFO", "input", levelStr)
	}

	opts := &slog.HandlerOptions{
		Level:     parsedLevel,
		AddSource: false, // Полностью отключаем вывод источника
	}
	// Используем JSONHandler вместо TextHandler
	handler := slog.NewJSONHandler(os.Stdout, opts)
	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger) // Устанавливаем как стандартный slog логгер
}

// ensureInitialized проверяет, инициализирован ли логгер.
func ensureInitialized() {
	if globalLogger == nil {
		InitSlog("INFO") // Initialize with default if not already done
	}
}

// Debug logs a message at DebugLevel.
func Debug(msg string, args ...any) {
	ensureInitialized()
	if globalLogger.Enabled(nil, slog.LevelDebug) {
		globalLogger.Debug(msg, args...)
	}
}

// Info logs a message at InfoLevel.
func Info(msg string, args ...any) {
	ensureInitialized()
	if globalLogger.Enabled(nil, slog.LevelInfo) {
		globalLogger.Info(msg, args...)
	}
}

// Warn logs a message at WarnLevel.
func Warn(msg string, args ...any) {
	ensureInitialized()
	if globalLogger.Enabled(nil, slog.LevelWarn) {
		globalLogger.Warn(msg, args...)
	}
}

// Error logs a message at ErrorLevel.
func Error(msg string, args ...any) {
	ensureInitialized()
	if globalLogger.Enabled(nil, slog.LevelError) {
		globalLogger.Error(msg, args...)
	}
}

// Fatal logs a message at ErrorLevel then exits.
func Fatal(msg string, args ...any) {
	ensureInitialized()
	// Логируем всегда перед выходом, независимо от Enabled, т.к. это Fatal
	globalLogger.Error(msg, args...) 
	os.Exit(1)
}
