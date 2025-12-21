// Package ha provides high availability operations
package ha

import (
	"context"
	"fmt"
	"time"

	"mysql-ha/internal/agent"
	"mysql-ha/internal/dcs"
	"mysql-ha/internal/election"

	"github.com/google/uuid"
)

// FailoverManager handles failover operations
type FailoverManager struct {
	dcs      dcs.DCS
	cooldown time.Duration
}

// NewFailoverManager creates a new FailoverManager
func NewFailoverManager(d dcs.DCS, cooldown time.Duration) *FailoverManager {
	return &FailoverManager{
		dcs:      d,
		cooldown: cooldown,
	}
}

// ExecuteFailover performs automatic failover
func (fm *FailoverManager) ExecuteFailover(ctx context.Context, oldLeader string, candidates []*election.Candidate) (*agent.FailoverEvent, error) {
	event := &agent.FailoverEvent{
		ID:        generateEventID(),
		Type:      "failover",
		OldLeader: oldLeader,
		Reason:    "leader failure detected",
		StartTime: time.Now(),
	}

	// Select new leader
	result := election.SelectLeader(candidates)
	if result.Winner == nil {
		event.Success = false
		event.ErrorMessage = result.Reason
		event.EndTime = time.Now()
		return event, fmt.Errorf("no suitable candidate: %s", result.Reason)
	}

	event.NewLeader = result.Winner.NodeID

	// Record event
	event.Success = true
	event.EndTime = time.Now()

	// Store event in DCS
	if err := fm.recordEvent(ctx, event); err != nil {
		return event, fmt.Errorf("failed to record failover event: %w", err)
	}

	return event, nil
}

// recordEvent stores the failover event in DCS
func (fm *FailoverManager) recordEvent(ctx context.Context, event *agent.FailoverEvent) error {
	data, err := event.Serialize()
	if err != nil {
		return err
	}
	return fm.dcs.Set(ctx, "/events/"+event.ID, data)
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return uuid.New().String()
}

// CanFailover checks if failover is allowed (cooldown check)
func (fm *FailoverManager) CanFailover(ctx context.Context) (bool, error) {
	// Get last failover event
	data, err := fm.dcs.Get(ctx, "/last_failover")
	if err != nil {
		return true, nil // No previous failover, allow
	}
	if data == nil {
		return true, nil
	}

	event, err := agent.DeserializeFailoverEvent(data)
	if err != nil {
		return true, nil
	}

	// Check cooldown
	if time.Since(event.EndTime) < fm.cooldown {
		return false, nil
	}

	return true, nil
}
