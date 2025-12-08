// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if len(cfg.Hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(cfg.Hosts))
	}

	if cfg.Hosts[0].Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", cfg.Hosts[0].Host)
	}

	if cfg.Hosts[0].Port != 3000 {
		t.Errorf("Expected port 3000, got %d", cfg.Hosts[0].Port)
	}

	if cfg.Role != RoleReadOnly {
		t.Errorf("Expected role '%s', got '%s'", RoleReadOnly, cfg.Role)
	}

	if cfg.Transport != "stdio" {
		t.Errorf("Expected transport 'stdio', got '%s'", cfg.Transport)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid default",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "no hosts",
			config: &Config{
				Hosts:     []Host{},
				Role:      RoleReadOnly,
				Transport: "stdio",
			},
			wantErr: true,
		},
		{
			name: "empty host address",
			config: &Config{
				Hosts:     []Host{{Host: "", Port: 3000}},
				Role:      RoleReadOnly,
				Transport: "stdio",
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			config: &Config{
				Hosts:     []Host{{Host: "localhost", Port: 0}},
				Role:      RoleReadOnly,
				Transport: "stdio",
			},
			wantErr: true,
		},
		{
			name: "invalid role",
			config: &Config{
				Hosts:     []Host{{Host: "localhost", Port: 3000}},
				Role:      "invalid",
				Transport: "stdio",
			},
			wantErr: true,
		},
		{
			name: "invalid transport",
			config: &Config{
				Hosts:     []Host{{Host: "localhost", Port: 3000}},
				Role:      RoleReadOnly,
				Transport: "invalid",
			},
			wantErr: true,
		},
		{
			name: "all roles valid",
			config: &Config{
				Hosts:     []Host{{Host: "localhost", Port: 3000}},
				Role:      RoleAdmin,
				Transport: "sse",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCanWrite(t *testing.T) {
	tests := []struct {
		role     Role
		canWrite bool
	}{
		{RoleReadOnly, false},
		{RoleReadWrite, true},
		{RoleAdmin, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			cfg := &Config{Role: tt.role}
			if cfg.CanWrite() != tt.canWrite {
				t.Errorf("CanWrite() = %v, want %v", cfg.CanWrite(), tt.canWrite)
			}
		})
	}
}

func TestCanAdmin(t *testing.T) {
	tests := []struct {
		role     Role
		canAdmin bool
	}{
		{RoleReadOnly, false},
		{RoleReadWrite, false},
		{RoleAdmin, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			cfg := &Config{Role: tt.role}
			if cfg.CanAdmin() != tt.canAdmin {
				t.Errorf("CanAdmin() = %v, want %v", cfg.CanAdmin(), tt.canAdmin)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
		"hosts": [{"host": "testhost", "port": 3001}],
		"role": "admin",
		"transport": "sse",
		"timeout_ms": 500
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Hosts[0].Host != "testhost" {
		t.Errorf("Expected host 'testhost', got '%s'", cfg.Hosts[0].Host)
	}

	if cfg.Role != RoleAdmin {
		t.Errorf("Expected role 'admin', got '%s'", cfg.Role)
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
		"hosts": [{"host": "localhost", "port": 3000}],
		"password_env": "TEST_AEROSPIKE_PASSWORD",
		"transport": "stdio"
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set env var
	os.Setenv("TEST_AEROSPIKE_PASSWORD", "secret123")
	defer os.Unsetenv("TEST_AEROSPIKE_PASSWORD")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Password != "secret123" {
		t.Errorf("Expected password 'secret123', got '%s'", cfg.Password)
	}
}
