package election

import (
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: mysql-ha-solution, Property 6: Leader Election by GTID Position**
// For any set of candidate nodes with different GTID positions, the election algorithm
// should select the candidate with the most advanced GTID position.
func TestLeaderElectionByGTIDPosition(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Generator for GTID transaction count
	gtidCountGen := gen.IntRange(1, 10000)

	properties.Property("candidate with highest GTID wins", prop.ForAll(
		func(count1, count2, count3 int) bool {
			// Create candidates with different GTID positions
			candidates := []*Candidate{
				{NodeID: "node1", GTIDExecuted: fmt.Sprintf("uuid1:1-%d", count1), IsHealthy: true},
				{NodeID: "node2", GTIDExecuted: fmt.Sprintf("uuid1:1-%d", count2), IsHealthy: true},
				{NodeID: "node3", GTIDExecuted: fmt.Sprintf("uuid1:1-%d", count3), IsHealthy: true},
			}

			result := SelectLeader(candidates)
			if result.Winner == nil {
				return false
			}

			// Find the maximum count
			maxCount := count1
			if count2 > maxCount {
				maxCount = count2
			}
			if count3 > maxCount {
				maxCount = count3
			}

			// Winner should have the max count
			winnerGTID := result.Winner.GTIDExecuted
			expectedGTID := fmt.Sprintf("uuid1:1-%d", maxCount)

			return winnerGTID == expectedGTID
		},
		gtidCountGen,
		gtidCountGen,
		gtidCountGen,
	))

	properties.Property("unhealthy candidates are not selected", prop.ForAll(
		func(count1, count2 int) bool {
			// Node1 has higher GTID but is unhealthy
			candidates := []*Candidate{
				{NodeID: "node1", GTIDExecuted: fmt.Sprintf("uuid1:1-%d", count1+1000), IsHealthy: false},
				{NodeID: "node2", GTIDExecuted: fmt.Sprintf("uuid1:1-%d", count2), IsHealthy: true},
			}

			result := SelectLeader(candidates)
			if result.Winner == nil {
				return false
			}

			// Winner should be node2 (healthy) even though node1 has higher GTID
			return result.Winner.NodeID == "node2"
		},
		gtidCountGen,
		gtidCountGen,
	))

	properties.Property("empty candidate list returns no winner", prop.ForAll(
		func(_ int) bool {
			result := SelectLeader([]*Candidate{})
			return result.Winner == nil
		},
		gen.Int(),
	))

	properties.Property("single healthy candidate wins", prop.ForAll(
		func(count int) bool {
			candidates := []*Candidate{
				{NodeID: "node1", GTIDExecuted: fmt.Sprintf("uuid1:1-%d", count), IsHealthy: true},
			}

			result := SelectLeader(candidates)
			return result.Winner != nil && result.Winner.NodeID == "node1"
		},
		gtidCountGen,
	))

	properties.TestingRun(t)
}

func TestCompareGTIDSets(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"uuid1:1-100", "", 1},
		{"", "uuid1:1-100", -1},
		{"uuid1:1-100", "uuid1:1-100", 0},
		{"uuid1:1-200", "uuid1:1-100", 1},
		{"uuid1:1-100", "uuid1:1-200", -1},
		{"uuid1:1-100,uuid2:1-50", "uuid1:1-100", 1},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.a, tt.b), func(t *testing.T) {
			result := CompareGTIDSets(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("CompareGTIDSets(%q, %q) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestRankCandidates(t *testing.T) {
	candidates := []*Candidate{
		{NodeID: "node1", GTIDExecuted: "uuid1:1-100", IsHealthy: true},
		{NodeID: "node2", GTIDExecuted: "uuid1:1-300", IsHealthy: true},
		{NodeID: "node3", GTIDExecuted: "uuid1:1-200", IsHealthy: true},
	}

	ranked := RankCandidates(candidates)

	if len(ranked) != 3 {
		t.Fatalf("Expected 3 candidates, got %d", len(ranked))
	}

	// Should be ordered by GTID descending
	if ranked[0].NodeID != "node2" {
		t.Errorf("Expected node2 first, got %s", ranked[0].NodeID)
	}
	if ranked[1].NodeID != "node3" {
		t.Errorf("Expected node3 second, got %s", ranked[1].NodeID)
	}
	if ranked[2].NodeID != "node1" {
		t.Errorf("Expected node1 third, got %s", ranked[2].NodeID)
	}
}
