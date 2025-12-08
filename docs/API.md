# Aerospike MCP Server API Reference

This document provides comprehensive documentation for all tools, resources, and configuration options available in the Aerospike MCP Server.

## Table of Contents

- [Authentication & Authorization](#authentication--authorization)
- [Tools](#tools)
  - [Schema Operations](#schema-operations)
  - [Read Operations](#read-operations)
  - [Write Operations](#write-operations)
  - [Index Management](#index-management)
  - [UDF Management](#udf-management)
  - [Cluster Operations](#cluster-operations)
- [Resources](#resources)
- [Configuration](#configuration)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)
- [Audit Logging](#audit-logging)

---

## Authentication & Authorization

### Roles

| Role | Description | Permissions |
|------|-------------|-------------|
| `read-only` | Read-only access | Schema inspection, queries, scans |
| `read-write` | Read and write access | All read operations + put, delete, batch_write, operate |
| `admin` | Full administrative access | All operations including index/UDF management |

### Configuration

```json
{
  "user": "mcp_service",
  "password_env": "AEROSPIKE_PASSWORD",
  "role": "read-write"
}
```

---

## Tools

### Schema Operations

#### list_namespaces

Enumerate all namespaces configured on the connected Aerospike cluster.

**Parameters:** None

**Returns:**
```json
[
  {
    "name": "user_profiles",
    "replication_factor": 2,
    "memory_used_bytes": 1073741824,
    "memory_size": 4294967296,
    "storage_engine": "memory",
    "object_count": 1000000
  }
]
```

---

#### describe_namespace

Retrieve detailed configuration and statistics for a specific namespace.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace name |

**Returns:** Namespace configuration and statistics

---

#### list_sets

List all sets within a namespace with record counts and memory utilization.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace name |

**Returns:**
```json
[
  {
    "name": "users",
    "namespace": "user_profiles",
    "object_count": 500000,
    "memory_bytes": 536870912,
    "stop_writes": false
  }
]
```

---

#### describe_set

Retrieve detailed statistics for a specific set including bin schema inference.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace name |
| `set_name` | string | Yes | Target set name |

---

### Read Operations

#### get_record

Retrieve a single record by primary key.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | No | Target set |
| `key` | string | Yes | Primary key value |
| `bins` | array | No | Specific bins to retrieve (default: all) |

**Returns:**
```json
{
  "key": "user123",
  "namespace": "user_profiles",
  "set": "users",
  "bins": {
    "name": "John Doe",
    "email": "john@example.com"
  },
  "generation": 5,
  "expiration": 0
}
```

---

#### batch_get

Retrieve multiple records in a single network round-trip.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `keys` | array | Yes | Array of key objects |
| `max_concurrent` | integer | No | Maximum concurrent requests (default: 100) |

**Key Object:**
```json
{
  "key": "user123",
  "set": "users",
  "bins": ["name", "email"]
}
```

---

#### query_records

Execute a secondary index query with optional filter expressions.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | No | Target set |
| `index_name` | string | Yes | Secondary index to query |
| `filter` | object | Yes | Filter expression |
| `max_records` | integer | No | Result limit (default: 1000) |

**Filter Types:**
- `equal`: Exact match filter
- `range`: Numeric range filter

```json
{
  "bin_name": "age",
  "filter_type": "range",
  "begin": 18,
  "end": 65
}
```

---

#### scan_set

Perform a full set scan with sampling and projection support.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | No | Target set |
| `bins` | array | No | Specific bins to retrieve |
| `max_records` | integer | No | Maximum records to return (default: 1000) |
| `sample_percent` | integer | No | Sample percentage (1-100) |

**Safety Note:** Requires explicit confirmation for sets exceeding 100,000 records.

---

### Write Operations

*Requires `read-write` or `admin` role*

#### put_record

Insert or update a single record.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | No | Target set |
| `key` | string | Yes | Primary key |
| `bins` | object | Yes | Bin name-value pairs |
| `ttl` | integer | No | Record TTL in seconds (-1 for namespace default) |

---

#### delete_record

Remove a single record by primary key.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | No | Target set |
| `key` | string | Yes | Primary key |

**Returns:**
```json
{
  "existed": true
}
```

---

#### batch_write

Execute multiple write operations in a batch.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `operations` | array | Yes | Array of write operations |

**Operation Object:**
```json
{
  "namespace": "user_profiles",
  "set": "users",
  "key": "user123",
  "bins": {"name": "John"},
  "ttl": 3600,
  "operation": "put"
}
```

**Limit:** Maximum 5,000 operations per batch.

---

#### operate

Execute atomic read-modify-write operations on a single record.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | No | Target set |
| `key` | string | Yes | Primary key |
| `operations` | array | Yes | Array of operations |
| `ttl` | integer | No | Record TTL |

**Operation Types:**

| Type | Description | Value Required |
|------|-------------|----------------|
| `increment` | Increment numeric bin | Integer |
| `append` | Append to string bin | String |
| `prepend` | Prepend to string bin | String |
| `touch` | Update record TTL | No |
| `read` | Read bin value | No |

---

### Index Management

*Requires `admin` role*

#### list_indexes

Enumerate all secondary indexes in a namespace.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |

---

#### create_index

Create a secondary index on a bin.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | No | Target set |
| `index_name` | string | Yes | Index identifier |
| `bin_name` | string | Yes | Bin to index |
| `index_type` | enum | Yes | NUMERIC, STRING, GEO2DSPHERE, BLOB |
| `collection_type` | enum | No | DEFAULT, LIST, MAPKEYS, MAPVALUES |

---

#### drop_index

Remove a secondary index.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `index_name` | string | Yes | Index identifier |
| `confirm` | boolean | Yes | Must be `true` |

---

#### truncate_set

Remove all records from a set.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | Yes | Target set |
| `confirm` | boolean | Yes | Must be `true` |
| `confirm_destructive` | boolean | Yes | Must be `true` |

**⚠️ EXTREME CAUTION:** This operation is irreversible.

---

### UDF Management

*Requires `admin` role*

#### list_udfs

List all registered User-Defined Functions.

**Parameters:** None

---

#### register_udf

Register a Lua UDF module on the cluster.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `module_name` | string | Yes | UDF module identifier (must end with .lua) |
| `code` | string | Yes | Lua source code |

---

#### remove_udf

Remove a UDF module from the cluster.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `module_name` | string | Yes | UDF module identifier |
| `confirm` | boolean | Yes | Must be `true` |

---

#### execute_udf

Execute a UDF on a single record.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | string | Yes | Target namespace |
| `set_name` | string | No | Target set |
| `key` | string | Yes | Primary key |
| `module_name` | string | Yes | UDF module name |
| `function_name` | string | Yes | Function to execute |
| `args` | array | No | Function arguments |

---

### Cluster Operations

#### cluster_info

Retrieve cluster topology, node health, and migration status.

**Parameters:** None

**Returns:**
```json
{
  "name": "aerospike-cluster",
  "size": 3,
  "nodes": [
    {
      "name": "BB9010016AE4202",
      "address": "192.168.1.100:3000",
      "active": true
    }
  ],
  "migrating": false
}
```

---

#### node_stats

Retrieve performance metrics for a specific node or all nodes.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `node_name` | string | No | Specific node name (returns all if not specified) |

**Returns:**
```json
[
  {
    "name": "BB9010016AE4202",
    "address": "192.168.1.100:3000",
    "cluster_size": 3,
    "uptime_seconds": 86400,
    "used_memory_bytes": 1073741824,
    "total_memory_bytes": 4294967296,
    "client_connections": 50
  }
]
```

---

## Resources

Resources provide read-only access to database metadata.

| URI | Description |
|-----|-------------|
| `aerospike://cluster/info` | Cluster topology and status |
| `aerospike://ns/{name}` | Namespace configuration |
| `aerospike://ns/{name}/sets` | Set listing with statistics |
| `aerospike://ns/{name}/indexes` | Secondary index definitions |
| `aerospike://udfs` | Registered UDF modules |
| `aerospike://schema/{ns}/{set}` | Inferred bin schema |

---

## Configuration

### Full Configuration Example

```json
{
  "hosts": [
    { "host": "aerospike-node-1.internal", "port": 3000 },
    { "host": "aerospike-node-2.internal", "port": 3000 }
  ],
  "namespace": "ad_platform",
  "user": "mcp_service",
  "password_env": "AEROSPIKE_PASSWORD",
  "tls": {
    "enabled": true,
    "ca_file": "/etc/ssl/aerospike-ca.pem"
  },
  "role": "read-write",
  "timeout_ms": 1000,
  "max_retries": 2,
  "default_max_records": 1000,
  "max_batch_size": 5000,
  "transport": "stdio",
  "audit": {
    "enabled": true,
    "file_path": "/var/log/aerospike-mcp.log",
    "buffer_size": 100,
    "rate_limit_enabled": true,
    "rate_limit_rps": 100,
    "rate_limit_burst": 200
  }
}
```

---

## Error Handling

### JSON-RPC Error Codes

| Code | Name | Description |
|------|------|-------------|
| -32700 | Parse Error | Invalid JSON |
| -32600 | Invalid Request | Invalid JSON-RPC |
| -32601 | Method Not Found | Unknown method |
| -32602 | Invalid Params | Invalid parameters |
| -32603 | Internal Error | Server error |

### Tool Errors

Tool errors are returned in the `content` with `isError: true`:

```json
{
  "content": [
    { "type": "text", "text": "Error: namespace not found" }
  ],
  "isError": true
}
```

---

## Rate Limiting

Write operations are rate-limited to prevent overwhelming the cluster.

### Configuration

```json
{
  "audit": {
    "rate_limit_enabled": true,
    "rate_limit_rps": 100,
    "rate_limit_burst": 200
  }
}
```

### Rate-Limited Operations

- `put_record`
- `delete_record`
- `batch_write`
- `operate`

---

## Audit Logging

All operations are logged for compliance and debugging.

### Log Format (JSON)

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "AUDIT",
  "category": "WRITE",
  "operation": "put_record",
  "namespace": "user_profiles",
  "set": "users",
  "key": "user123",
  "duration_ns": 1500000,
  "success": true,
  "record_count": 1
}
```

### Categories

| Category | Description |
|----------|-------------|
| `READ` | Read operations |
| `WRITE` | Write operations |
| `ADMIN` | Administrative operations |
| `AUTH` | Authentication events |
| `SYSTEM` | System events |

### Levels

| Level | Description |
|-------|-------------|
| `INFO` | Informational |
| `WARNING` | Warnings |
| `ERROR` | Errors |
| `AUDIT` | Audit trail events |
