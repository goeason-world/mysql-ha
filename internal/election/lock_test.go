package election

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: mysql-ha-solution, Property 7: Lock Renewal Interval**
// For any TTL value, the lock renewal interval should equal exactly one-third of the TTL value.
func TestLockRenewalInterval(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("renewal interval equals TTL/3", prop.ForAll(
		func(ttlSeconds int) bool {
			if ttlSeconds <= 0 {
				return true // Skip invalid TTL
			}

			ttl := time.Duration(ttlSeconds) * time.Second
			renewalInterval := CalculateRenewalInterval(ttl)
			expected := ttl / 3

			return renewalInterval == expected
		},
		gen.IntRange(1, 300),
	))

	properties.Property("renewal interval is always less than TTL", prop.ForAll(
		func(ttlSeconds int) bool {
			if ttlSeconds <= 0 {
				return true
			}

			ttl := time.Duration(ttlSeconds) * time.Second
			renewalInterval := CalculateRenewalInterval(ttl)

			return renewalInterval < ttl
		},
		gen.IntRange(1, 300),
	))

	properties.Property("three renewals fit within TTL", prop.ForAll(
		func(ttlSeconds int) bool {
			if ttlSeconds <= 0 {
				return true
			}

			ttl := time.Duration(ttlSeconds) * time.Second
			renewalInterval := CalculateRenewalInterval(ttl)

			// Three renewal intervals should equal TTL
			return renewalInterval*3 == ttl
		},
		gen.IntRange(3, 300).SuchThat(func(n int) bool { return n%3 == 0 }),
	))

	properties.TestingRun(t)
}

func TestLockManagerCreation(t *testing.T) {
	ttl := 30 * time.Second
	lm := NewLockManager(nil, "test-lock", ttl)

	if lm.GetTTL() != ttl {
		t.Errorf("Expected TTL %v, got %v", ttl, lm.GetTTL())
	}

	expectedRenewal := 10 * time.Second
	if lm.GetRenewalPeriod() != expectedRenewal {
		t.Errorf("Expected renewal period %v, got %v", expectedRenewal, lm.GetRenewalPeriod())
	}

	if lm.IsLeader() {
		t.Error("New lock manager should not be leader")
	}
}
