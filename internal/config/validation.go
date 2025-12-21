package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation error: %s - %s", e.Field, e.Message)
}

// Validate validates the configuration and returns an error if invalid
func (c *Config) Validate() error {
	var errors []string

	// Validate required fields
	if c.Name == "" {
		errors = append(errors, "name is required")
	}

	// Validate DCS config
	if len(c.DCS.Endpoints) == 0 {
		errors = append(errors, "dcs.endpoints is required")
	}
	for i, ep := range c.DCS.Endpoints {
		if ep == "" {
			errors = append(errors, fmt.Sprintf("dcs.endpoints[%d] cannot be empty", i))
		}
	}

	// Validate MySQL config
	if c.MySQL.Host == "" {
		errors = append(errors, "mysql.host is required")
	}
	if c.MySQL.Port < 1 || c.MySQL.Port > 65535 {
		errors = append(errors, "mysql.port must be between 1 and 65535")
	}
	if c.MySQL.User == "" {
		errors = append(errors, "mysql.user is required")
	}
	if c.MySQL.Password == "" {
		errors = append(errors, "mysql.password is required")
	}
	if c.MySQL.ReplicationUser == "" {
		errors = append(errors, "mysql.replication_user is required")
	}
	if c.MySQL.ReplicationPass == "" {
		errors = append(errors, "mysql.replication_password is required")
	}

	// Validate API config
	if c.API.Port < 1 || c.API.Port > 65535 {
		errors = append(errors, "api.port must be between 1 and 65535")
	}

	// Validate Log config
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(c.Log.Level)] {
		errors = append(errors, "log.level must be one of: debug, info, warn, error")
	}

	// Validate HA config
	if c.HA.TTL < 5 {
		errors = append(errors, "ha.ttl must be at least 5 seconds")
	}
	if c.HA.LoopWait < 1 {
		errors = append(errors, "ha.loop_wait must be at least 1 second")
	}
	if c.HA.RetryTimeout < 1 {
		errors = append(errors, "ha.retry_timeout must be at least 1 second")
	}
	if c.HA.MaxReplicationLag < 0 {
		errors = append(errors, "ha.max_replication_lag cannot be negative")
	}
	if c.HA.FailoverCooldown < 0 {
		errors = append(errors, "ha.failover_cooldown cannot be negative")
	}

	if len(errors) > 0 {
		return &ValidationError{
			Field:   "config",
			Message: strings.Join(errors, "; "),
		}
	}

	return nil
}
