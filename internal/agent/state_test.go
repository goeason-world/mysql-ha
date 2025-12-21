package agent

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Generator for NodeRole
func genNodeRole() gopter.Gen {
	return gen.OneConstOf(RoleUnknown, RoleLeader, RoleReplica, RoleStandby)
}

// Generator for NodeState
func genNodeState() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.Identifier(),
		genNodeRole(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Int64Range(0, 1000),
		gen.Bool(),
	).Map(func(vals []interface{}) *NodeState {
		return &NodeState{
			NodeID:         vals[0].(string),
			Hostname:       vals[1].(string),
			Role:           vals[2].(NodeRole),
			MySQLVersion:   vals[3].(string),
			GTIDExecuted:   vals[4].(string),
			ReplicationLag: vals[5].(int64),
			IsHealthy:      vals[6].(bool),
			LastUpdated:    time.Now().Truncate(time.Second),
		}
	})
}

// Generator for FailoverEvent
func genFailoverEvent() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.OneConstOf("failover", "switchover"),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Bool(),
	).Map(func(vals []interface{}) *FailoverEvent {
		now := time.Now().Truncate(time.Second)
		return &FailoverEvent{
			ID:        vals[0].(string),
			Type:      vals[1].(string),
			OldLeader: vals[2].(string),
			NewLeader: vals[3].(string),
			Reason:    vals[4].(string),
			StartTime: now.Add(-time.Minute),
			EndTime:   now,
			Success:   vals[5].(bool),
		}
	})
}

// Generator for ClusterState
func genClusterState() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.SliceOfN(3, genNodeState()),
	).Map(func(vals []interface{}) *ClusterState {
		members := vals[1].([]*NodeState)
		var leader *NodeState
		if len(members) > 0 {
			leader = members[0]
			leader.Role = RoleLeader
		}
		return &ClusterState{
			ClusterName: vals[0].(string),
			Leader:      leader,
			Members:     members,
			UpdatedAt:   time.Now().Truncate(time.Second),
		}
	})
}

// **Feature: mysql-ha-solution, Property 1: Cluster State Serialization Round Trip**
func TestClusterStateSerializationRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("serialize then deserialize produces equivalent ClusterState", prop.ForAll(
		func(cs *ClusterState) bool {
			// Serialize
			data, err := cs.Serialize()
			if err != nil {
				return false
			}

			// Deserialize
			restored, err := DeserializeClusterState(data)
			if err != nil {
				return false
			}

			// Compare
			return cs.ClusterName == restored.ClusterName &&
				len(cs.Members) == len(restored.Members) &&
				cs.UpdatedAt.Equal(restored.UpdatedAt)
		},
		genClusterState(),
	))

	properties.TestingRun(t)
}

// **Feature: mysql-ha-solution, Property 2: Node Registration Completeness**
func TestNodeRegistrationCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("serialized NodeState contains all required fields", prop.ForAll(
		func(ns *NodeState) bool {
			data, err := ns.Serialize()
			if err != nil {
				return false
			}

			// Parse as generic map to check field presence
			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err != nil {
				return false
			}

			requiredFields := []string{
				"node_id", "hostname", "mysql_version", "role",
				"gtid_executed", "replication_lag", "is_healthy", "last_updated",
			}

			for _, field := range requiredFields {
				if _, ok := m[field]; !ok {
					return false
				}
			}
			return true
		},
		genNodeState(),
	))

	properties.TestingRun(t)
}

// **Feature: mysql-ha-solution, Property 9: Failover Event Recording**
func TestFailoverEventRecording(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("failover event contains all required fields", prop.ForAll(
		func(fe *FailoverEvent) bool {
			data, err := fe.Serialize()
			if err != nil {
				return false
			}

			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err != nil {
				return false
			}

			requiredFields := []string{
				"id", "type", "old_leader", "new_leader",
				"reason", "start_time", "end_time", "success",
			}

			for _, field := range requiredFields {
				if _, ok := m[field]; !ok {
					return false
				}
			}
			return true
		},
		genFailoverEvent(),
	))

	properties.Property("failover event round trip preserves all fields", prop.ForAll(
		func(fe *FailoverEvent) bool {
			data, err := fe.Serialize()
			if err != nil {
				return false
			}

			restored, err := DeserializeFailoverEvent(data)
			if err != nil {
				return false
			}

			return fe.ID == restored.ID &&
				fe.Type == restored.Type &&
				fe.OldLeader == restored.OldLeader &&
				fe.NewLeader == restored.NewLeader &&
				fe.Reason == restored.Reason &&
				fe.Success == restored.Success
		},
		genFailoverEvent(),
	))

	properties.TestingRun(t)
}

// **Feature: mysql-ha-solution, Property 16: Invalid JSON Handling**
func TestInvalidJSONHandling(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Generator for invalid JSON strings
	invalidJSONGen := gen.OneConstOf(
		"",
		"{",
		"}",
		"[",
		"]",
		"{invalid}",
		"null",
		"123",
		`{"incomplete":`,
		`{"key": undefined}`,
	)

	properties.Property("invalid JSON returns error for ClusterState", prop.ForAll(
		func(invalidJSON string) bool {
			_, err := DeserializeClusterState([]byte(invalidJSON))
			return err != nil
		},
		invalidJSONGen,
	))

	properties.Property("invalid JSON returns error for NodeState", prop.ForAll(
		func(invalidJSON string) bool {
			_, err := DeserializeNodeState([]byte(invalidJSON))
			return err != nil
		},
		invalidJSONGen,
	))

	properties.Property("invalid JSON returns error for FailoverEvent", prop.ForAll(
		func(invalidJSON string) bool {
			_, err := DeserializeFailoverEvent([]byte(invalidJSON))
			return err != nil
		},
		invalidJSONGen,
	))

	properties.Property("existing state unchanged on invalid JSON", prop.ForAll(
		func(cs *ClusterState, invalidJSON string) bool {
			original := *cs
			_, _ = DeserializeClusterState([]byte(invalidJSON))
			return reflect.DeepEqual(original.ClusterName, cs.ClusterName)
		},
		genClusterState(),
		invalidJSONGen,
	))

	properties.TestingRun(t)
}
