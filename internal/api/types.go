package api

import "fmt"

// SwitchoverRequest represents a switchover API request
type SwitchoverRequest struct {
	TargetNodeID string `json:"target_node_id"`
	Reason       string `json:"reason,omitempty"`
}

// Validate validates the switchover request
func (r *SwitchoverRequest) Validate() error {
	if r.TargetNodeID == "" {
		return fmt.Errorf("target_node_id is required")
	}
	return nil
}

// ClusterResponse represents the cluster status response
type ClusterResponse struct {
	ClusterName string         `json:"cluster_name"`
	Leader      *NodeResponse  `json:"leader,omitempty"`
	Nodes       []NodeResponse `json:"nodes"`
}

// NodeResponse represents a node in API responses
type NodeResponse struct {
	NodeID         string `json:"node_id"`
	Hostname       string `json:"hostname"`
	Role           string `json:"role"`
	MySQLVersion   string `json:"mysql_version"`
	GTIDExecuted   string `json:"gtid_executed"`
	ReplicationLag int64  `json:"replication_lag"`
	IsHealthy      bool   `json:"is_healthy"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// ValidateRequest validates any API request with required fields
func ValidateRequest(fields map[string]string) error {
	for field, value := range fields {
		if value == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
}
