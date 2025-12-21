package config

// Default configuration values
const (
	DefaultMySQLPort         = 3306
	DefaultAPIListen         = "0.0.0.0"
	DefaultAPIPort           = 8080
	DefaultLogLevel          = "info"
	DefaultLogMaxSize        = 100 // MB
	DefaultLogMaxBackups     = 3
	DefaultLogMaxAge         = 7 // days
	DefaultTTL               = 30
	DefaultLoopWait          = 10
	DefaultRetryTimeout      = 30
	DefaultMaxReplicationLag = 10
	DefaultFailoverCooldown  = 60
)

// applyDefaults applies default values to missing configuration fields
func applyDefaults(c *Config) {
	// MySQL defaults
	if c.MySQL.Port == 0 {
		c.MySQL.Port = DefaultMySQLPort
	}

	// API defaults
	if c.API.Listen == "" {
		c.API.Listen = DefaultAPIListen
	}
	if c.API.Port == 0 {
		c.API.Port = DefaultAPIPort
	}

	// Log defaults
	if c.Log.Level == "" {
		c.Log.Level = DefaultLogLevel
	}
	if c.Log.MaxSize == 0 {
		c.Log.MaxSize = DefaultLogMaxSize
	}
	if c.Log.MaxBackups == 0 {
		c.Log.MaxBackups = DefaultLogMaxBackups
	}
	if c.Log.MaxAge == 0 {
		c.Log.MaxAge = DefaultLogMaxAge
	}

	// HA defaults
	if c.HA.TTL == 0 {
		c.HA.TTL = DefaultTTL
	}
	if c.HA.LoopWait == 0 {
		c.HA.LoopWait = DefaultLoopWait
	}
	if c.HA.RetryTimeout == 0 {
		c.HA.RetryTimeout = DefaultRetryTimeout
	}
	if c.HA.MaxReplicationLag == 0 {
		c.HA.MaxReplicationLag = DefaultMaxReplicationLag
	}
	if c.HA.FailoverCooldown == 0 {
		c.HA.FailoverCooldown = DefaultFailoverCooldown
	}
}
