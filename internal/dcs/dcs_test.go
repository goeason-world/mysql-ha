package dcs

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: mysql-ha-solution, Property 4: Exponential Backoff Calculation**
// For any sequence of retry attempts, the backoff interval should double with each attempt
// (starting from base interval) and never exceed the maximum interval of 30 seconds.
func TestExponentialBackoffCalculation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("backoff doubles with each attempt", prop.ForAll(
		func(attempt int) bool {
			if attempt < 0 || attempt > 10 {
				return true // Skip extreme values
			}

			baseInterval := time.Second
			maxInterval := 30 * time.Second

			interval := CalculateBackoff(attempt, baseInterval, maxInterval)

			// Expected interval is base * 2^attempt, capped at max
			expected := baseInterval * (1 << attempt)
			if expected > maxInterval {
				expected = maxInterval
			}

			return interval == expected
		},
		gen.IntRange(0, 10),
	))

	properties.Property("backoff never exceeds max interval", prop.ForAll(
		func(attempt int) bool {
			baseInterval := time.Second
			maxInterval := 30 * time.Second

			interval := CalculateBackoff(attempt, baseInterval, maxInterval)
			return interval <= maxInterval
		},
		gen.IntRange(0, 100),
	))

	properties.Property("backoff starts at base interval", prop.ForAll(
		func(baseMs int) bool {
			if baseMs <= 0 {
				return true
			}

			baseInterval := time.Duration(baseMs) * time.Millisecond
			maxInterval := 30 * time.Second

			interval := CalculateBackoff(0, baseInterval, maxInterval)
			return interval == baseInterval
		},
		gen.IntRange(1, 5000),
	))

	properties.Property("backoff sequence is monotonically increasing until max", prop.ForAll(
		func(attempts int) bool {
			if attempts < 2 || attempts > 20 {
				return true
			}

			baseInterval := time.Second
			maxInterval := 30 * time.Second

			var prev time.Duration
			for i := 0; i < attempts; i++ {
				curr := CalculateBackoff(i, baseInterval, maxInterval)
				if i > 0 && curr < prev {
					return false // Should never decrease
				}
				prev = curr
			}
			return true
		},
		gen.IntRange(2, 20),
	))

	properties.TestingRun(t)
}

func TestBackoffStruct(t *testing.T) {
	b := NewBackoff()

	// First attempt should be base interval
	if b.Next() != time.Second {
		t.Error("First backoff should be 1 second")
	}

	// Second attempt should be 2 seconds
	if b.Next() != 2*time.Second {
		t.Error("Second backoff should be 2 seconds")
	}

	// Third attempt should be 4 seconds
	if b.Next() != 4*time.Second {
		t.Error("Third backoff should be 4 seconds")
	}

	// Reset should start over
	b.Reset()
	if b.Next() != time.Second {
		t.Error("After reset, backoff should be 1 second")
	}
}

func TestBackoffMaxInterval(t *testing.T) {
	b := NewBackoff()

	// Keep calling Next until we hit max
	var lastInterval time.Duration
	for i := 0; i < 10; i++ {
		lastInterval = b.Next()
	}

	// Should be capped at 30 seconds
	if lastInterval != 30*time.Second {
		t.Errorf("Expected max interval of 30s, got %v", lastInterval)
	}
}
