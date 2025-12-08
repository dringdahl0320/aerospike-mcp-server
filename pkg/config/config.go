// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration types and loading for the Aerospike MCP server.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Host represents an Aerospike cluster node.
type Host struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// TLSConfig holds TLS configuration options.
type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CAFile   string `json:"ca_file,omitempty"`
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
}

// Role defines the permission level for database operations.
type Role string

const (
	RoleReadOnly  Role = "read-only"
	RoleReadWrite Role = "read-write"
	RoleAdmin     Role = "admin"
)

// Config holds the complete configuration for the Aerospike MCP server.
type Config struct {
	// Cluster connection settings
	Hosts     []Host `json:"hosts"`
	Namespace string `json:"namespace,omitempty"`

	// Authentication
	User        string `json:"user,omitempty"`
	Password    string `json:"password,omitempty"`
	PasswordEnv string `json:"password_env,omitempty"`

	// TLS configuration
	TLS TLSConfig `json:"tls,omitempty"`

	// Authorization
	Role Role `json:"role"`

	// Client settings
	TimeoutMs  int `json:"timeout_ms"`
	MaxRetries int `json:"max_retries"`

	// Safety constraints
	DefaultMaxRecords int `json:"default_max_records"`
	MaxBatchSize      int `json:"max_batch_size"`

	// Server settings
	Transport string `json:"transport"` // "stdio", "sse", "websocket"
	Port      int    `json:"port,omitempty"`

	// Audit settings
	Audit AuditConfig `json:"audit,omitempty"`
}

// AuditConfig holds audit logging configuration.
type AuditConfig struct {
	Enabled          bool    `json:"enabled"`
	FilePath         string  `json:"file_path,omitempty"`
	BufferSize       int     `json:"buffer_size"`
	RateLimitEnabled bool    `json:"rate_limit_enabled"`
	RateLimitRPS     float64 `json:"rate_limit_rps"`
	RateLimitBurst   int     `json:"rate_limit_burst"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Hosts: []Host{
			{Host: "localhost", Port: 3000},
		},
		Role:              RoleReadOnly,
		TimeoutMs:         1000,
		MaxRetries:        2,
		DefaultMaxRecords: 1000,
		MaxBatchSize:      5000,
		Transport:         "stdio",
		Audit: AuditConfig{
			Enabled:          true,
			BufferSize:       100,
			RateLimitEnabled: true,
			RateLimitRPS:     100,
			RateLimitBurst:   200,
		},
	}
}

// Load reads configuration from a file path or uses defaults.
// If configPath is empty, it checks for AEROSPIKE_MCP_CONFIG env var.
func Load(configPath string) (*Config, error) {
	// Check environment variable if no path provided
	if configPath == "" {
		configPath = os.Getenv("AEROSPIKE_MCP_CONFIG")
	}

	cfg := DefaultConfig()

	// If still no config path, return defaults
	if configPath == "" {
		return cfg, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Resolve password from environment variable if specified
	if cfg.PasswordEnv != "" && cfg.Password == "" {
		cfg.Password = os.Getenv(cfg.PasswordEnv)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if len(c.Hosts) == 0 {
		return fmt.Errorf("at least one host must be specified")
	}

	for i, host := range c.Hosts {
		if host.Host == "" {
			return fmt.Errorf("host[%d]: host address is required", i)
		}
		if host.Port <= 0 || host.Port > 65535 {
			return fmt.Errorf("host[%d]: invalid port %d", i, host.Port)
		}
	}

	switch c.Role {
	case RoleReadOnly, RoleReadWrite, RoleAdmin:
		// Valid roles
	case "":
		c.Role = RoleReadOnly
	default:
		return fmt.Errorf("invalid role: %s (must be read-only, read-write, or admin)", c.Role)
	}

	validTransports := []string{"stdio", "sse", "websocket"}
	transportValid := false
	for _, t := range validTransports {
		if strings.EqualFold(c.Transport, t) {
			transportValid = true
			break
		}
	}
	if !transportValid {
		return fmt.Errorf("invalid transport: %s (must be stdio, sse, or websocket)", c.Transport)
	}

	if c.TimeoutMs <= 0 {
		c.TimeoutMs = 1000
	}

	if c.MaxRetries < 0 {
		c.MaxRetries = 2
	}

	if c.DefaultMaxRecords <= 0 {
		c.DefaultMaxRecords = 1000
	}

	if c.MaxBatchSize <= 0 {
		c.MaxBatchSize = 5000
	}

	return nil
}

// CanWrite returns true if the role permits write operations.
func (c *Config) CanWrite() bool {
	return c.Role == RoleReadWrite || c.Role == RoleAdmin
}

// CanAdmin returns true if the role permits administrative operations.
func (c *Config) CanAdmin() bool {
	return c.Role == RoleAdmin
}
