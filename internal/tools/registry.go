// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

// Package tools implements MCP tool definitions and handlers for Aerospike operations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/onchain-media/aerospike-mcp-server/internal/aerospike"
	"github.com/onchain-media/aerospike-mcp-server/pkg/config"
)

// ToolDefinition represents an MCP tool definition.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema represents the JSON schema for tool inputs.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property represents a property in the input schema.
type Property struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Items       *Property   `json:"items,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// Registry manages available MCP tools.
type Registry struct {
	client *aerospike.Client
	config *config.Config
	tools  map[string]ToolHandler
}

// ToolHandler is a function that handles a tool call.
type ToolHandler func(ctx context.Context, args json.RawMessage) (interface{}, error)

// NewRegistry creates a new tool registry.
func NewRegistry(client *aerospike.Client, cfg *config.Config) *Registry {
	r := &Registry{
		client: client,
		config: cfg,
		tools:  make(map[string]ToolHandler),
	}

	// Register schema/namespace tools
	r.registerSchemaTools()

	// Register query/read tools
	r.registerReadTools()

	// Register write tools (if permitted)
	if cfg.CanWrite() {
		r.registerWriteTools()
	}

	// Register index tools (if admin)
	if cfg.CanAdmin() {
		r.registerIndexTools()
	}

	// Register cluster tools
	r.registerClusterTools()

	return r
}

// List returns all available tool definitions.
func (r *Registry) List() []ToolDefinition {
	definitions := []ToolDefinition{
		// Schema/Namespace Tools
		{
			Name:        "list_namespaces",
			Description: "Enumerate all namespaces configured on the connected Aerospike cluster",
			InputSchema: InputSchema{Type: "object"},
		},
		{
			Name:        "describe_namespace",
			Description: "Retrieve detailed configuration and statistics for a specific namespace",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"namespace": {Type: "string", Description: "Target namespace name"},
				},
				Required: []string{"namespace"},
			},
		},
		{
			Name:        "list_sets",
			Description: "List all sets within a namespace with record counts and memory utilization",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"namespace": {Type: "string", Description: "Target namespace name"},
				},
				Required: []string{"namespace"},
			},
		},
		{
			Name:        "describe_set",
			Description: "Retrieve detailed statistics for a specific set including bin schema inference",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"namespace": {Type: "string", Description: "Target namespace name"},
					"set_name":  {Type: "string", Description: "Target set name"},
				},
				Required: []string{"namespace", "set_name"},
			},
		},
		// Query/Read Tools
		{
			Name:        "get_record",
			Description: "Retrieve a single record by primary key",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"namespace": {Type: "string", Description: "Target namespace"},
					"set_name":  {Type: "string", Description: "Target set (optional)"},
					"key":       {Type: "string", Description: "Primary key value"},
					"bins":      {Type: "array", Description: "Specific bins to retrieve (default: all)", Items: &Property{Type: "string"}},
				},
				Required: []string{"namespace", "key"},
			},
		},
		{
			Name:        "batch_get",
			Description: "Retrieve multiple records in a single network round-trip",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"namespace":      {Type: "string", Description: "Target namespace"},
					"keys":           {Type: "array", Description: "Array of key objects", Items: &Property{Type: "object"}},
					"max_concurrent": {Type: "integer", Description: "Maximum concurrent requests (default: 100)", Default: 100},
				},
				Required: []string{"namespace", "keys"},
			},
		},
		{
			Name:        "query_records",
			Description: "Execute a secondary index query with optional filter expressions",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"namespace":   {Type: "string", Description: "Target namespace"},
					"set_name":    {Type: "string", Description: "Target set (optional)"},
					"index_name":  {Type: "string", Description: "Secondary index to query"},
					"filter":      {Type: "object", Description: "Filter expression (equality, range, or geo)"},
					"max_records": {Type: "integer", Description: "Result limit (default: 1000)", Default: 1000},
				},
				Required: []string{"namespace", "index_name", "filter"},
			},
		},
		{
			Name:        "scan_set",
			Description: "Perform a full set scan with sampling and projection support. Requires explicit confirmation for sets exceeding 100,000 records.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"namespace":      {Type: "string", Description: "Target namespace"},
					"set_name":       {Type: "string", Description: "Target set (optional)"},
					"bins":           {Type: "array", Description: "Specific bins to retrieve", Items: &Property{Type: "string"}},
					"max_records":    {Type: "integer", Description: "Maximum records to return (default: 1000)", Default: 1000},
					"sample_percent": {Type: "integer", Description: "Sample percentage (1-100)"},
				},
				Required: []string{"namespace"},
			},
		},
		// Cluster Tools
		{
			Name:        "cluster_info",
			Description: "Retrieve cluster topology, node health, and migration status",
			InputSchema: InputSchema{Type: "object"},
		},
		{
			Name:        "list_indexes",
			Description: "Enumerate all secondary indexes in a namespace",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"namespace": {Type: "string", Description: "Target namespace"},
				},
				Required: []string{"namespace"},
			},
		},
	}

	// Add write tools if permitted
	if r.config.CanWrite() {
		definitions = append(definitions,
			ToolDefinition{
				Name:        "put_record",
				Description: "Insert or update a single record",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"namespace": {Type: "string", Description: "Target namespace"},
						"set_name":  {Type: "string", Description: "Target set (optional)"},
						"key":       {Type: "string", Description: "Primary key"},
						"bins":      {Type: "object", Description: "Bin name-value pairs"},
						"ttl":       {Type: "integer", Description: "Record TTL in seconds (-1 for namespace default)", Default: -1},
					},
					Required: []string{"namespace", "key", "bins"},
				},
			},
			ToolDefinition{
				Name:        "delete_record",
				Description: "Remove a single record by primary key. Deletion operations are logged and require generation match by default.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"namespace": {Type: "string", Description: "Target namespace"},
						"set_name":  {Type: "string", Description: "Target set (optional)"},
						"key":       {Type: "string", Description: "Primary key"},
					},
					Required: []string{"namespace", "key"},
				},
			},
			ToolDefinition{
				Name:        "batch_write",
				Description: "Execute multiple write operations (put/delete) in a batch. Maximum 5,000 operations per batch to prevent timeout issues.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"operations": {
							Type:        "array",
							Description: "Array of write operations",
							Items: &Property{
								Type:        "object",
								Description: "Write operation with namespace, set, key, bins, ttl, and operation type (put/delete)",
							},
						},
					},
					Required: []string{"operations"},
				},
			},
			ToolDefinition{
				Name:        "operate",
				Description: "Execute atomic read-modify-write operations on a single record. Supports increment, append, prepend, touch, and read operations.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"namespace": {Type: "string", Description: "Target namespace"},
						"set_name":  {Type: "string", Description: "Target set (optional)"},
						"key":       {Type: "string", Description: "Primary key"},
						"operations": {
							Type:        "array",
							Description: "Array of operations: {type: 'increment'|'append'|'prepend'|'touch'|'read', bin_name: string, value: any}",
							Items:       &Property{Type: "object"},
						},
						"ttl": {Type: "integer", Description: "Record TTL in seconds", Default: -1},
					},
					Required: []string{"namespace", "key", "operations"},
				},
			},
		)
	}

	// Add admin tools if permitted
	if r.config.CanAdmin() {
		definitions = append(definitions,
			ToolDefinition{
				Name:        "create_index",
				Description: "Create a secondary index on a bin",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"namespace":       {Type: "string", Description: "Target namespace"},
						"set_name":        {Type: "string", Description: "Target set (optional)"},
						"index_name":      {Type: "string", Description: "Index identifier"},
						"bin_name":        {Type: "string", Description: "Bin to index"},
						"index_type":      {Type: "string", Description: "Index type", Enum: []string{"NUMERIC", "STRING", "GEO2DSPHERE", "BLOB"}},
						"collection_type": {Type: "string", Description: "Collection type", Enum: []string{"DEFAULT", "LIST", "MAPKEYS", "MAPVALUES"}},
					},
					Required: []string{"namespace", "index_name", "bin_name", "index_type"},
				},
			},
			ToolDefinition{
				Name:        "drop_index",
				Description: "Remove a secondary index. Requires explicit confirmation in production environments.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"namespace":  {Type: "string", Description: "Target namespace"},
						"index_name": {Type: "string", Description: "Index identifier"},
						"confirm":    {Type: "boolean", Description: "Confirmation flag (required: true)"},
					},
					Required: []string{"namespace", "index_name", "confirm"},
				},
			},
			ToolDefinition{
				Name:        "truncate_set",
				Description: "Remove all records from a set. EXTREME CAUTION - Requires double confirmation.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"namespace":           {Type: "string", Description: "Target namespace"},
						"set_name":            {Type: "string", Description: "Target set"},
						"confirm":             {Type: "boolean", Description: "First confirmation flag"},
						"confirm_destructive": {Type: "boolean", Description: "Second confirmation flag"},
					},
					Required: []string{"namespace", "set_name", "confirm", "confirm_destructive"},
				},
			},
			// UDF Management
			ToolDefinition{
				Name:        "list_udfs",
				Description: "List all registered User-Defined Functions",
				InputSchema: InputSchema{Type: "object"},
			},
			ToolDefinition{
				Name:        "register_udf",
				Description: "Register a Lua UDF module on the cluster",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"module_name": {Type: "string", Description: "UDF module identifier"},
						"code":        {Type: "string", Description: "Lua source code"},
					},
					Required: []string{"module_name", "code"},
				},
			},
			ToolDefinition{
				Name:        "remove_udf",
				Description: "Remove a UDF module from the cluster",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"module_name": {Type: "string", Description: "UDF module identifier"},
						"confirm":     {Type: "boolean", Description: "Confirmation flag"},
					},
					Required: []string{"module_name", "confirm"},
				},
			},
			ToolDefinition{
				Name:        "execute_udf",
				Description: "Execute a UDF on a single record",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"namespace":     {Type: "string", Description: "Target namespace"},
						"set_name":      {Type: "string", Description: "Target set"},
						"key":           {Type: "string", Description: "Primary key"},
						"module_name":   {Type: "string", Description: "UDF module name"},
						"function_name": {Type: "string", Description: "Function to execute"},
						"args":          {Type: "array", Description: "Function arguments", Items: &Property{Type: "object"}},
					},
					Required: []string{"namespace", "key", "module_name", "function_name"},
				},
			},
		)
	}

	// Add node_stats tool (available to all roles)
	definitions = append(definitions, ToolDefinition{
		Name:        "node_stats",
		Description: "Retrieve performance metrics for a specific node or all nodes",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"node_name": {Type: "string", Description: "Specific node name (optional, returns all if not specified)"},
			},
		},
	})

	return definitions
}

// Call executes a tool by name with the given arguments.
func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (interface{}, error) {
	handler, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return handler(ctx, args)
}

// ============================================================================
// Tool Registration
// ============================================================================

func (r *Registry) registerSchemaTools() {
	r.tools["list_namespaces"] = r.handleListNamespaces
	r.tools["describe_namespace"] = r.handleDescribeNamespace
	r.tools["list_sets"] = r.handleListSets
	r.tools["describe_set"] = r.handleDescribeSet
}

func (r *Registry) registerReadTools() {
	r.tools["get_record"] = r.handleGetRecord
	r.tools["batch_get"] = r.handleBatchGet
	r.tools["query_records"] = r.handleQueryRecords
	r.tools["scan_set"] = r.handleScanSet
}

func (r *Registry) registerWriteTools() {
	r.tools["put_record"] = r.handlePutRecord
	r.tools["delete_record"] = r.handleDeleteRecord
	r.tools["batch_write"] = r.handleBatchWrite
	r.tools["operate"] = r.handleOperate
}

func (r *Registry) registerIndexTools() {
	r.tools["create_index"] = r.handleCreateIndex
	r.tools["drop_index"] = r.handleDropIndex
	r.tools["truncate_set"] = r.handleTruncateSet
	// UDF tools
	r.tools["list_udfs"] = r.handleListUDFs
	r.tools["register_udf"] = r.handleRegisterUDF
	r.tools["remove_udf"] = r.handleRemoveUDF
	r.tools["execute_udf"] = r.handleExecuteUDF
}

func (r *Registry) registerClusterTools() {
	r.tools["cluster_info"] = r.handleClusterInfo
	r.tools["list_indexes"] = r.handleListIndexes
	r.tools["node_stats"] = r.handleNodeStats
}

// ============================================================================
// Tool Handlers
// ============================================================================

func (r *Registry) handleListNamespaces(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return r.client.ListNamespaces(ctx)
}

type describeNamespaceArgs struct {
	Namespace string `json:"namespace"`
}

func (r *Registry) handleDescribeNamespace(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a describeNamespaceArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.DescribeNamespace(ctx, a.Namespace)
}

type listSetsArgs struct {
	Namespace string `json:"namespace"`
}

func (r *Registry) handleListSets(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a listSetsArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.ListSets(ctx, a.Namespace)
}

type describeSetArgs struct {
	Namespace string `json:"namespace"`
	SetName   string `json:"set_name"`
}

func (r *Registry) handleDescribeSet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a describeSetArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.DescribeSet(ctx, a.Namespace, a.SetName)
}

type getRecordArgs struct {
	Namespace string   `json:"namespace"`
	SetName   string   `json:"set_name"`
	Key       string   `json:"key"`
	Bins      []string `json:"bins"`
}

func (r *Registry) handleGetRecord(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a getRecordArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.GetRecord(ctx, a.Namespace, a.SetName, a.Key, a.Bins)
}

type batchGetArgs struct {
	Namespace string `json:"namespace"`
	Keys      []struct {
		Key  string   `json:"key"`
		Set  string   `json:"set"`
		Bins []string `json:"bins"`
	} `json:"keys"`
	MaxConcurrent int `json:"max_concurrent"`
}

func (r *Registry) handleBatchGet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a batchGetArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	requests := make([]aerospike.BatchGetRequest, len(a.Keys))
	for i, k := range a.Keys {
		requests[i] = aerospike.BatchGetRequest{
			Namespace: a.Namespace,
			Set:       k.Set,
			Key:       k.Key,
			BinNames:  k.Bins,
		}
	}

	return r.client.BatchGet(ctx, requests)
}

type queryRecordsArgs struct {
	Namespace  string                `json:"namespace"`
	SetName    string                `json:"set_name"`
	IndexName  string                `json:"index_name"`
	Filter     aerospike.QueryFilter `json:"filter"`
	MaxRecords int                   `json:"max_records"`
}

func (r *Registry) handleQueryRecords(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a queryRecordsArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.QueryRecords(ctx, a.Namespace, a.SetName, a.IndexName, a.Filter, a.MaxRecords)
}

type scanSetArgs struct {
	Namespace     string   `json:"namespace"`
	SetName       string   `json:"set_name"`
	Bins          []string `json:"bins"`
	MaxRecords    int      `json:"max_records"`
	SamplePercent int      `json:"sample_percent"`
}

func (r *Registry) handleScanSet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a scanSetArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.ScanSet(ctx, a.Namespace, a.SetName, a.Bins, a.MaxRecords, a.SamplePercent)
}

type putRecordArgs struct {
	Namespace string                 `json:"namespace"`
	SetName   string                 `json:"set_name"`
	Key       string                 `json:"key"`
	Bins      map[string]interface{} `json:"bins"`
	TTL       int                    `json:"ttl"`
}

func (r *Registry) handlePutRecord(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a putRecordArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if err := r.client.PutRecord(ctx, a.Namespace, a.SetName, a.Key, a.Bins, a.TTL); err != nil {
		return nil, err
	}
	return map[string]string{"status": "ok"}, nil
}

type deleteRecordArgs struct {
	Namespace string `json:"namespace"`
	SetName   string `json:"set_name"`
	Key       string `json:"key"`
}

func (r *Registry) handleDeleteRecord(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a deleteRecordArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	existed, err := r.client.DeleteRecord(ctx, a.Namespace, a.SetName, a.Key)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"existed": existed}, nil
}

type batchWriteArgs struct {
	Operations []aerospike.BatchWriteRequest `json:"operations"`
}

func (r *Registry) handleBatchWrite(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a batchWriteArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.BatchWrite(ctx, a.Operations)
}

type operateArgs struct {
	Namespace  string                     `json:"namespace"`
	SetName    string                     `json:"set_name"`
	Key        string                     `json:"key"`
	Operations []aerospike.OperateRequest `json:"operations"`
	TTL        int                        `json:"ttl"`
}

func (r *Registry) handleOperate(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a operateArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.Operate(ctx, a.Namespace, a.SetName, a.Key, a.Operations, a.TTL)
}

func (r *Registry) handleClusterInfo(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return r.client.GetClusterInfo(ctx)
}

type listIndexesArgs struct {
	Namespace string `json:"namespace"`
}

func (r *Registry) handleListIndexes(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a listIndexesArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.ListIndexes(ctx, a.Namespace)
}

type nodeStatsArgs struct {
	NodeName string `json:"node_name"`
}

func (r *Registry) handleNodeStats(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a nodeStatsArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return r.client.GetNodeStats(ctx, a.NodeName)
}

// ============================================================================
// Admin Tool Handlers
// ============================================================================

type createIndexArgs struct {
	Namespace      string `json:"namespace"`
	SetName        string `json:"set_name"`
	IndexName      string `json:"index_name"`
	BinName        string `json:"bin_name"`
	IndexType      string `json:"index_type"`
	CollectionType string `json:"collection_type"`
}

func (r *Registry) handleCreateIndex(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a createIndexArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	err := r.client.CreateIndex(ctx, a.Namespace, a.SetName, a.IndexName, a.BinName,
		aerospike.IndexType(a.IndexType), aerospike.CollectionType(a.CollectionType))
	if err != nil {
		return nil, err
	}

	return map[string]string{"status": "ok", "index": a.IndexName}, nil
}

type dropIndexArgs struct {
	Namespace string `json:"namespace"`
	IndexName string `json:"index_name"`
	Confirm   bool   `json:"confirm"`
}

func (r *Registry) handleDropIndex(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a dropIndexArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if !a.Confirm {
		return nil, fmt.Errorf("drop_index requires confirm=true")
	}

	if err := r.client.DropIndex(ctx, a.Namespace, a.IndexName); err != nil {
		return nil, err
	}

	return map[string]string{"status": "ok", "dropped": a.IndexName}, nil
}

type truncateSetArgs struct {
	Namespace          string `json:"namespace"`
	SetName            string `json:"set_name"`
	Confirm            bool   `json:"confirm"`
	ConfirmDestructive bool   `json:"confirm_destructive"`
}

func (r *Registry) handleTruncateSet(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a truncateSetArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if !a.Confirm || !a.ConfirmDestructive {
		return nil, fmt.Errorf("truncate_set requires both confirm=true and confirm_destructive=true")
	}

	if err := r.client.TruncateSet(ctx, a.Namespace, a.SetName); err != nil {
		return nil, err
	}

	return map[string]string{"status": "ok", "truncated": fmt.Sprintf("%s.%s", a.Namespace, a.SetName)}, nil
}

// ============================================================================
// UDF Tool Handlers
// ============================================================================

func (r *Registry) handleListUDFs(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return r.client.ListUDFs(ctx)
}

type registerUDFArgs struct {
	ModuleName string `json:"module_name"`
	Code       string `json:"code"`
}

func (r *Registry) handleRegisterUDF(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a registerUDFArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if err := r.client.RegisterUDF(ctx, a.ModuleName, a.Code); err != nil {
		return nil, err
	}

	return map[string]string{"status": "ok", "module": a.ModuleName}, nil
}

type removeUDFArgs struct {
	ModuleName string `json:"module_name"`
	Confirm    bool   `json:"confirm"`
}

func (r *Registry) handleRemoveUDF(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a removeUDFArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if !a.Confirm {
		return nil, fmt.Errorf("remove_udf requires confirm=true")
	}

	if err := r.client.RemoveUDF(ctx, a.ModuleName); err != nil {
		return nil, err
	}

	return map[string]string{"status": "ok", "removed": a.ModuleName}, nil
}

type executeUDFArgs struct {
	Namespace    string        `json:"namespace"`
	SetName      string        `json:"set_name"`
	Key          string        `json:"key"`
	ModuleName   string        `json:"module_name"`
	FunctionName string        `json:"function_name"`
	Args         []interface{} `json:"args"`
}

func (r *Registry) handleExecuteUDF(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var a executeUDFArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	result, err := r.client.ExecuteUDF(ctx, a.Namespace, a.SetName, a.Key, a.ModuleName, a.FunctionName, a.Args)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{"result": result}, nil
}
