// Package agent provides the core HA agent functionality
package agent

import (
	"encoding/json"
	"fmt"
	"time"
)

// NodeRole defines the role of a node in the cluster
type NodeRole string

const (
	RoleUnknown NodeRole = "unknown"
	RoleLeader  NodeRole = "leader"
	RoleReplica NodeRole = "replica"
	RoleStandby NodeRole = "standby"
)

// NodeState represents the state of a single node
type NodeState struct {
	NodeID          string           `json:"node_id"`
	Hostname        string           `json:"hostname"`
	Role            NodeRole         `json:"role"`
	AgentVersion    string           `json:"agent_version"`
	MySQLVersion    string           `json:"mysql_version"`
	GTIDExecuted    string           `json:"gtid_executed"`
	ReplicationLag  int64            `json:"replication_lag"`
	IsHealthy       bool             `json:"is_healthy"`
	LastUpdated     time.Time        `json:"last_updated"`
	ReplicationInfo *ReplicationInfo `json:"replication_info,omitempty"`
}

// ReplicationInfo contains MySQL replication details
type ReplicationInfo struct {
	MasterHost       string `json:"master_host,omitempty"`
	MasterPort       int    `json:"master_port,omitempty"`
	SlaveIORunning   bool   `json:"slave_io_running"`
	SlaveSQLRunning  bool   `json:"slave_sql_running"`
	RetrievedGTIDSet string `json:"retrieved_gtid_set,omitempty"`
	ExecutedGTIDSet  string `json:"executed_gtid_set,omitempty"`
	LastError        string `json:"last_error,omitempty"`
}

// ClusterState represents the state of the entire cluster
type ClusterState struct {
	ClusterName  string         `json:"cluster_name"`
	Leader       *NodeState     `json:"leader,omitempty"`
	Members      []*NodeState   `json:"members"`
	LastFailover *FailoverEvent `json:"last_failover,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// FailoverEvent represents a failover or switchover event
type FailoverEvent struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"` // "failover" or "switchover"
	OldLeader    string    `json:"old_leader"`
	NewLeader    string    `json:"new_leader"`
	Reason       string    `json:"reason"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// ReplicationStatus contains MySQL replication status
type ReplicationStatus struct {
	SlaveIORunning      bool   `json:"slave_io_running"`
	SlaveSQLRunning     bool   `json:"slave_sql_running"`
	MasterHost          string `json:"master_host"`
	MasterPort          int    `json:"master_port"`
	SecondsBehindMaster *int64 `json:"seconds_behind_master"`
	GTIDExecuted        string `json:"gtid_executed"`
	RetrievedGTIDSet    string `json:"retrieved_gtid_set"`
	ExecutedGTIDSet     string `json:"executed_gtid_set"`
	LastError           string `json:"last_error,omitempty"`
}

// Serialize serializes ClusterState to JSON
func (cs *ClusterState) Serialize() ([]byte, error) {
	return json.MarshalIndent(cs, "", "  ")
}

// DeserializeClusterState deserializes JSON to ClusterState
func DeserializeClusterState(data []byte) (*ClusterState, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	// Check for JSON null
	if string(data) == "null" {
		return nil, fmt.Errorf("null is not a valid cluster state")
	}

	var cs ClusterState
	if err := json.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("failed to deserialize cluster state: %w", err)
	}
	return &cs, nil
}

// Serialize serializes NodeState to JSON
func (ns *NodeState) Serialize() ([]byte, error) {
	return json.MarshalIndent(ns, "", "  ")
}

// DeserializeNodeState deserializes JSON to NodeState
func DeserializeNodeState(data []byte) (*NodeState, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	// Check for JSON null
	if string(data) == "null" {
		return nil, fmt.Errorf("null is not a valid node state")
	}

	var ns NodeState
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("failed to deserialize node state: %w", err)
	}
	return &ns, nil
}

// Serialize serializes FailoverEvent to JSON
func (fe *FailoverEvent) Serialize() ([]byte, error) {
	return json.MarshalIndent(fe, "", "  ")
}

// DeserializeFailoverEvent deserializes JSON to FailoverEvent
func DeserializeFailoverEvent(data []byte) (*FailoverEvent, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	// Check for JSON null
	if string(data) == "null" {
		return nil, fmt.Errorf("null is not a valid failover event")
	}

	var fe FailoverEvent
	if err := json.Unmarshal(data, &fe); err != nil {
		return nil, fmt.Errorf("failed to deserialize failover event: %w", err)
	}
	return &fe, nil
}

// HasRequiredFields checks if NodeState has all required fields for registration
func (ns *NodeState) HasRequiredFields() bool {
	return ns.NodeID != "" &&
		ns.Hostname != "" &&
		ns.MySQLVersion != "" &&
		ns.Role != "" &&
		!ns.LastUpdated.IsZero()
}

// HasRequiredFields checks if FailoverEvent has all required fields
func (fe *FailoverEvent) HasRequiredFields() bool {
	return fe.ID != "" &&
		fe.Type != "" &&
		fe.OldLeader != "" &&
		fe.NewLeader != "" &&
		fe.Reason != "" &&
		!fe.StartTime.IsZero() &&
		!fe.EndTime.IsZero()
}
