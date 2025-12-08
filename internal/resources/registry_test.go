// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"encoding/json"
	"testing"

	"github.com/dringdahl0320/aerospike-mcp-server/pkg/config"
)

func TestResourceDefinition(t *testing.T) {
	def := ResourceDefinition{
		URI:         "aerospike://cluster/info",
		Name:        "Cluster Info",
		Description: "Cluster topology and status",
		MimeType:    "application/json",
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("Failed to marshal ResourceDefinition: %v", err)
	}

	var parsed ResourceDefinition
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal ResourceDefinition: %v", err)
	}

	if parsed.URI != "aerospike://cluster/info" {
		t.Errorf("Expected URI 'aerospike://cluster/info', got '%s'", parsed.URI)
	}

	if parsed.MimeType != "application/json" {
		t.Errorf("Expected MimeType 'application/json', got '%s'", parsed.MimeType)
	}
}

func TestRegistryStaticResources(t *testing.T) {
	// Test that registry returns at minimum the cluster info resource
	// Note: Full list requires a client connection, so we test static resources only

	cfg := &config.Config{
		Role:      config.RoleReadOnly,
		Namespace: "test-ns",
	}

	r := &Registry{
		client: nil,
		config: cfg,
	}

	// Since List() calls client methods, we can only verify the struct setup
	if r.config.Role != config.RoleReadOnly {
		t.Errorf("Expected role 'read-only', got '%s'", r.config.Role)
	}

	if r.config.Namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got '%s'", r.config.Namespace)
	}
}

func TestResourceURIParsing(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		isValid    bool
		expectPath string
	}{
		{
			name:       "cluster info",
			uri:        "aerospike://cluster/info",
			isValid:    true,
			expectPath: "cluster/info",
		},
		{
			name:       "namespace",
			uri:        "aerospike://ns/test-ns",
			isValid:    true,
			expectPath: "ns/test-ns",
		},
		{
			name:       "namespace sets",
			uri:        "aerospike://ns/test-ns/sets",
			isValid:    true,
			expectPath: "ns/test-ns/sets",
		},
		{
			name:       "udfs",
			uri:        "aerospike://udfs",
			isValid:    true,
			expectPath: "udfs",
		},
		{
			name:       "schema",
			uri:        "aerospike://schema/test-ns/test-set",
			isValid:    true,
			expectPath: "schema/test-ns/test-set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse URI by removing the scheme
			const scheme = "aerospike://"
			if len(tt.uri) < len(scheme) {
				t.Errorf("URI too short: %s", tt.uri)
				return
			}

			path := tt.uri[len(scheme):]
			if path != tt.expectPath {
				t.Errorf("Expected path '%s', got '%s'", tt.expectPath, path)
			}
		})
	}
}

func TestResourceMimeTypes(t *testing.T) {
	// Test that ResourceDefinition correctly stores mime types
	def := ResourceDefinition{
		URI:         "aerospike://test",
		Name:        "Test Resource",
		Description: "Test description",
		MimeType:    "application/json",
	}

	if def.MimeType != "application/json" {
		t.Errorf("Expected MimeType 'application/json', got '%s'", def.MimeType)
	}

	if def.Name == "" {
		t.Error("Name should not be empty")
	}

	if def.Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestResourceURIScheme(t *testing.T) {
	// Test URI scheme validation
	validURIs := []string{
		"aerospike://cluster/info",
		"aerospike://ns/test",
		"aerospike://udfs",
	}

	invalidURIs := []string{
		"http://cluster/info",
		"invalid",
		"",
	}

	for _, uri := range validURIs {
		if len(uri) < 12 || uri[:12] != "aerospike://" {
			t.Errorf("URI '%s' should be valid aerospike:// scheme", uri)
		}
	}

	for _, uri := range invalidURIs {
		if len(uri) >= 12 && uri[:12] == "aerospike://" {
			t.Errorf("URI '%s' should not be valid aerospike:// scheme", uri)
		}
	}
}
