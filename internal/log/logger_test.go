package log

import (
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: mysql-ha-solution, Property 14: Log Message Formatting**
// For any log event (state change, operation step, or error), the formatted log message
// should contain timestamp, node identifier, event type, and relevant context details.
func TestLogMessageFormatting(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	nonEmptyString := gen.Identifier().Map(func(s string) string {
		if s == "" {
			return "default"
		}
		return s
	})

	properties.Property("formatted log entry contains all required fields", prop.ForAll(
		func(nodeID, eventType, message string) bool {
			entry := LogEntry{
				Timestamp: time.Now(),
				Level:     INFO,
				NodeID:    nodeID,
				EventType: eventType,
				Message:   message,
				Context:   map[string]interface{}{"key": "value"},
			}

			formatted := FormatLogEntry(entry)

			// Verify all required fields are present
			hasTimestamp := strings.Contains(formatted, time.Now().Format("2006-01-02"))
			hasLevel := strings.Contains(formatted, "INFO")
			hasNodeID := strings.Contains(formatted, nodeID)
			hasEventType := strings.Contains(formatted, eventType)
			hasMessage := strings.Contains(formatted, message)

			return hasTimestamp && hasLevel && hasNodeID && hasEventType && hasMessage
		},
		nonEmptyString,
		nonEmptyString,
		nonEmptyString,
	))

	properties.Property("state change log contains old and new state", prop.ForAll(
		func(oldState, newState, reason string) bool {
			entry := LogEntry{
				Timestamp: time.Now(),
				Level:     INFO,
				NodeID:    "node1",
				EventType: "state_change",
				Message:   "state change from " + oldState + " to " + newState + " reason: " + reason,
			}

			formatted := FormatLogEntry(entry)
			return strings.Contains(formatted, "state_change") &&
				strings.Contains(formatted, oldState) &&
				strings.Contains(formatted, newState)
		},
		nonEmptyString,
		nonEmptyString,
		nonEmptyString,
	))

	properties.TestingRun(t)
}

// **Feature: mysql-ha-solution, Property 15: Log Level Filtering**
// For any log level configuration and log message, the message should be output
// if and only if the message level is greater than or equal to the configured level.
func TestLogLevelFiltering(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	levelGen := gen.IntRange(0, 3).Map(func(i int) Level { return Level(i) })

	properties.Property("message logged iff level >= min level", prop.ForAll(
		func(minLevel, msgLevel Level) bool {
			logger := &Logger{
				minLevel: minLevel,
			}

			shouldLog := logger.ShouldLog(msgLevel)
			expected := msgLevel >= minLevel

			return shouldLog == expected
		},
		levelGen,
		levelGen,
	))

	properties.Property("DEBUG messages only logged when min level is DEBUG", prop.ForAll(
		func(minLevel Level) bool {
			logger := &Logger{minLevel: minLevel}
			shouldLog := logger.ShouldLog(DEBUG)
			return shouldLog == (minLevel == DEBUG)
		},
		levelGen,
	))

	properties.Property("ERROR messages always logged regardless of min level", prop.ForAll(
		func(minLevel Level) bool {
			logger := &Logger{minLevel: minLevel}
			return logger.ShouldLog(ERROR)
		},
		levelGen,
	))

	properties.TestingRun(t)
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", DEBUG},
		{"DEBUG", DEBUG},
		{"info", INFO},
		{"INFO", INFO},
		{"warn", WARN},
		{"WARN", WARN},
		{"warning", WARN},
		{"error", ERROR},
		{"ERROR", ERROR},
		{"unknown", INFO}, // default
		{"", INFO},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
