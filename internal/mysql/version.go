// Package mysql provides MySQL management functionality
package mysql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a MySQL version
type Version struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

// SupportedVersions defines the supported MySQL versions
var SupportedVersions = []struct {
	Major int
	Minor int
}{
	{5, 7},
	{8, 0},
}

// ParseVersion parses a MySQL version string
func ParseVersion(versionStr string) (*Version, error) {
	// MySQL version format: 5.7.42, 8.0.33, 8.0.33-0ubuntu0.20.04.1
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(versionStr)
	if len(matches) < 4 {
		return nil, fmt.Errorf("invalid version format: %s", versionStr)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Raw:   versionStr,
	}, nil
}

// IsSupported checks if the version is supported
func (v *Version) IsSupported() bool {
	for _, sv := range SupportedVersions {
		if v.Major == sv.Major && v.Minor == sv.Minor {
			return true
		}
	}
	return false
}

// Is57 returns true if this is MySQL 5.7.x
func (v *Version) Is57() bool {
	return v.Major == 5 && v.Minor == 7
}

// Is80 returns true if this is MySQL 8.0.x
func (v *Version) Is80() bool {
	return v.Major == 8 && v.Minor == 0
}

// String returns the version string
func (v *Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ValidateVersion validates a version string and returns an error if unsupported
func ValidateVersion(versionStr string) error {
	v, err := ParseVersion(versionStr)
	if err != nil {
		return err
	}
	if !v.IsSupported() {
		return fmt.Errorf("unsupported MySQL version: %s (supported: 5.7.x, 8.0.x)", versionStr)
	}
	return nil
}

// ReplicationCommand generates version-appropriate replication commands
type ReplicationCommand struct {
	Version *Version
}

// NewReplicationCommand creates a new ReplicationCommand for the given version
func NewReplicationCommand(v *Version) *ReplicationCommand {
	return &ReplicationCommand{Version: v}
}

// StartReplicationSQL generates the SQL to start replication
func (rc *ReplicationCommand) StartReplicationSQL(masterHost string, masterPort int, user, password string, useGTID bool) string {
	if rc.Version.Is80() {
		// MySQL 8.0 uses CHANGE REPLICATION SOURCE
		// 添加 GET_SOURCE_PUBLIC_KEY=1 解决 caching_sha2_password 认证问题
		if useGTID {
			return fmt.Sprintf(
				"CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_PORT=%d, SOURCE_USER='%s', SOURCE_PASSWORD='%s', SOURCE_AUTO_POSITION=1, GET_SOURCE_PUBLIC_KEY=1",
				masterHost, masterPort, user, password,
			)
		}
		return fmt.Sprintf(
			"CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_PORT=%d, SOURCE_USER='%s', SOURCE_PASSWORD='%s', GET_SOURCE_PUBLIC_KEY=1",
			masterHost, masterPort, user, password,
		)
	}

	// MySQL 5.7 uses CHANGE MASTER TO
	if useGTID {
		return fmt.Sprintf(
			"CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1",
			masterHost, masterPort, user, password,
		)
	}
	return fmt.Sprintf(
		"CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s'",
		masterHost, masterPort, user, password,
	)
}

// StartSlaveSQL generates the SQL to start the slave/replica
func (rc *ReplicationCommand) StartSlaveSQL() string {
	if rc.Version.Is80() {
		return "START REPLICA"
	}
	return "START SLAVE"
}

// StopSlaveSQL generates the SQL to stop the slave/replica
func (rc *ReplicationCommand) StopSlaveSQL() string {
	if rc.Version.Is80() {
		return "STOP REPLICA"
	}
	return "STOP SLAVE"
}

// ResetSlaveSQL generates the SQL to reset the slave/replica
func (rc *ReplicationCommand) ResetSlaveSQL() string {
	if rc.Version.Is80() {
		return "RESET REPLICA ALL"
	}
	return "RESET SLAVE ALL"
}

// ShowSlaveStatusSQL generates the SQL to show slave/replica status
func (rc *ReplicationCommand) ShowSlaveStatusSQL() string {
	if rc.Version.Is80() {
		return "SHOW REPLICA STATUS"
	}
	return "SHOW SLAVE STATUS"
}

// IsValidReplicationCommand checks if a command is syntactically valid for the version
func (rc *ReplicationCommand) IsValidReplicationCommand(cmd string) bool {
	cmd = strings.ToUpper(strings.TrimSpace(cmd))

	if rc.Version.Is80() {
		// MySQL 8.0 commands
		validPrefixes := []string{
			"CHANGE REPLICATION SOURCE",
			"START REPLICA",
			"STOP REPLICA",
			"RESET REPLICA",
			"SHOW REPLICA STATUS",
		}
		for _, prefix := range validPrefixes {
			if strings.HasPrefix(cmd, prefix) {
				return true
			}
		}
		return false
	}

	// MySQL 5.7 commands
	validPrefixes := []string{
		"CHANGE MASTER TO",
		"START SLAVE",
		"STOP SLAVE",
		"RESET SLAVE",
		"SHOW SLAVE STATUS",
	}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}
