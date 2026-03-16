package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Level represents log level
type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
	LevelDebug
)

// Logger provides structured JSON logging
type Logger struct {
	logger *log.Logger
	mu     sync.Mutex
	level  Level
}

// Global logger instance
var defaultLogger *Logger

func init() {
	defaultLogger = New(os.Stdout, LevelInfo)
}

// New creates a new structured logger
func New(output *os.File, level Level) *Logger {
	return &Logger{
		logger: log.New(output, "", 0),
		level:  level,
	}
}

// Get returns the global logger
func Get() *Logger {
	return defaultLogger
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Info logs an info message with structured fields
func (l *Logger) Info(msg string, fields ...Field) {
	if l.level > LevelInfo {
		return
	}
	l.log(LevelInfo, msg, fields)
}

// Warn logs a warning message with structured fields
func (l *Logger) Warn(msg string, fields ...Field) {
	if l.level > LevelWarn {
		return
	}
	l.log(LevelWarn, msg, fields)
}

// Error logs an error message with structured fields
func (l *Logger) Err(msg string, fields ...Field) {
	l.log(LevelError, msg, fields)
}

// Debug logs a debug message with structured fields
func (l *Logger) Debug(msg string, fields ...Field) {
	if l.level > LevelDebug {
		return
	}
	l.log(LevelDebug, msg, fields)
}

// Request logs an HTTP request with structured fields
func (l *Logger) Request(method, path, remoteAddr, userAgent string, duration time.Duration, status int) {
	fields := []Field{
		String("method", method),
		String("path", path),
		String("remote_addr", remoteAddr),
		String("user_agent", userAgent),
		Int("status", status),
		Float64("duration_ms", float64(duration.Milliseconds())),
	}
	l.log(LevelInfo, "HTTP request", fields)
}

// ProxyRequest logs a proxy request
func (l *Logger) ProxyRequest(method, model, path, upstream string, duration time.Duration, status int, err error) {
	fields := []Field{
		String("method", method),
		String("model", model),
		String("path", path),
		String("upstream", upstream),
		Int("status", status),
		Float64("duration_ms", float64(duration.Milliseconds())),
	}
	if err != nil {
		fields = append(fields, Field{Key: "error", Value: err.Error()})
	}
	l.log(LevelInfo, "Proxy request", fields)
}

func (l *Logger) log(level Level, msg string, fields []Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     levelString(level),
		"message":   msg,
	}

	for _, f := range fields {
		entry[f.Key] = f.Value
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		// Fallback to plain text
		l.logger.Printf("[%s] %s %v", levelString(level), msg, fields)
		return
	}
	l.logger.Println(string(jsonBytes))
}

func levelString(level Level) string {
	switch level {
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a bool field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field
func Err(key string, err error) Field {
	if err == nil {
		return Field{Key: key, Value: nil}
	}
	return Field{Key: key, Value: err.Error()}
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Convenience functions for global logger
func Info(msg string, fields ...Field) {
	defaultLogger.Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	defaultLogger.Warn(msg, fields...)
}

func Error(msg string, fields ...Field) {
	defaultLogger.Err(msg, fields...)
}

func Debug(msg string, fields ...Field) {
	defaultLogger.Debug(msg, fields...)
}

func Request(method, path, remoteAddr, userAgent string, duration time.Duration, status int) {
	defaultLogger.Request(method, path, remoteAddr, userAgent, duration, status)
}

func ProxyRequest(method, model, path, upstream string, duration time.Duration, status int, err error) {
	defaultLogger.ProxyRequest(method, model, path, upstream, duration, status, err)
}

// FormatDuration formats duration for logging
func FormatDuration(d time.Duration) string {
	return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
}
