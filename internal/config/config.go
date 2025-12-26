// Package config provides configuration management for MySQL HA Agent
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the main configuration structure
type Config struct {
	Name          string      `yaml:"name"`
	Namespace     string      `yaml:"namespace"`
	Scope         string      `yaml:"scope"`
	AdvertiseHost string      `yaml:"advertise_host"` // External IP for other nodes to connect
	Version       string      `yaml:"version"`        // Agent version injected by webadmin
	DCS           DCSConfig   `yaml:"dcs"`
	MySQL         MySQLConfig `yaml:"mysql"`
	API           APIConfig   `yaml:"api"`
	Log           LogConfig   `yaml:"log"`
	HA            HAConfig    `yaml:"ha"`
}

// DCSConfig holds etcd connection settings
type DCSConfig struct {
	Endpoints []string   `yaml:"endpoints"`
	Username  string     `yaml:"username,omitempty"`
	Password  string     `yaml:"password,omitempty"`
	TLS       *TLSConfig `yaml:"tls,omitempty"`
}

// TLSConfig holds TLS settings for etcd connection
type TLSConfig struct {
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	CAFile     string `yaml:"ca_file"`
	SkipVerify bool   `yaml:"skip_verify"`
}

// MySQLConfig holds MySQL connection settings
type MySQLConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	ReplicationUser string `yaml:"replication_user"`
	ReplicationPass string `yaml:"replication_password"`
	DataDir         string `yaml:"data_dir,omitempty"`
}

// APIConfig holds HTTP API server settings
type APIConfig struct {
	Listen   string `yaml:"listen"`
	Port     int    `yaml:"port"`
	APIKey   string `yaml:"api_key,omitempty"`
	CertFile string `yaml:"cert_file,omitempty"`
	KeyFile  string `yaml:"key_file,omitempty"`
}

// LogConfig holds logging settings
type LogConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file,omitempty"`
	MaxSize    int    `yaml:"max_size,omitempty"` // MB
	MaxBackups int    `yaml:"max_backups,omitempty"`
	MaxAge     int    `yaml:"max_age,omitempty"` // days
	Compress   bool   `yaml:"compress,omitempty"`
}

// HAConfig holds high availability settings
type HAConfig struct {
	TTL               int `yaml:"ttl"`                 // Lock TTL in seconds
	LoopWait          int `yaml:"loop_wait"`           // Main loop interval in seconds
	RetryTimeout      int `yaml:"retry_timeout"`       // Retry timeout in seconds
	MaxReplicationLag int `yaml:"max_replication_lag"` // Max replication lag in seconds
	FailoverCooldown  int `yaml:"failover_cooldown"`   // Failover cooldown in seconds
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return ParseConfig(data)
}

// ParseConfig parses configuration from YAML bytes
func ParseConfig(data []byte) (*Config, error) {
	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	applyDefaults(config)
	return config, nil
}
