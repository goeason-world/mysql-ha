package ha

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: mysql-ha-solution, Property 8: Switchover Validation**
// For any switchover request with a target replica, the validation should pass
// if and only if the replica is healthy AND has replication lag below the configured threshold.
func TestSwitchoverValidation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("validation passes iff healthy AND lag below threshold", prop.ForAll(
		func(healthy bool, lag, maxLag int64) bool {
			if maxLag < 0 {
				return true // Skip invalid max lag
			}

			result := ValidateSwitchoverRequest(healthy, lag, maxLag)
			expected := healthy && lag <= maxLag

			return result == expected
		},
		gen.Bool(),
		gen.Int64Range(0, 1000),
		gen.Int64Range(0, 100),
	))

	properties.Property("unhealthy replica always fails validation", prop.ForAll(
		func(lag, maxLag int64) bool {
			result := ValidateSwitchoverRequest(false, lag, maxLag)
			return result == false
		},
		gen.Int64Range(0, 1000),
		gen.Int64Range(0, 100),
	))

	properties.Property("healthy replica with zero lag passes", prop.ForAll(
		func(maxLag int64) bool {
			if maxLag < 0 {
				return true
			}
			result := ValidateSwitchoverRequest(true, 0, maxLag)
			return result == true
		},
		gen.Int64Range(0, 100),
	))

	properties.Property("lag exceeding threshold fails", prop.ForAll(
		func(maxLag int64) bool {
			if maxLag < 0 {
				return true
			}
			// Lag is maxLag + 1, should fail
			result := ValidateSwitchoverRequest(true, maxLag+1, maxLag)
			return result == false
		},
		gen.Int64Range(0, 100),
	))

	properties.TestingRun(t)
}

func TestSwitchoverManagerValidation(t *testing.T) {
	sm := NewSwitchoverManager(nil, 10)

	// Test healthy with low lag
	v := sm.ValidateSwitchover(true, 5)
	if !v.IsValid {
		t.Error("Expected valid for healthy replica with low lag")
	}

	// Test unhealthy
	v = sm.ValidateSwitchover(false, 5)
	if v.IsValid {
		t.Error("Expected invalid for unhealthy replica")
	}

	// Test high lag
	v = sm.ValidateSwitchover(true, 15)
	if v.IsValid {
		t.Error("Expected invalid for high lag")
	}
}
