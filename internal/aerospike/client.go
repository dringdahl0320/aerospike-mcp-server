// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

// Package aerospike provides a wrapper around the official Aerospike Go client.
package aerospike

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	as "github.com/aerospike/aerospike-client-go/v7"
	"github.com/onchain-media/aerospike-mcp-server/pkg/config"
)

// Client wraps the Aerospike client with additional MCP-specific functionality.
type Client struct {
	client           *as.Client
	config           *config.Config
	defaultNamespace string
	readPolicy       *as.BasePolicy
	writePolicy      *as.WritePolicy
	scanPolicy       *as.ScanPolicy
	queryPolicy      *as.QueryPolicy
	batchPolicy      *as.BatchPolicy
}

// NewClient creates a new Aerospike client connection.
func NewClient(cfg *config.Config) (*Client, error) {
	// Build host list
	hosts := make([]*as.Host, len(cfg.Hosts))
	for i, h := range cfg.Hosts {
		hosts[i] = as.NewHost(h.Host, h.Port)
	}

	// Configure client policy
	clientPolicy := as.NewClientPolicy()
	clientPolicy.Timeout = time.Duration(cfg.TimeoutMs) * time.Millisecond

	// Set authentication if provided
	if cfg.User != "" {
		clientPolicy.User = cfg.User
		clientPolicy.Password = cfg.Password
	}

	// Configure TLS if enabled
	if cfg.TLS.Enabled {
		tlsConfig, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("configuring TLS: %w", err)
		}
		clientPolicy.TlsConfig = tlsConfig
	}

	// Connect to cluster
	client, err := as.NewClientWithPolicyAndHost(clientPolicy, hosts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to Aerospike cluster: %w", err)
	}

	// Build policies
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond

	readPolicy := as.NewPolicy()
	readPolicy.TotalTimeout = timeout
	readPolicy.MaxRetries = cfg.MaxRetries

	writePolicy := as.NewWritePolicy(0, 0)
	writePolicy.TotalTimeout = timeout
	writePolicy.MaxRetries = cfg.MaxRetries

	scanPolicy := as.NewScanPolicy()
	scanPolicy.TotalTimeout = timeout
	scanPolicy.MaxRetries = cfg.MaxRetries

	queryPolicy := as.NewQueryPolicy()
	queryPolicy.TotalTimeout = timeout
	queryPolicy.MaxRetries = cfg.MaxRetries

	batchPolicy := as.NewBatchPolicy()
	batchPolicy.TotalTimeout = timeout
	batchPolicy.MaxRetries = cfg.MaxRetries

	return &Client{
		client:           client,
		config:           cfg,
		defaultNamespace: cfg.Namespace,
		readPolicy:       readPolicy,
		writePolicy:      writePolicy,
		scanPolicy:       scanPolicy,
		queryPolicy:      queryPolicy,
		batchPolicy:      batchPolicy,
	}, nil
}

// buildTLSConfig creates a TLS configuration from the provided settings.
func buildTLSConfig(cfg config.TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load CA certificate
	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate if provided (mTLS)
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// Close closes the Aerospike client connection.
func (c *Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

// ClusterName returns the name of the connected cluster.
func (c *Client) ClusterName() string {
	nodes := c.client.GetNodes()
	if len(nodes) == 0 {
		return "unknown"
	}
	return nodes[0].GetName()
}

// IsConnected returns true if the client is connected to the cluster.
func (c *Client) IsConnected() bool {
	return c.client.IsConnected()
}

// Config returns the client configuration.
func (c *Client) Config() *config.Config {
	return c.config
}

// ============================================================================
// Schema and Namespace Operations
// ============================================================================

// NamespaceInfo contains namespace metadata.
type NamespaceInfo struct {
	Name              string `json:"name"`
	ReplicationFactor int    `json:"replication_factor"`
	MemoryUsedBytes   int64  `json:"memory_used_bytes"`
	MemorySize        int64  `json:"memory_size"`
	StorageEngine     string `json:"storage_engine"`
	ObjectCount       int64  `json:"object_count"`
}

// ListNamespaces returns all namespaces in the cluster.
func (c *Client) ListNamespaces(ctx context.Context) ([]NamespaceInfo, error) {
	node := c.client.GetNodes()[0]
	infoMap, err := node.RequestInfo(as.NewInfoPolicy(), "namespaces")
	if err != nil {
		return nil, fmt.Errorf("requesting namespaces: %w", err)
	}

	nsNames := strings.Split(infoMap["namespaces"], ";")
	namespaces := make([]NamespaceInfo, 0, len(nsNames))

	for _, name := range nsNames {
		if name == "" {
			continue
		}
		info, err := c.DescribeNamespace(ctx, name)
		if err != nil {
			// Log warning but continue
			continue
		}
		namespaces = append(namespaces, *info)
	}

	return namespaces, nil
}

// DescribeNamespace returns detailed information about a namespace.
func (c *Client) DescribeNamespace(ctx context.Context, namespace string) (*NamespaceInfo, error) {
	node := c.client.GetNodes()[0]
	infoMap, err := node.RequestInfo(as.NewInfoPolicy(), "namespace/"+namespace)
	if err != nil {
		return nil, fmt.Errorf("requesting namespace info: %w", err)
	}

	info := &NamespaceInfo{Name: namespace}

	// Parse the info string
	infoStr := infoMap["namespace/"+namespace]
	pairs := strings.Split(infoStr, ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, value := kv[0], kv[1]

		switch key {
		case "replication-factor":
			info.ReplicationFactor, _ = strconv.Atoi(value)
		case "memory_used_bytes":
			info.MemoryUsedBytes, _ = strconv.ParseInt(value, 10, 64)
		case "memory-size":
			info.MemorySize, _ = strconv.ParseInt(value, 10, 64)
		case "storage-engine":
			info.StorageEngine = value
		case "objects":
			info.ObjectCount, _ = strconv.ParseInt(value, 10, 64)
		}
	}

	return info, nil
}

// SetInfo contains set metadata.
type SetInfo struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	ObjectCount int64  `json:"object_count"`
	MemoryBytes int64  `json:"memory_bytes"`
	StopWrites  bool   `json:"stop_writes"`
}

// ListSets returns all sets in a namespace.
func (c *Client) ListSets(ctx context.Context, namespace string) ([]SetInfo, error) {
	node := c.client.GetNodes()[0]
	infoMap, err := node.RequestInfo(as.NewInfoPolicy(), "sets/"+namespace)
	if err != nil {
		return nil, fmt.Errorf("requesting sets: %w", err)
	}

	setsStr := infoMap["sets/"+namespace]
	if setsStr == "" {
		return []SetInfo{}, nil
	}

	setLines := strings.Split(setsStr, ";")
	sets := make([]SetInfo, 0, len(setLines))

	for _, line := range setLines {
		if line == "" {
			continue
		}
		set := SetInfo{Namespace: namespace}
		pairs := strings.Split(line, ":")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key, value := kv[0], kv[1]

			switch key {
			case "set":
				set.Name = value
			case "objects":
				set.ObjectCount, _ = strconv.ParseInt(value, 10, 64)
			case "memory_data_bytes":
				set.MemoryBytes, _ = strconv.ParseInt(value, 10, 64)
			case "stop-writes-count":
				set.StopWrites = value != "0"
			}
		}
		if set.Name != "" {
			sets = append(sets, set)
		}
	}

	return sets, nil
}

// DescribeSet returns detailed information about a set.
func (c *Client) DescribeSet(ctx context.Context, namespace, setName string) (*SetInfo, error) {
	sets, err := c.ListSets(ctx, namespace)
	if err != nil {
		return nil, err
	}

	for _, set := range sets {
		if set.Name == setName {
			return &set, nil
		}
	}

	return nil, fmt.Errorf("set not found: %s.%s", namespace, setName)
}

// ============================================================================
// Query and Read Operations
// ============================================================================

// Record represents an Aerospike record.
type Record struct {
	Key        string                 `json:"key"`
	Namespace  string                 `json:"namespace"`
	Set        string                 `json:"set,omitempty"`
	Bins       map[string]interface{} `json:"bins"`
	Generation uint32                 `json:"generation"`
	Expiration uint32                 `json:"expiration"`
}

// GetRecord retrieves a single record by key.
func (c *Client) GetRecord(ctx context.Context, namespace, setName, keyValue string, binNames []string) (*Record, error) {
	key, err := as.NewKey(namespace, setName, keyValue)
	if err != nil {
		return nil, fmt.Errorf("creating key: %w", err)
	}

	var rec *as.Record
	if len(binNames) > 0 {
		rec, err = c.client.Get(c.readPolicy, key, binNames...)
	} else {
		rec, err = c.client.Get(c.readPolicy, key)
	}

	if err != nil {
		return nil, fmt.Errorf("getting record: %w", err)
	}

	if rec == nil {
		return nil, nil // Record not found
	}

	return &Record{
		Key:        keyValue,
		Namespace:  namespace,
		Set:        setName,
		Bins:       rec.Bins,
		Generation: rec.Generation,
		Expiration: rec.Expiration,
	}, nil
}

// BatchGetRequest represents a batch get request item.
type BatchGetRequest struct {
	Namespace string   `json:"namespace"`
	Set       string   `json:"set,omitempty"`
	Key       string   `json:"key"`
	BinNames  []string `json:"bin_names,omitempty"`
}

// BatchGet retrieves multiple records in a single request.
func (c *Client) BatchGet(ctx context.Context, requests []BatchGetRequest) ([]*Record, error) {
	if len(requests) > c.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(requests), c.config.MaxBatchSize)
	}

	keys := make([]*as.Key, len(requests))
	for i, req := range requests {
		key, err := as.NewKey(req.Namespace, req.Set, req.Key)
		if err != nil {
			return nil, fmt.Errorf("creating key %d: %w", i, err)
		}
		keys[i] = key
	}

	records, err := c.client.BatchGet(c.batchPolicy, keys)
	if err != nil {
		return nil, fmt.Errorf("batch get: %w", err)
	}

	results := make([]*Record, len(records))
	for i, rec := range records {
		if rec == nil {
			results[i] = nil
			continue
		}
		results[i] = &Record{
			Key:        requests[i].Key,
			Namespace:  requests[i].Namespace,
			Set:        requests[i].Set,
			Bins:       rec.Bins,
			Generation: rec.Generation,
			Expiration: rec.Expiration,
		}
	}

	return results, nil
}

// QueryFilter represents a query filter.
type QueryFilter struct {
	BinName    string      `json:"bin_name"`
	FilterType string      `json:"filter_type"` // "equal", "range", "contains"
	Value      interface{} `json:"value"`
	Begin      int64       `json:"begin,omitempty"`
	End        int64       `json:"end,omitempty"`
}

// QueryRecords executes a secondary index query.
func (c *Client) QueryRecords(ctx context.Context, namespace, setName, indexName string, filter QueryFilter, maxRecords int) ([]*Record, error) {
	if maxRecords <= 0 {
		maxRecords = c.config.DefaultMaxRecords
	}

	stmt := as.NewStatement(namespace, setName)

	// Apply filter
	var asFilter *as.Filter
	switch filter.FilterType {
	case "equal":
		switch v := filter.Value.(type) {
		case int, int64:
			asFilter = as.NewEqualFilter(filter.BinName, v)
		case string:
			asFilter = as.NewEqualFilter(filter.BinName, v)
		}
	case "range":
		asFilter = as.NewRangeFilter(filter.BinName, filter.Begin, filter.End)
	}

	if asFilter != nil {
		stmt.SetFilter(asFilter)
	}

	recordset, err := c.client.Query(c.queryPolicy, stmt)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer recordset.Close()

	records := make([]*Record, 0)
	for rec := range recordset.Results() {
		if rec.Err != nil {
			return nil, fmt.Errorf("query result error: %w", rec.Err)
		}
		records = append(records, &Record{
			Key:        fmt.Sprintf("%v", rec.Record.Key.Value()),
			Namespace:  namespace,
			Set:        setName,
			Bins:       rec.Record.Bins,
			Generation: rec.Record.Generation,
			Expiration: rec.Record.Expiration,
		})
		if len(records) >= maxRecords {
			break
		}
	}

	return records, nil
}

// ScanSet performs a full set scan.
func (c *Client) ScanSet(ctx context.Context, namespace, setName string, binNames []string, maxRecords int, samplePercent int) ([]*Record, error) {
	if maxRecords <= 0 {
		maxRecords = c.config.DefaultMaxRecords
	}

	policy := as.NewScanPolicy()
	policy.TotalTimeout = c.scanPolicy.TotalTimeout
	policy.MaxRetries = c.scanPolicy.MaxRetries

	recordset, err := c.client.ScanAll(policy, namespace, setName, binNames...)
	if err != nil {
		return nil, fmt.Errorf("executing scan: %w", err)
	}
	defer recordset.Close()

	records := make([]*Record, 0)
	for rec := range recordset.Results() {
		if rec.Err != nil {
			return nil, fmt.Errorf("scan result error: %w", rec.Err)
		}
		records = append(records, &Record{
			Key:        fmt.Sprintf("%v", rec.Record.Key.Value()),
			Namespace:  namespace,
			Set:        setName,
			Bins:       rec.Record.Bins,
			Generation: rec.Record.Generation,
			Expiration: rec.Record.Expiration,
		})
		if len(records) >= maxRecords {
			break
		}
	}

	return records, nil
}

// ============================================================================
// Write Operations
// ============================================================================

// PutRecord inserts or updates a record.
func (c *Client) PutRecord(ctx context.Context, namespace, setName, keyValue string, bins map[string]interface{}, ttl int) error {
	if !c.config.CanWrite() {
		return fmt.Errorf("write operations not permitted for role: %s", c.config.Role)
	}

	key, err := as.NewKey(namespace, setName, keyValue)
	if err != nil {
		return fmt.Errorf("creating key: %w", err)
	}

	policy := as.NewWritePolicy(0, uint32(ttl))
	policy.TotalTimeout = c.writePolicy.TotalTimeout
	policy.MaxRetries = c.writePolicy.MaxRetries

	// Normalize bins to convert float64 whole numbers to int64 for proper Aerospike type handling
	normalizedBins := normalizeBins(bins)
	binMap := as.BinMap(normalizedBins)
	if err := c.client.Put(policy, key, binMap); err != nil {
		return fmt.Errorf("putting record: %w", err)
	}

	return nil
}

// DeleteRecord removes a record.
func (c *Client) DeleteRecord(ctx context.Context, namespace, setName, keyValue string) (bool, error) {
	if !c.config.CanWrite() {
		return false, fmt.Errorf("write operations not permitted for role: %s", c.config.Role)
	}

	key, err := as.NewKey(namespace, setName, keyValue)
	if err != nil {
		return false, fmt.Errorf("creating key: %w", err)
	}

	existed, err := c.client.Delete(c.writePolicy, key)
	if err != nil {
		return false, fmt.Errorf("deleting record: %w", err)
	}

	return existed, nil
}

// BatchWriteRequest represents a single write operation in a batch.
type BatchWriteRequest struct {
	Namespace string                 `json:"namespace"`
	Set       string                 `json:"set,omitempty"`
	Key       string                 `json:"key"`
	Bins      map[string]interface{} `json:"bins"`
	TTL       int                    `json:"ttl,omitempty"`
	Operation string                 `json:"operation"` // "put", "delete"
}

// BatchWriteResult represents the result of a batch write operation.
type BatchWriteResult struct {
	Key     string `json:"key"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// BatchWrite executes multiple write operations.
func (c *Client) BatchWrite(ctx context.Context, requests []BatchWriteRequest) ([]BatchWriteResult, error) {
	if !c.config.CanWrite() {
		return nil, fmt.Errorf("write operations not permitted for role: %s", c.config.Role)
	}

	if len(requests) > c.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(requests), c.config.MaxBatchSize)
	}

	results := make([]BatchWriteResult, len(requests))

	for i, req := range requests {
		results[i] = BatchWriteResult{Key: req.Key}

		key, err := as.NewKey(req.Namespace, req.Set, req.Key)
		if err != nil {
			results[i].Success = false
			results[i].Error = fmt.Sprintf("creating key: %v", err)
			continue
		}

		switch req.Operation {
		case "put", "":
			policy := as.NewWritePolicy(0, uint32(req.TTL))
			policy.TotalTimeout = c.writePolicy.TotalTimeout
			// Normalize bins to convert float64 whole numbers to int64
			normalizedBins := normalizeBins(req.Bins)
			binMap := as.BinMap(normalizedBins)
			if err := c.client.Put(policy, key, binMap); err != nil {
				results[i].Success = false
				results[i].Error = fmt.Sprintf("put: %v", err)
			} else {
				results[i].Success = true
			}

		case "delete":
			if _, err := c.client.Delete(c.writePolicy, key); err != nil {
				results[i].Success = false
				results[i].Error = fmt.Sprintf("delete: %v", err)
			} else {
				results[i].Success = true
			}

		default:
			results[i].Success = false
			results[i].Error = fmt.Sprintf("unknown operation: %s", req.Operation)
		}
	}

	return results, nil
}

// OperationType defines the type of atomic operation.
type OperationType string

const (
	OpIncrement OperationType = "increment"
	OpAppend    OperationType = "append"
	OpPrepend   OperationType = "prepend"
	OpTouch     OperationType = "touch"
	OpRead      OperationType = "read"
)

// OperateRequest represents an atomic operation request.
type OperateRequest struct {
	Type    OperationType `json:"type"`
	BinName string        `json:"bin_name"`
	Value   interface{}   `json:"value,omitempty"`
}

// OperateResult represents the result of an operate call.
type OperateResult struct {
	Bins       map[string]interface{} `json:"bins,omitempty"`
	Generation uint32                 `json:"generation"`
	Success    bool                   `json:"success"`
}

// Operate executes atomic read-modify-write operations on a single record.
func (c *Client) Operate(ctx context.Context, namespace, setName, keyValue string, operations []OperateRequest, ttl int) (*OperateResult, error) {
	if !c.config.CanWrite() {
		return nil, fmt.Errorf("write operations not permitted for role: %s", c.config.Role)
	}

	key, err := as.NewKey(namespace, setName, keyValue)
	if err != nil {
		return nil, fmt.Errorf("creating key: %w", err)
	}

	// Build operations
	ops := make([]*as.Operation, 0, len(operations))
	for _, op := range operations {
		switch op.Type {
		case OpIncrement:
			if intVal, ok := toInt64(op.Value); ok {
				ops = append(ops, as.AddOp(as.NewBin(op.BinName, intVal)))
			} else {
				return nil, fmt.Errorf("increment requires integer value for bin %s", op.BinName)
			}

		case OpAppend:
			if strVal, ok := op.Value.(string); ok {
				ops = append(ops, as.AppendOp(as.NewBin(op.BinName, strVal)))
			} else {
				return nil, fmt.Errorf("append requires string value for bin %s", op.BinName)
			}

		case OpPrepend:
			if strVal, ok := op.Value.(string); ok {
				ops = append(ops, as.PrependOp(as.NewBin(op.BinName, strVal)))
			} else {
				return nil, fmt.Errorf("prepend requires string value for bin %s", op.BinName)
			}

		case OpTouch:
			ops = append(ops, as.TouchOp())

		case OpRead:
			if op.BinName != "" {
				ops = append(ops, as.GetBinOp(op.BinName))
			} else {
				ops = append(ops, as.GetOp())
			}

		default:
			return nil, fmt.Errorf("unknown operation type: %s", op.Type)
		}
	}

	policy := as.NewWritePolicy(0, uint32(ttl))
	policy.TotalTimeout = c.writePolicy.TotalTimeout

	rec, err := c.client.Operate(policy, key, ops...)
	if err != nil {
		return nil, fmt.Errorf("operate: %w", err)
	}

	result := &OperateResult{
		Success: true,
	}
	if rec != nil {
		result.Bins = rec.Bins
		result.Generation = rec.Generation
	}

	return result, nil
}

// toInt64 converts various numeric types to int64.
func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int32:
		return int64(val), true
	case int64:
		return val, true
	case float64:
		return int64(val), true
	case float32:
		return int64(val), true
	default:
		return 0, false
	}
}

// normalizeBinValue converts float64 values that represent whole numbers to int64.
// This is necessary because JSON unmarshals all numbers as float64, but Aerospike's
// increment operation only works on integer bins.
func normalizeBinValue(v interface{}) interface{} {
	switch val := v.(type) {
	case float64:
		// Check if it's a whole number
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	case float32:
		// Check if it's a whole number
		if val == float32(int64(val)) {
			return int64(val)
		}
		return val
	default:
		return v
	}
}

// normalizeBins converts all whole-number floats in a bin map to integers.
func normalizeBins(bins map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(bins))
	for k, v := range bins {
		normalized[k] = normalizeBinValue(v)
	}
	return normalized
}

// ============================================================================
// Index Operations
// ============================================================================

// IndexInfo contains secondary index metadata.
type IndexInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Set       string `json:"set,omitempty"`
	Bin       string `json:"bin"`
	Type      string `json:"type"`
	State     string `json:"state"`
}

// ListIndexes returns all secondary indexes in a namespace.
func (c *Client) ListIndexes(ctx context.Context, namespace string) ([]IndexInfo, error) {
	node := c.client.GetNodes()[0]
	infoMap, err := node.RequestInfo(as.NewInfoPolicy(), "sindex/"+namespace)
	if err != nil {
		return nil, fmt.Errorf("requesting indexes: %w", err)
	}

	sindexStr := infoMap["sindex/"+namespace]
	if sindexStr == "" {
		return []IndexInfo{}, nil
	}

	lines := strings.Split(sindexStr, ";")
	indexes := make([]IndexInfo, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		idx := IndexInfo{Namespace: namespace}
		pairs := strings.Split(line, ":")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch kv[0] {
			case "indexname":
				idx.Name = kv[1]
			case "set":
				idx.Set = kv[1]
			case "bin":
				idx.Bin = kv[1]
			case "type":
				idx.Type = kv[1]
			case "state":
				idx.State = kv[1]
			}
		}
		if idx.Name != "" {
			indexes = append(indexes, idx)
		}
	}

	return indexes, nil
}

// IndexType represents the type of secondary index.
type IndexType string

const (
	IndexTypeNumeric     IndexType = "NUMERIC"
	IndexTypeString      IndexType = "STRING"
	IndexTypeGeo2DSphere IndexType = "GEO2DSPHERE"
	IndexTypeBlob        IndexType = "BLOB"
)

// CollectionType represents the collection type for index.
type CollectionType string

const (
	CollectionDefault   CollectionType = "DEFAULT"
	CollectionList      CollectionType = "LIST"
	CollectionMapKeys   CollectionType = "MAPKEYS"
	CollectionMapValues CollectionType = "MAPVALUES"
)

// CreateIndex creates a secondary index on a bin.
func (c *Client) CreateIndex(ctx context.Context, namespace, setName, indexName, binName string, indexType IndexType, collectionType CollectionType) error {
	if !c.config.CanAdmin() {
		return fmt.Errorf("admin operations not permitted for role: %s", c.config.Role)
	}

	var asIndexType as.IndexType
	switch indexType {
	case IndexTypeNumeric:
		asIndexType = as.NUMERIC
	case IndexTypeString:
		asIndexType = as.STRING
	case IndexTypeGeo2DSphere:
		asIndexType = as.GEO2DSPHERE
	case IndexTypeBlob:
		asIndexType = as.BLOB
	default:
		return fmt.Errorf("invalid index type: %s", indexType)
	}

	var asCollectionType as.IndexCollectionType
	switch collectionType {
	case CollectionDefault, "":
		asCollectionType = as.ICT_DEFAULT
	case CollectionList:
		asCollectionType = as.ICT_LIST
	case CollectionMapKeys:
		asCollectionType = as.ICT_MAPKEYS
	case CollectionMapValues:
		asCollectionType = as.ICT_MAPVALUES
	default:
		return fmt.Errorf("invalid collection type: %s", collectionType)
	}

	task, err := c.client.CreateComplexIndex(nil, namespace, setName, indexName, binName, asIndexType, asCollectionType)
	if err != nil {
		return fmt.Errorf("creating index: %w", err)
	}

	// Wait for index creation to complete
	for {
		done, err := task.IsDone()
		if err != nil {
			return fmt.Errorf("waiting for index creation: %w", err)
		}
		if done {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue waiting
		}
	}

	return nil
}

// DropIndex removes a secondary index.
func (c *Client) DropIndex(ctx context.Context, namespace, indexName string) error {
	if !c.config.CanAdmin() {
		return fmt.Errorf("admin operations not permitted for role: %s", c.config.Role)
	}

	if err := c.client.DropIndex(nil, namespace, "", indexName); err != nil {
		return fmt.Errorf("dropping index: %w", err)
	}

	return nil
}

// TruncateSet removes all records from a set.
func (c *Client) TruncateSet(ctx context.Context, namespace, setName string) error {
	if !c.config.CanAdmin() {
		return fmt.Errorf("admin operations not permitted for role: %s", c.config.Role)
	}

	if err := c.client.Truncate(nil, namespace, setName, nil); err != nil {
		return fmt.Errorf("truncating set: %w", err)
	}

	return nil
}

// ============================================================================
// UDF Operations
// ============================================================================

// UDFInfo contains UDF module metadata.
type UDFInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Hash string `json:"hash"`
}

// ListUDFs returns all registered UDF modules.
func (c *Client) ListUDFs(ctx context.Context) ([]UDFInfo, error) {
	udfs, err := c.client.ListUDF(nil)
	if err != nil {
		return nil, fmt.Errorf("listing UDFs: %w", err)
	}

	result := make([]UDFInfo, len(udfs))
	for i, udf := range udfs {
		result[i] = UDFInfo{
			Name: udf.Filename,
			Type: string(udf.Language),
			Hash: string(udf.Hash),
		}
	}

	return result, nil
}

// RegisterUDF registers a Lua UDF module on the cluster.
func (c *Client) RegisterUDF(ctx context.Context, moduleName, code string) error {
	if !c.config.CanAdmin() {
		return fmt.Errorf("admin operations not permitted for role: %s", c.config.Role)
	}

	task, err := c.client.RegisterUDF(nil, []byte(code), moduleName, as.LUA)
	if err != nil {
		return fmt.Errorf("registering UDF: %w", err)
	}

	// Wait for registration to complete
	for {
		done, err := task.IsDone()
		if err != nil {
			return fmt.Errorf("waiting for UDF registration: %w", err)
		}
		if done {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue waiting
		}
	}

	return nil
}

// RemoveUDF removes a UDF module from the cluster.
func (c *Client) RemoveUDF(ctx context.Context, moduleName string) error {
	if !c.config.CanAdmin() {
		return fmt.Errorf("admin operations not permitted for role: %s", c.config.Role)
	}

	task, err := c.client.RemoveUDF(nil, moduleName)
	if err != nil {
		return fmt.Errorf("removing UDF: %w", err)
	}

	// Wait for removal to complete
	for {
		done, err := task.IsDone()
		if err != nil {
			return fmt.Errorf("waiting for UDF removal: %w", err)
		}
		if done {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue waiting
		}
	}

	return nil
}

// ExecuteUDF executes a UDF on a single record.
func (c *Client) ExecuteUDF(ctx context.Context, namespace, setName, keyValue, moduleName, functionName string, args []interface{}) (interface{}, error) {
	key, err := as.NewKey(namespace, setName, keyValue)
	if err != nil {
		return nil, fmt.Errorf("creating key: %w", err)
	}

	result, err := c.client.Execute(nil, key, moduleName, functionName, as.NewValue(args))
	if err != nil {
		return nil, fmt.Errorf("executing UDF: %w", err)
	}

	return result, nil
}

// ============================================================================
// Cluster Operations
// ============================================================================

// ClusterInfo contains cluster topology and health information.
type ClusterInfo struct {
	Name      string     `json:"name"`
	Size      int        `json:"size"`
	Nodes     []NodeInfo `json:"nodes"`
	Migrating bool       `json:"migrating"`
}

// NodeInfo contains information about a cluster node.
type NodeInfo struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Active  bool   `json:"active"`
}

// GetClusterInfo returns cluster topology and status.
func (c *Client) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	nodes := c.client.GetNodes()
	nodeInfos := make([]NodeInfo, len(nodes))

	clusterName := ""
	for i, node := range nodes {
		if i == 0 {
			clusterName = node.GetName()
		}
		nodeInfos[i] = NodeInfo{
			Name:    node.GetName(),
			Address: node.GetHost().String(),
			Active:  node.IsActive(),
		}
	}

	return &ClusterInfo{
		Name:      clusterName,
		Size:      len(nodes),
		Nodes:     nodeInfos,
		Migrating: false, // Would need to check migration stats
	}, nil
}

// NodeStats contains performance metrics for a node.
type NodeStats struct {
	Name        string            `json:"name"`
	Address     string            `json:"address"`
	ClusterSize int               `json:"cluster_size"`
	Uptime      int64             `json:"uptime_seconds"`
	UsedMemory  int64             `json:"used_memory_bytes"`
	TotalMemory int64             `json:"total_memory_bytes"`
	UsedDisk    int64             `json:"used_disk_bytes"`
	TotalDisk   int64             `json:"total_disk_bytes"`
	ClientConns int               `json:"client_connections"`
	Stats       map[string]string `json:"stats"`
}

// GetNodeStats returns performance metrics for a specific node or all nodes.
func (c *Client) GetNodeStats(ctx context.Context, nodeName string) ([]NodeStats, error) {
	nodes := c.client.GetNodes()
	results := make([]NodeStats, 0)

	for _, node := range nodes {
		if nodeName != "" && node.GetName() != nodeName {
			continue
		}

		infoMap, err := node.RequestInfo(as.NewInfoPolicy(), "statistics")
		if err != nil {
			continue
		}

		stats := parseInfoString(infoMap["statistics"])

		nodeStats := NodeStats{
			Name:    node.GetName(),
			Address: node.GetHost().String(),
			Stats:   stats,
		}

		// Parse specific stats
		if v, ok := stats["cluster_size"]; ok {
			nodeStats.ClusterSize, _ = strconv.Atoi(v)
		}
		if v, ok := stats["uptime"]; ok {
			nodeStats.Uptime, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := stats["system_total_mem_size"]; ok {
			nodeStats.TotalMemory, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := stats["system_free_mem_pct"]; ok {
			pct, _ := strconv.ParseInt(v, 10, 64)
			nodeStats.UsedMemory = nodeStats.TotalMemory * (100 - pct) / 100
		}
		if v, ok := stats["client_connections"]; ok {
			nodeStats.ClientConns, _ = strconv.Atoi(v)
		}

		results = append(results, nodeStats)

		if nodeName != "" {
			break
		}
	}

	if nodeName != "" && len(results) == 0 {
		return nil, fmt.Errorf("node not found: %s", nodeName)
	}

	return results, nil
}

// parseInfoString parses a semicolon-separated key=value info string.
func parseInfoString(info string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(info, ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}
