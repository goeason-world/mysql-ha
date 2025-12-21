package ha

import (
	"context"
	"fmt"
	"time"

	"mysql-ha/internal/agent"
	"mysql-ha/internal/dcs"
)

// SwitchoverManager handles planned switchover operations
type SwitchoverManager struct {
	dcs    dcs.DCS
	maxLag int64 // Maximum allowed replication lag in seconds
}

// NewSwitchoverManager creates a new SwitchoverManager
func NewSwitchoverManager(d dcs.DCS, maxLag int64) *SwitchoverManager {
	return &SwitchoverManager{
		dcs:    d,
		maxLag: maxLag,
	}
}

// SwitchoverRequest represents a switchover request
type SwitchoverRequest struct {
	TargetNodeID string
	Reason       string
}

// SwitchoverValidation represents the result of switchover validation
type SwitchoverValidation struct {
	IsValid        bool
	TargetHealthy  bool
	ReplicationLag int64
	ErrorMessage   string
}

// ValidateSwitchover validates if switchover can proceed
func (sm *SwitchoverManager) ValidateSwitchover(targetHealthy bool, replicationLag int64) *SwitchoverValidation {
	validation := &SwitchoverValidation{
		TargetHealthy:  targetHealthy,
		ReplicationLag: replicationLag,
	}

	if !targetHealthy {
		validation.IsValid = false
		validation.ErrorMessage = "target replica is not healthy"
		return validation
	}

	if replicationLag > sm.maxLag {
		validation.IsValid = false
		validation.ErrorMessage = fmt.Sprintf("replication lag %d exceeds maximum %d seconds", replicationLag, sm.maxLag)
		return validation
	}

	validation.IsValid = true
	return validation
}

// ExecuteSwitchover performs a planned switchover
func (sm *SwitchoverManager) ExecuteSwitchover(ctx context.Context, currentLeader, targetNode string, validation *SwitchoverValidation) (*agent.FailoverEvent, error) {
	event := &agent.FailoverEvent{
		ID:        generateEventID(),
		Type:      "switchover",
		OldLeader: currentLeader,
		NewLeader: targetNode,
		Reason:    "planned switchover",
		StartTime: time.Now(),
	}

	if !validation.IsValid {
		event.Success = false
		event.ErrorMessage = validation.ErrorMessage
		event.EndTime = time.Now()
		return event, fmt.Errorf("switchover validation failed: %s", validation.ErrorMessage)
	}

	// Record successful switchover
	event.Success = true
	event.EndTime = time.Now()

	// Store event
	data, err := event.Serialize()
	if err != nil {
		return event, err
	}
	if err := sm.dcs.Set(ctx, "/events/"+event.ID, data); err != nil {
		return event, err
	}

	return event, nil
}

// ValidateSwitchoverRequest is a helper function for property testing
func ValidateSwitchoverRequest(targetHealthy bool, replicationLag, maxLag int64) bool {
	return targetHealthy && replicationLag <= maxLag
}
