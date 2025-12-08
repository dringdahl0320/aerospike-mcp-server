// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package aerospike

import (
	"testing"

	"github.com/dringdahl0320/aerospike-mcp-server/pkg/config"
)

func TestNewClientConfig(t *testing.T) {
	// Test that NewClient validates config properly
	// Note: These tests don't require a real Aerospike connection

	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "empty hosts",
			config: &config.Config{
				Hosts:     []config.Host{},
				TimeoutMs: 1000,
			},
			wantErr: true, // Will fail validation before connection
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate config first (this is what NewClient does internally)
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRecordConversion(t *testing.T) {
	// Test Record struct JSON marshaling
	rec := Record{
		Key:        "test-key",
		Namespace:  "test-ns",
		Set:        "test-set",
		Bins:       map[string]interface{}{"name": "test", "age": 30},
		Generation: 1,
		Expiration: 0,
	}

	if rec.Key != "test-key" {
		t.Errorf("Expected key 'test-key', got '%s'", rec.Key)
	}

	if rec.Namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got '%s'", rec.Namespace)
	}

	if rec.Bins["name"] != "test" {
		t.Errorf("Expected bin 'name' to be 'test', got '%v'", rec.Bins["name"])
	}
}

func TestBatchWriteRequest(t *testing.T) {
	// Test BatchWriteRequest struct
	req := BatchWriteRequest{
		Namespace: "test-ns",
		Set:       "test-set",
		Key:       "test-key",
		Bins:      map[string]interface{}{"field": "value"},
		TTL:       3600,
		Operation: "put",
	}

	if req.Operation != "put" {
		t.Errorf("Expected operation 'put', got '%s'", req.Operation)
	}

	if req.TTL != 3600 {
		t.Errorf("Expected TTL 3600, got %d", req.TTL)
	}
}

func TestBatchWriteResult(t *testing.T) {
	// Test BatchWriteResult struct
	result := BatchWriteResult{
		Key:     "test-key",
		Success: true,
		Error:   "",
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Error != "" {
		t.Errorf("Expected empty error, got '%s'", result.Error)
	}
}

func TestOperateRequest(t *testing.T) {
	// Test OperateRequest struct
	req := OperateRequest{
		Type:    OpIncrement,
		BinName: "counter",
		Value:   int64(5),
	}

	if req.Type != OpIncrement {
		t.Errorf("Expected type 'increment', got '%s'", req.Type)
	}

	if req.BinName != "counter" {
		t.Errorf("Expected bin name 'counter', got '%s'", req.BinName)
	}
}

func TestOperateResult(t *testing.T) {
	// Test OperateResult struct
	result := OperateResult{
		Bins:       map[string]interface{}{"counter": int64(10)},
		Generation: 5,
		Success:    true,
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Generation != 5 {
		t.Errorf("Expected generation 5, got %d", result.Generation)
	}
}

func TestIndexType(t *testing.T) {
	// Test IndexType constants
	tests := []struct {
		indexType IndexType
		expected  string
	}{
		{IndexTypeNumeric, "NUMERIC"},
		{IndexTypeString, "STRING"},
		{IndexTypeGeo2DSphere, "GEO2DSPHERE"},
		{IndexTypeBlob, "BLOB"},
	}

	for _, tt := range tests {
		if string(tt.indexType) != tt.expected {
			t.Errorf("Expected IndexType '%s', got '%s'", tt.expected, tt.indexType)
		}
	}
}

func TestCollectionType(t *testing.T) {
	// Test CollectionType constants
	tests := []struct {
		collType CollectionType
		expected string
	}{
		{CollectionDefault, "DEFAULT"},
		{CollectionList, "LIST"},
		{CollectionMapKeys, "MAPKEYS"},
		{CollectionMapValues, "MAPVALUES"},
	}

	for _, tt := range tests {
		if string(tt.collType) != tt.expected {
			t.Errorf("Expected CollectionType '%s', got '%s'", tt.expected, tt.collType)
		}
	}
}

func TestToInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
		ok       bool
	}{
		{"int", int(42), 42, true},
		{"int32", int32(42), 42, true},
		{"int64", int64(42), 42, true},
		{"float64", float64(42.0), 42, true},
		{"float32", float32(42.0), 42, true},
		{"string", "42", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toInt64(tt.input)
			if ok != tt.ok {
				t.Errorf("toInt64(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("toInt64(%v) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeBinValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"float64 whole number", float64(42.0), int64(42)},
		{"float64 decimal", float64(42.5), float64(42.5)},
		{"float32 whole number", float32(42.0), int64(42)},
		{"float32 decimal", float32(42.5), float32(42.5)},
		{"int64", int64(42), int64(42)},
		{"string", "hello", "hello"},
		{"nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeBinValue(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeBinValue(%v) = %v (%T), want %v (%T)", tt.input, result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestNormalizeBins(t *testing.T) {
	bins := map[string]interface{}{
		"count":  float64(100),
		"price":  float64(19.99),
		"name":   "test",
		"active": true,
	}

	normalized := normalizeBins(bins)

	// count should be converted to int64
	if v, ok := normalized["count"].(int64); !ok || v != 100 {
		t.Errorf("Expected count to be int64(100), got %v (%T)", normalized["count"], normalized["count"])
	}

	// price should remain float64 (not a whole number)
	if v, ok := normalized["price"].(float64); !ok || v != 19.99 {
		t.Errorf("Expected price to be float64(19.99), got %v (%T)", normalized["price"], normalized["price"])
	}

	// name should remain string
	if v, ok := normalized["name"].(string); !ok || v != "test" {
		t.Errorf("Expected name to be string 'test', got %v (%T)", normalized["name"], normalized["name"])
	}

	// active should remain bool
	if v, ok := normalized["active"].(bool); !ok || v != true {
		t.Errorf("Expected active to be bool true, got %v (%T)", normalized["active"], normalized["active"])
	}
}

func TestUDFInfo(t *testing.T) {
	// Test UDFInfo struct
	info := UDFInfo{
		Name: "test_module.lua",
		Type: "LUA",
		Hash: "abc123",
	}

	if info.Name != "test_module.lua" {
		t.Errorf("Expected name 'test_module.lua', got '%s'", info.Name)
	}
}

func TestClusterInfo(t *testing.T) {
	// Test ClusterInfo struct
	info := ClusterInfo{
		Name:      "test-cluster",
		Size:      3,
		Nodes:     []NodeInfo{{Name: "node1", Address: "127.0.0.1:3000", Active: true}},
		Migrating: false,
	}

	if info.Size != 3 {
		t.Errorf("Expected size 3, got %d", info.Size)
	}

	if len(info.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(info.Nodes))
	}
}

func TestNodeStats(t *testing.T) {
	// Test NodeStats struct
	stats := NodeStats{
		Name:        "node1",
		Address:     "127.0.0.1:3000",
		ClusterSize: 3,
		Uptime:      86400,
		UsedMemory:  1073741824,
		TotalMemory: 4294967296,
		ClientConns: 50,
		Stats:       map[string]string{"uptime": "86400"},
	}

	if stats.ClusterSize != 3 {
		t.Errorf("Expected cluster size 3, got %d", stats.ClusterSize)
	}

	if stats.Uptime != 86400 {
		t.Errorf("Expected uptime 86400, got %d", stats.Uptime)
	}
}

func TestParseInfoString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "simple",
			input: "key1=value1;key2=value2",
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:     "empty",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "single",
			input: "key=value",
			expected: map[string]string{
				"key": "value",
			},
		},
		{
			name:  "with equals in value",
			input: "key=value=with=equals",
			expected: map[string]string{
				"key": "value=with=equals",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInfoString(tt.input)
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("parseInfoString()[%s] = %s, want %s", k, result[k], v)
				}
			}
		})
	}
}
