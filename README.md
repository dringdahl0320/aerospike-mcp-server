# Aerospike MCP Server

A Model Context Protocol (MCP) server for Aerospike database, enabling AI assistants to interact with Aerospike clusters for Ad-Tech and real-time bidding applications.

## Features

- **Schema Inspection**: List namespaces, sets, and infer bin schemas
- **Query Operations**: Get records, batch operations, secondary index queries, and scans
- **Write Operations**: Put, delete, and batch write records (role-based)
- **Index Management**: Create, list, and drop secondary indexes (admin role)
- **Cluster Operations**: Monitor cluster health and node statistics
- **Safety Constraints**: Built-in limits and confirmation requirements for destructive operations

## Installation

### Prerequisites

- Go 1.21 or later
- Access to an Aerospike cluster

### Build from Source

```bash
git clone https://github.com/dringdahl0320/aerospike-mcp-server.git
cd aerospike-mcp-server
make build
```

### Install

```bash
make install
```

## Configuration

Create a configuration file (e.g., `aerospike-mcp.json`):

```json
{
  "hosts": [{ "host": "localhost", "port": 3000 }],
  "namespace": "ad_platform",
  "user": "mcp_service",
  "password_env": "AEROSPIKE_PASSWORD",
  "tls": { "enabled": false },
  "role": "read-write",
  "timeout_ms": 1000,
  "max_retries": 2
}
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `hosts` | Aerospike cluster nodes | `localhost:3000` |
| `namespace` | Default namespace | - |
| `user` | Authentication username | - |
| `password` | Authentication password | - |
| `password_env` | Environment variable for password | - |
| `tls.enabled` | Enable TLS connection | `false` |
| `tls.ca_file` | CA certificate file path | - |
| `role` | Permission role: `read-only`, `read-write`, `admin` | `read-only` |
| `timeout_ms` | Operation timeout in milliseconds | `1000` |
| `max_retries` | Maximum retry attempts | `2` |
| `transport` | Transport protocol: `stdio`, `sse`, `websocket` | `stdio` |

### Roles and Permissions

| Role | Permissions |
|------|-------------|
| `read-only` | List, describe, get, query operations |
| `read-write` | Read operations + put, batch_write, delete |
| `admin` | All operations including index/UDF management and truncate |

## IDE Integration

### Windsurf

Add to `~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "aerospike": {
      "command": "aerospike-mcp-server",
      "args": ["--config", "/path/to/aerospike-mcp.json"],
      "env": { "AEROSPIKE_PASSWORD": "${AEROSPIKE_PASSWORD}" }
    }
  }
}
```

### Claude Desktop

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "aerospike": {
      "command": "aerospike-mcp-server",
      "args": ["--config", "/path/to/aerospike-mcp.json"]
    }
  }
}
```

## Available Tools

### Schema/Namespace Operations

- `list_namespaces` - Enumerate all namespaces
- `describe_namespace` - Get namespace details
- `list_sets` - List sets with statistics
- `describe_set` - Get set details with schema inference

### Query/Read Operations

- `get_record` - Retrieve a single record by key
- `batch_get` - Retrieve multiple records
- `query_records` - Execute secondary index query
- `scan_set` - Perform set scan with sampling

### Write Operations (read-write, admin roles)

- `put_record` - Insert or update a record
- `delete_record` - Remove a record
- `batch_write` - Execute multiple writes (up to 5,000 operations per batch)
- `operate` - Atomic read-modify-write operations (increment, append, prepend, touch, read)

### Index Management (admin role)

- `list_indexes` - List secondary indexes
- `create_index` - Create a secondary index (NUMERIC, STRING, GEO2DSPHERE, BLOB)
- `drop_index` - Remove a secondary index (requires confirmation)
- `truncate_set` - Remove all records from a set (requires double confirmation)

### UDF Management (admin role)

- `list_udfs` - List all registered User-Defined Functions
- `register_udf` - Register a Lua UDF module
- `remove_udf` - Remove a UDF module (requires confirmation)
- `execute_udf` - Execute a UDF on a single record

### Cluster Operations

- `cluster_info` - Get cluster topology and health
- `node_stats` - Get performance metrics for nodes (memory, connections, uptime)

## Security Features

### Audit Logging

All operations are logged for compliance and debugging:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "AUDIT",
  "category": "WRITE",
  "operation": "put_record",
  "duration_ns": 1500000,
  "success": true
}
```

### Rate Limiting

Write operations are rate-limited to protect the cluster:

```json
{
  "audit": {
    "rate_limit_enabled": true,
    "rate_limit_rps": 100,
    "rate_limit_burst": 200
  }
}
```

### Input Validation

- Namespace/set/bin names validated against Aerospike limits
- Key length validation
- UDF code safety checks
- Batch size limits enforced

## Available Resources

| URI | Description |
|-----|-------------|
| `aerospike://cluster/info` | Cluster topology and status |
| `aerospike://ns/{name}` | Namespace configuration |
| `aerospike://ns/{name}/sets` | Set listing with statistics |
| `aerospike://ns/{name}/indexes` | Secondary index definitions |
| `aerospike://udfs` | Registered UDF modules |
| `aerospike://schema/{ns}/{set}` | Inferred bin schema |

## Transport Protocols

### stdio (Default)

Standard input/output transport for local IDE integration. Used by Windsurf, Cursor, and Claude Desktop.

```json
{
  "transport": "stdio"
}
```

### SSE (Server-Sent Events)

HTTP-based transport for remote/shared server deployments. Enables web clients and team environments.

```json
{
  "transport": "sse",
  "port": 8080
}
```

Run the server:

```bash
./bin/aerospike-mcp-server --config examples/config.sse.json
```

Endpoints:

- `GET /sse` - SSE connection endpoint (returns message URL in `endpoint` event)
- `POST /message?sessionId=<id>` - Send JSON-RPC requests
- `GET /health` - Health check

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

## Ad-Tech Use Cases

This MCP server is optimized for Ad-Tech operations:

- **User Profile Store**: < 1ms P99 for audience segments and frequency caps
- **Bid Request Cache**: < 500μs P99 for OpenRTB bid/response caching
- **Campaign Pacing**: Real-time spend tracking and budget controls
- **Attribution Store**: High-throughput click/conversion event streams
- **Fraud Scoring**: Low-latency ML feature vector lookups

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting pull requests.

## Support

- [GitHub Issues](https://github.com/dringdahl0320/aerospike-mcp-server/issues)
- [Aerospike Documentation](https://docs.aerospike.com/)
- [MCP Specification](https://modelcontextprotocol.io/)
