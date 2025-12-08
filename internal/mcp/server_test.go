// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRequestParsing(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		method  string
	}{
		{
			name:    "valid initialize",
			input:   `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			wantErr: false,
			method:  "initialize",
		},
		{
			name:    "valid tools/list",
			input:   `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
			wantErr: false,
			method:  "tools/list",
		},
		{
			name:    "invalid json",
			input:   `{"jsonrpc":"2.0","id":1,"method":`,
			wantErr: true,
			method:  "",
		},
		{
			name:    "missing jsonrpc",
			input:   `{"id":1,"method":"test"}`,
			wantErr: false, // Parse succeeds, validation fails later
			method:  "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req Request
			err := json.Unmarshal([]byte(tt.input), &req)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && req.Method != tt.method {
				t.Errorf("Method = %s, want %s", req.Method, tt.method)
			}
		})
	}
}

func TestResponseStructure(t *testing.T) {
	// Test successful response
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  map[string]string{"status": "ok"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got '%v'", parsed["jsonrpc"])
	}
}

func TestErrorResponse(t *testing.T) {
	// Test error response
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Error: &Error{
			Code:    InvalidParams,
			Message: "Invalid params",
			Data:    "test error",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	errorObj := parsed["error"].(map[string]interface{})
	if errorObj["code"].(float64) != float64(InvalidParams) {
		t.Errorf("Expected error code %d, got %v", InvalidParams, errorObj["code"])
	}
}

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"ParseError", ParseError, -32700},
		{"InvalidRequest", InvalidRequest, -32600},
		{"MethodNotFound", MethodNotFound, -32601},
		{"InvalidParams", InvalidParams, -32602},
		{"InternalError", InternalError, -32603},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

func TestInitializeResult(t *testing.T) {
	result := InitializeResult{}
	result.ProtocolVersion = MCPVersion
	result.ServerInfo.Name = ServerName
	result.ServerInfo.Version = ServerVersion
	result.Capabilities.Tools = &ToolsCapability{ListChanged: false}
	result.Capabilities.Resources = &ResourcesCapability{Subscribe: false, ListChanged: false}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal InitializeResult: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal InitializeResult: %v", err)
	}

	if parsed["protocolVersion"] != MCPVersion {
		t.Errorf("Expected protocol version '%s', got '%v'", MCPVersion, parsed["protocolVersion"])
	}

	serverInfo := parsed["serverInfo"].(map[string]interface{})
	if serverInfo["name"] != ServerName {
		t.Errorf("Expected server name '%s', got '%v'", ServerName, serverInfo["name"])
	}
}

func TestToolsCallParams(t *testing.T) {
	params := ToolsCallParams{
		Name:      "get_record",
		Arguments: json.RawMessage(`{"namespace":"test","key":"key1"}`),
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal ToolsCallParams: %v", err)
	}

	var parsed ToolsCallParams
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal ToolsCallParams: %v", err)
	}

	if parsed.Name != "get_record" {
		t.Errorf("Expected name 'get_record', got '%s'", parsed.Name)
	}
}

func TestToolsCallResult(t *testing.T) {
	result := ToolsCallResult{
		Content: []ContentBlock{
			{Type: "text", Text: "test result"},
		},
		IsError: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal ToolsCallResult: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal ToolsCallResult: %v", err)
	}

	content := parsed["content"].([]interface{})
	if len(content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(content))
	}
}

func TestIsWriteOperation(t *testing.T) {
	tests := []struct {
		op      string
		isWrite bool
	}{
		{"put_record", true},
		{"delete_record", true},
		{"batch_write", true},
		{"operate", true},
		{"get_record", false},
		{"list_namespaces", false},
		{"cluster_info", false},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			if isWriteOperation(tt.op) != tt.isWrite {
				t.Errorf("isWriteOperation(%s) = %v, want %v", tt.op, !tt.isWrite, tt.isWrite)
			}
		})
	}
}

func TestIsAdminOperation(t *testing.T) {
	tests := []struct {
		op      string
		isAdmin bool
	}{
		{"create_index", true},
		{"drop_index", true},
		{"truncate_set", true},
		{"register_udf", true},
		{"remove_udf", true},
		{"put_record", false},
		{"get_record", false},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			if isAdminOperation(tt.op) != tt.isAdmin {
				t.Errorf("isAdminOperation(%s) = %v, want %v", tt.op, !tt.isAdmin, tt.isAdmin)
			}
		})
	}
}

func TestErrorString(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil error", nil, ""},
		{"with error", context.Canceled, "context canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errorString(tt.err)
			if result != tt.expected {
				t.Errorf("errorString() = '%s', want '%s'", result, tt.expected)
			}
		})
	}
}

func TestContentBlock(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Text: "Hello, World!",
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal ContentBlock: %v", err)
	}

	var parsed ContentBlock
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal ContentBlock: %v", err)
	}

	if parsed.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", parsed.Type)
	}

	if parsed.Text != "Hello, World!" {
		t.Errorf("Expected text 'Hello, World!', got '%s'", parsed.Text)
	}
}

func TestMCPConstants(t *testing.T) {
	if MCPVersion != "2024-11-05" {
		t.Errorf("Expected MCPVersion '2024-11-05', got '%s'", MCPVersion)
	}

	if ServerName != "aerospike-mcp-server" {
		t.Errorf("Expected ServerName 'aerospike-mcp-server', got '%s'", ServerName)
	}

	if ServerVersion != "0.1.0" {
		t.Errorf("Expected ServerVersion '0.1.0', got '%s'", ServerVersion)
	}
}
