package config

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: mysql-ha-solution, Property 3: Configuration Default Values**
// For any partial configuration with missing optional fields, loading the configuration
// should produce a Config object where all missing fields have their documented default values.
func TestConfigurationDefaults(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Generator for non-empty strings
	nonEmptyString := gen.Identifier().Map(func(s string) string {
		if s == "" {
			return "default"
		}
		return s
	})

	properties.Property("missing optional fields get default values", prop.ForAll(
		func(name, endpoint, host, user, pass, replUser, replPass string) bool {
			// Create minimal config YAML with only required fields
			yaml := `
name: ` + name + `
dcs:
  endpoints:
    - ` + endpoint + `
mysql:
  host: ` + host + `
  user: ` + user + `
  password: ` + pass + `
  replication_user: ` + replUser + `
  replication_password: ` + replPass + `
`
			config, err := ParseConfig([]byte(yaml))
			if err != nil {
				return false
			}

			// Verify all defaults are applied
			return config.MySQL.Port == DefaultMySQLPort &&
				config.API.Listen == DefaultAPIListen &&
				config.API.Port == DefaultAPIPort &&
				config.Log.Level == DefaultLogLevel &&
				config.Log.MaxSize == DefaultLogMaxSize &&
				config.Log.MaxBackups == DefaultLogMaxBackups &&
				config.Log.MaxAge == DefaultLogMaxAge &&
				config.HA.TTL == DefaultTTL &&
				config.HA.LoopWait == DefaultLoopWait &&
				config.HA.RetryTimeout == DefaultRetryTimeout &&
				config.HA.MaxReplicationLag == DefaultMaxReplicationLag &&
				config.HA.FailoverCooldown == DefaultFailoverCooldown
		},
		nonEmptyString,
		nonEmptyString,
		nonEmptyString,
		nonEmptyString,
		nonEmptyString,
		nonEmptyString,
		nonEmptyString,
	))

	properties.TestingRun(t)
}

// **Feature: mysql-ha-solution, Property 5: Configuration Validation**
// For any configuration with invalid values, the configuration parser should return
// a descriptive error message identifying the specific validation failure.
func TestConfigurationValidation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Test missing required fields produce validation errors
	properties.Property("missing name produces validation error", prop.ForAll(
		func(endpoints []string, host, user, pass, replUser, replPass string) bool {
			if len(endpoints) == 0 || host == "" || user == "" || pass == "" || replUser == "" || replPass == "" {
				return true
			}

			config := &Config{
				Name: "", // Missing required field
				DCS:  DCSConfig{Endpoints: endpoints},
				MySQL: MySQLConfig{
					Host:            host,
					Port:            3306,
					User:            user,
					Password:        pass,
					ReplicationUser: replUser,
					ReplicationPass: replPass,
				},
			}
			applyDefaults(config)
			err := config.Validate()
			return err != nil
		},
		gen.SliceOfN(1, gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 })),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	// Test invalid port range produces validation error
	properties.Property("invalid port produces validation error", prop.ForAll(
		func(port int) bool {
			if port >= 1 && port <= 65535 {
				return true // Valid port, skip
			}

			config := &Config{
				Name: "test",
				DCS:  DCSConfig{Endpoints: []string{"localhost:2379"}},
				MySQL: MySQLConfig{
					Host:            "localhost",
					Port:            port,
					User:            "root",
					Password:        "pass",
					ReplicationUser: "repl",
					ReplicationPass: "replpass",
				},
			}
			applyDefaults(config)
			err := config.Validate()
			return err != nil
		},
		gen.IntRange(-1000, 70000),
	))

	// Test invalid log level produces validation error
	properties.Property("invalid log level produces validation error", prop.ForAll(
		func(level string) bool {
			validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
			if validLevels[level] {
				return true // Valid level, skip
			}

			config := &Config{
				Name: "test",
				DCS:  DCSConfig{Endpoints: []string{"localhost:2379"}},
				MySQL: MySQLConfig{
					Host:            "localhost",
					Port:            3306,
					User:            "root",
					Password:        "pass",
					ReplicationUser: "repl",
					ReplicationPass: "replpass",
				},
				Log: LogConfig{Level: level},
			}
			applyDefaults(config)
			// Override the default level with invalid one
			config.Log.Level = level
			err := config.Validate()
			return err != nil
		},
		gen.AlphaString().SuchThat(func(s string) bool {
			validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "": true}
			return !validLevels[s]
		}),
	))

	properties.TestingRun(t)
}
