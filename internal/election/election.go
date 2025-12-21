package election

import (
	"sort"
	"strings"
)

// Candidate represents a node that can be elected as leader
type Candidate struct {
	NodeID       string
	GTIDExecuted string
	IsHealthy    bool
	Priority     int // Higher priority = more preferred
}

// ElectionResult contains the result of leader election
type ElectionResult struct {
	Winner     *Candidate
	Candidates []*Candidate
	Reason     string
}

// SelectLeader selects the best candidate for leader based on GTID position
// The candidate with the most advanced GTID position wins
func SelectLeader(candidates []*Candidate) *ElectionResult {
	if len(candidates) == 0 {
		return &ElectionResult{
			Winner: nil,
			Reason: "no candidates available",
		}
	}

	// Filter healthy candidates
	var healthyCandidates []*Candidate
	for _, c := range candidates {
		if c.IsHealthy {
			healthyCandidates = append(healthyCandidates, c)
		}
	}

	if len(healthyCandidates) == 0 {
		return &ElectionResult{
			Winner:     nil,
			Candidates: candidates,
			Reason:     "no healthy candidates available",
		}
	}

	// Sort by GTID position (descending) and priority
	sorted := make([]*Candidate, len(healthyCandidates))
	copy(sorted, healthyCandidates)

	sort.Slice(sorted, func(i, j int) bool {
		// First compare by GTID position
		cmp := CompareGTIDSets(sorted[i].GTIDExecuted, sorted[j].GTIDExecuted)
		if cmp != 0 {
			return cmp > 0 // Higher GTID wins
		}
		// Then by priority
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		// Finally by node ID for determinism
		return sorted[i].NodeID < sorted[j].NodeID
	})

	return &ElectionResult{
		Winner:     sorted[0],
		Candidates: sorted,
		Reason:     "selected by GTID position",
	}
}

// CompareGTIDSets compares two GTID sets
// Returns: 1 if a > b, -1 if a < b, 0 if equal
func CompareGTIDSets(a, b string) int {
	// Empty GTID handling
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}

	// Parse GTID sets
	aGTIDs := parseGTIDSet(a)
	bGTIDs := parseGTIDSet(b)

	// Compare by total transaction count
	aCount := countTransactions(aGTIDs)
	bCount := countTransactions(bGTIDs)

	if aCount > bCount {
		return 1
	}
	if aCount < bCount {
		return -1
	}

	// If counts are equal, compare lexicographically
	if a > b {
		return 1
	}
	if a < b {
		return -1
	}
	return 0
}

// GTIDRange represents a range of GTIDs for a single server UUID
type GTIDRange struct {
	UUID  string
	Start int64
	End   int64
}

// parseGTIDSet parses a GTID set string into ranges
// Format: uuid:start-end,uuid:start-end,...
func parseGTIDSet(gtidSet string) []GTIDRange {
	var ranges []GTIDRange

	gtidSet = strings.TrimSpace(gtidSet)
	if gtidSet == "" {
		return ranges
	}

	// Split by comma for multiple UUIDs
	parts := strings.Split(gtidSet, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split UUID and range
		colonIdx := strings.LastIndex(part, ":")
		if colonIdx == -1 {
			continue
		}

		uuid := part[:colonIdx]
		rangeStr := part[colonIdx+1:]

		// Parse range (could be single number or start-end)
		var start, end int64
		if dashIdx := strings.Index(rangeStr, "-"); dashIdx != -1 {
			// Range format: start-end
			var s, e int64
			n, _ := parseRange(rangeStr, &s, &e)
			if n == 2 {
				start, end = s, e
			}
		} else {
			// Single transaction
			var n int64
			if _, err := parseNumber(rangeStr, &n); err == nil {
				start, end = n, n
			}
		}

		ranges = append(ranges, GTIDRange{
			UUID:  uuid,
			Start: start,
			End:   end,
		})
	}

	return ranges
}

// parseRange parses a range string like "1-100"
func parseRange(s string, start, end *int64) (int, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return 0, nil
	}

	var err error
	*start, err = parseInt64(parts[0])
	if err != nil {
		return 0, err
	}

	*end, err = parseInt64(parts[1])
	if err != nil {
		return 1, err
	}

	return 2, nil
}

// parseNumber parses a single number
func parseNumber(s string, n *int64) (int, error) {
	var err error
	*n, err = parseInt64(s)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

// parseInt64 parses a string to int64
func parseInt64(s string) (int64, error) {
	s = strings.TrimSpace(s)
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

// countTransactions counts total transactions in GTID ranges
func countTransactions(ranges []GTIDRange) int64 {
	var total int64
	for _, r := range ranges {
		if r.End >= r.Start {
			total += r.End - r.Start + 1
		}
	}
	return total
}

// RankCandidates ranks candidates by their GTID position
func RankCandidates(candidates []*Candidate) []*Candidate {
	if len(candidates) == 0 {
		return candidates
	}

	ranked := make([]*Candidate, len(candidates))
	copy(ranked, candidates)

	sort.Slice(ranked, func(i, j int) bool {
		cmp := CompareGTIDSets(ranked[i].GTIDExecuted, ranked[j].GTIDExecuted)
		if cmp != 0 {
			return cmp > 0
		}
		return ranked[i].NodeID < ranked[j].NodeID
	})

	return ranked
}
