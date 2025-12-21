// Package log provides structured logging for MySQL HA Agent
package log

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string into a Level
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// Logger provides structured logging
type Logger struct {
	zap      *zap.Logger
	sugar    *zap.SugaredLogger
	nodeID   string
	level    Level
	minLevel Level
}

// LogEntry represents a log entry for testing
type LogEntry struct {
	Timestamp time.Time
	Level     Level
	NodeID    string
	EventType string
	Message   string
	Context   map[string]interface{}
}

// Config holds logger configuration
type Config struct {
	Level  string
	NodeID string
	File   string
}

// New creates a new Logger
func New(cfg Config) (*Logger, error) {
	level := ParseLevel(cfg.Level)

	zapLevel := zapcore.InfoLevel
	switch level {
	case DEBUG:
		zapLevel = zapcore.DebugLevel
	case INFO:
		zapLevel = zapcore.InfoLevel
	case WARN:
		zapLevel = zapcore.WarnLevel
	case ERROR:
		zapLevel = zapcore.ErrorLevel
	}

	zapCfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(zapLevel),
		Development:       false,
		Encoding:          "json",
		DisableCaller:     true,
		DisableStacktrace: true,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "",
			MessageKey:     "message",
			StacktraceKey:  "",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   nil,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if cfg.File != "" {
		zapCfg.OutputPaths = append(zapCfg.OutputPaths, cfg.File)
	}

	zapLogger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &Logger{
		zap:      zapLogger,
		sugar:    zapLogger.Sugar(),
		nodeID:   cfg.NodeID,
		level:    level,
		minLevel: level,
	}, nil
}

// SetMinLevel sets the minimum log level
func (l *Logger) SetMinLevel(level Level) {
	l.minLevel = level
}

// ShouldLog returns true if the message should be logged based on level
func (l *Logger) ShouldLog(level Level) bool {
	return level >= l.minLevel
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	if !l.ShouldLog(DEBUG) {
		return
	}
	l.zap.Debug(msg, append(fields, zap.String("node_id", l.nodeID))...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...zap.Field) {
	if !l.ShouldLog(INFO) {
		return
	}
	l.zap.Info(msg, append(fields, zap.String("node_id", l.nodeID))...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	if !l.ShouldLog(WARN) {
		return
	}
	l.zap.Warn(msg, append(fields, zap.String("node_id", l.nodeID))...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...zap.Field) {
	if !l.ShouldLog(ERROR) {
		return
	}
	allFields := []zap.Field{zap.String("node_id", l.nodeID)}
	allFields = append(allFields, fields...)
	l.zap.Error(msg, allFields...)
}

// StateChange logs a state change event
func (l *Logger) StateChange(oldState, newState, reason string) {
	l.Info("state change",
		zap.String("event_type", "state_change"),
		zap.String("old_state", oldState),
		zap.String("new_state", newState),
		zap.String("reason", reason),
	)
}

// OperationStep logs an operation step
func (l *Logger) OperationStep(operation, step string, duration time.Duration) {
	l.Info("operation step",
		zap.String("event_type", "operation_step"),
		zap.String("operation", operation),
		zap.String("step", step),
		zap.Duration("duration", duration),
	)
}

// ErrorWithContext logs an error with context
func (l *Logger) ErrorWithContext(msg string, err error, context map[string]interface{}) {
	fields := []zap.Field{
		zap.String("node_id", l.nodeID),
		zap.String("event_type", "error"),
		zap.Error(err),
	}
	for k, v := range context {
		fields = append(fields, zap.Any(k, v))
	}
	l.zap.Error(msg, fields...)
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// FormatLogEntry formats a log entry for testing purposes
func FormatLogEntry(entry LogEntry) string {
	return fmt.Sprintf("[%s] [%s] [%s] [%s] %s",
		entry.Timestamp.Format(time.RFC3339),
		entry.Level.String(),
		entry.NodeID,
		entry.EventType,
		entry.Message,
	)
}
