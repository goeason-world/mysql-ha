package mysql

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: mysql-ha-solution, Property 12: MySQL Version Command Generation**
// For any MySQL version (5.7.x or 8.0.x), the replication command generator should
// produce syntactically correct commands for that specific version.
func TestMySQLVersionCommandGeneration(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Generator for supported versions
	supportedVersionGen := gen.OneConstOf(
		&Version{Major: 5, Minor: 7, Patch: 0},
		&Version{Major: 5, Minor: 7, Patch: 42},
		&Version{Major: 8, Minor: 0, Patch: 0},
		&Version{Major: 8, Minor: 0, Patch: 33},
	)

	properties.Property("generated commands are valid for version", prop.ForAll(
		func(v *Version) bool {
			rc := NewReplicationCommand(v)

			// Test all command types
			startRepl := rc.StartReplicationSQL("localhost", 3306, "repl", "pass", true)
			startSlave := rc.StartSlaveSQL()
			stopSlave := rc.StopSlaveSQL()
			resetSlave := rc.ResetSlaveSQL()
			showStatus := rc.ShowSlaveStatusSQL()

			// All commands should be valid for this version
			return rc.IsValidReplicationCommand(startRepl) &&
				rc.IsValidReplicationCommand(startSlave) &&
				rc.IsValidReplicationCommand(stopSlave) &&
				rc.IsValidReplicationCommand(resetSlave) &&
				rc.IsValidReplicationCommand(showStatus)
		},
		supportedVersionGen,
	))

	properties.Property("MySQL 8.0 uses REPLICA syntax", prop.ForAll(
		func(patch int) bool {
			v := &Version{Major: 8, Minor: 0, Patch: patch}
			rc := NewReplicationCommand(v)

			startSlave := rc.StartSlaveSQL()
			stopSlave := rc.StopSlaveSQL()
			showStatus := rc.ShowSlaveStatusSQL()

			return strings.Contains(startSlave, "REPLICA") &&
				strings.Contains(stopSlave, "REPLICA") &&
				strings.Contains(showStatus, "REPLICA")
		},
		gen.IntRange(0, 50),
	))

	properties.Property("MySQL 5.7 uses SLAVE syntax", prop.ForAll(
		func(patch int) bool {
			v := &Version{Major: 5, Minor: 7, Patch: patch}
			rc := NewReplicationCommand(v)

			startSlave := rc.StartSlaveSQL()
			stopSlave := rc.StopSlaveSQL()
			showStatus := rc.ShowSlaveStatusSQL()

			return strings.Contains(startSlave, "SLAVE") &&
				strings.Contains(stopSlave, "SLAVE") &&
				strings.Contains(showStatus, "SLAVE")
		},
		gen.IntRange(0, 50),
	))

	properties.Property("MySQL 8.0 uses CHANGE REPLICATION SOURCE", prop.ForAll(
		func(patch int) bool {
			v := &Version{Major: 8, Minor: 0, Patch: patch}
			rc := NewReplicationCommand(v)

			cmd := rc.StartReplicationSQL("localhost", 3306, "repl", "pass", true)
			return strings.Contains(cmd, "CHANGE REPLICATION SOURCE")
		},
		gen.IntRange(0, 50),
	))

	properties.Property("MySQL 5.7 uses CHANGE MASTER TO", prop.ForAll(
		func(patch int) bool {
			v := &Version{Major: 5, Minor: 7, Patch: patch}
			rc := NewReplicationCommand(v)

			cmd := rc.StartReplicationSQL("localhost", 3306, "repl", "pass", true)
			return strings.Contains(cmd, "CHANGE MASTER TO")
		},
		gen.IntRange(0, 50),
	))

	properties.TestingRun(t)
}

// **Feature: mysql-ha-solution, Property 13: Unsupported Version Rejection**
// For any MySQL version string that is not 5.7.x or 8.0.x, the version validator
// should return an error indicating the version is unsupported.
func TestUnsupportedVersionRejection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Generator for unsupported versions
	unsupportedVersionGen := gen.OneConstOf(
		"5.5.62",
		"5.6.51",
		"8.1.0",
		"8.2.0",
		"9.0.0",
		"4.1.0",
		"10.0.0",
	)

	properties.Property("unsupported versions return error", prop.ForAll(
		func(versionStr string) bool {
			err := ValidateVersion(versionStr)
			return err != nil && strings.Contains(err.Error(), "unsupported")
		},
		unsupportedVersionGen,
	))

	// Generator for supported versions
	supportedVersionGen := gen.OneConstOf(
		"5.7.0",
		"5.7.42",
		"5.7.99",
		"8.0.0",
		"8.0.33",
		"8.0.99",
	)

	properties.Property("supported versions do not return error", prop.ForAll(
		func(versionStr string) bool {
			err := ValidateVersion(versionStr)
			return err == nil
		},
		supportedVersionGen,
	))

	properties.TestingRun(t)
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input   string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{"5.7.42", 5, 7, 42, false},
		{"8.0.33", 8, 0, 33, false},
		{"8.0.33-0ubuntu0.20.04.1", 8, 0, 33, false},
		{"5.7.42-log", 5, 7, 42, false},
		{"invalid", 0, 0, 0, true},
		{"", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if v.Major != tt.major || v.Minor != tt.minor || v.Patch != tt.patch {
				t.Errorf("Got %d.%d.%d, want %d.%d.%d", v.Major, v.Minor, v.Patch, tt.major, tt.minor, tt.patch)
			}
		})
	}
}

func TestVersionIsSupported(t *testing.T) {
	tests := []struct {
		major     int
		minor     int
		supported bool
	}{
		{5, 7, true},
		{8, 0, true},
		{5, 6, false},
		{5, 5, false},
		{8, 1, false},
		{9, 0, false},
	}

	for _, tt := range tests {
		v := &Version{Major: tt.major, Minor: tt.minor}
		if v.IsSupported() != tt.supported {
			t.Errorf("Version %d.%d: expected supported=%v, got %v", tt.major, tt.minor, tt.supported, v.IsSupported())
		}
	}
}
