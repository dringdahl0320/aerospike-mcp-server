// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"encoding/json"
	"testing"

	"github.com/dringdahl0320/aerospike-mcp-server/pkg/config"
)

func TestToolDefinition(t *testing.T) {
	def := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"param1": {Type: "string", Description: "A string parameter"},
				"param2": {Type: "integer", Description: "An integer parameter"},
			},
			Required: []string{"param1"},
		},
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("Failed to marshal ToolDefinition: %v", err)
	}

	var parsed ToolDefinition
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal ToolDefinition: %v", err)
	}

	if parsed.Name != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", parsed.Name)
	}

	if len(parsed.InputSchema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(parsed.InputSchema.Properties))
	}

	if len(parsed.InputSchema.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(parsed.InputSchema.Required))
	}
}

func TestInputSchema(t *testing.T) {
	schema := InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"namespace": {Type: "string", Description: "Target namespace"},
			"key":       {Type: "string", Description: "Primary key"},
			"bins":      {Type: "array", Items: &Property{Type: "string"}},
		},
		Required: []string{"namespace", "key"},
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal InputSchema: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal InputSchema: %v", err)
	}

	if parsed["type"] != "object" {
		t.Errorf("Expected type 'object', got '%v'", parsed["type"])
	}

	props := parsed["properties"].(map[string]interface{})
	if len(props) != 3 {
		t.Errorf("Expected 3 properties, got %d", len(props))
	}
}

func TestProperty(t *testing.T) {
	tests := []struct {
		name     string
		prop     Property
		expected string
	}{
		{
			name:     "string type",
			prop:     Property{Type: "string", Description: "A string"},
			expected: "string",
		},
		{
			name:     "integer type",
			prop:     Property{Type: "integer", Description: "An integer"},
			expected: "integer",
		},
		{
			name:     "array with items",
			prop:     Property{Type: "array", Items: &Property{Type: "string"}},
			expected: "array",
		},
		{
			name:     "with enum",
			prop:     Property{Type: "string", Enum: []string{"a", "b", "c"}},
			expected: "string",
		},
		{
			name:     "with default",
			prop:     Property{Type: "integer", Default: 100},
			expected: "integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.prop.Type != tt.expected {
				t.Errorf("Expected type '%s', got '%s'", tt.expected, tt.prop.Type)
			}
		})
	}
}

func TestListDefinitionsReadOnly(t *testing.T) {
	cfg := &config.Config{
		Role: config.RoleReadOnly,
	}

	// Create registry without client (nil) for definition testing
	r := &Registry{
		client: nil,
		config: cfg,
		tools:  make(map[string]ToolHandler),
	}

	definitions := r.List()

	// Read-only should have schema and read tools but not write tools
	hasGetRecord := false
	hasPutRecord := false
	hasCreateIndex := false

	for _, def := range definitions {
		switch def.Name {
		case "get_record":
			hasGetRecord = true
		case "put_record":
			hasPutRecord = true
		case "create_index":
			hasCreateIndex = true
		}
	}

	if !hasGetRecord {
		t.Error("Expected get_record tool for read-only role")
	}

	if hasPutRecord {
		t.Error("put_record should not be available for read-only role")
	}

	if hasCreateIndex {
		t.Error("create_index should not be available for read-only role")
	}
}

func TestListDefinitionsReadWrite(t *testing.T) {
	cfg := &config.Config{
		Role: config.RoleReadWrite,
	}

	r := &Registry{
		client: nil,
		config: cfg,
		tools:  make(map[string]ToolHandler),
	}

	definitions := r.List()

	hasPutRecord := false
	hasDeleteRecord := false
	hasBatchWrite := false
	hasOperate := false
	hasCreateIndex := false

	for _, def := range definitions {
		switch def.Name {
		case "put_record":
			hasPutRecord = true
		case "delete_record":
			hasDeleteRecord = true
		case "batch_write":
			hasBatchWrite = true
		case "operate":
			hasOperate = true
		case "create_index":
			hasCreateIndex = true
		}
	}

	if !hasPutRecord {
		t.Error("Expected put_record tool for read-write role")
	}

	if !hasDeleteRecord {
		t.Error("Expected delete_record tool for read-write role")
	}

	if !hasBatchWrite {
		t.Error("Expected batch_write tool for read-write role")
	}

	if !hasOperate {
		t.Error("Expected operate tool for read-write role")
	}

	if hasCreateIndex {
		t.Error("create_index should not be available for read-write role")
	}
}

func TestListDefinitionsAdmin(t *testing.T) {
	cfg := &config.Config{
		Role: config.RoleAdmin,
	}

	r := &Registry{
		client: nil,
		config: cfg,
		tools:  make(map[string]ToolHandler),
	}

	definitions := r.List()

	hasCreateIndex := false
	hasDropIndex := false
	hasTruncateSet := false
	hasListUDFs := false
	hasRegisterUDF := false

	for _, def := range definitions {
		switch def.Name {
		case "create_index":
			hasCreateIndex = true
		case "drop_index":
			hasDropIndex = true
		case "truncate_set":
			hasTruncateSet = true
		case "list_udfs":
			hasListUDFs = true
		case "register_udf":
			hasRegisterUDF = true
		}
	}

	if !hasCreateIndex {
		t.Error("Expected create_index tool for admin role")
	}

	if !hasDropIndex {
		t.Error("Expected drop_index tool for admin role")
	}

	if !hasTruncateSet {
		t.Error("Expected truncate_set tool for admin role")
	}

	if !hasListUDFs {
		t.Error("Expected list_udfs tool for admin role")
	}

	if !hasRegisterUDF {
		t.Error("Expected register_udf tool for admin role")
	}
}

func TestToolCountByRole(t *testing.T) {
	tests := []struct {
		role     config.Role
		minTools int
	}{
		{config.RoleReadOnly, 10},  // At least 10 read-only tools
		{config.RoleReadWrite, 14}, // At least 14 tools with write
		{config.RoleAdmin, 20},     // At least 20 tools with admin
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			cfg := &config.Config{Role: tt.role}
			r := &Registry{
				client: nil,
				config: cfg,
				tools:  make(map[string]ToolHandler),
			}

			definitions := r.List()
			if len(definitions) < tt.minTools {
				t.Errorf("Expected at least %d tools for %s role, got %d",
					tt.minTools, tt.role, len(definitions))
			}
		})
	}
}

func TestSchemaToolsAlwaysPresent(t *testing.T) {
	schemaTools := []string{
		"list_namespaces",
		"describe_namespace",
		"list_sets",
		"describe_set",
	}

	for _, role := range []config.Role{config.RoleReadOnly, config.RoleReadWrite, config.RoleAdmin} {
		t.Run(string(role), func(t *testing.T) {
			cfg := &config.Config{Role: role}
			r := &Registry{
				client: nil,
				config: cfg,
				tools:  make(map[string]ToolHandler),
			}

			definitions := r.List()
			toolNames := make(map[string]bool)
			for _, def := range definitions {
				toolNames[def.Name] = true
			}

			for _, tool := range schemaTools {
				if !toolNames[tool] {
					t.Errorf("Expected schema tool '%s' for role %s", tool, role)
				}
			}
		})
	}
}
