# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2024-12-08

### Added

#### Phase 1: Core Infrastructure
- MCP protocol implementation with JSON-RPC 2.0
- Aerospike Go client v7 integration
- Schema inspection tools (list_namespaces, describe_namespace, list_sets, describe_set)
- Read operations (get_record, batch_get, query_records, scan_set)
- Role-based access control (read-only, read-write, admin)
- TLS support for secure connections
- Configuration via JSON file and environment variables

#### Phase 2: Advanced Operations
- Batch write operations (batch_write)
- Atomic read-modify-write operations (operate)
- SSE (Server-Sent Events) transport
- Cluster info and index listing tools

#### Phase 3: Index and UDF Management
- Secondary index management (create_index, drop_index)
- Set truncation with confirmation (truncate_set)
- UDF management (list_udfs, register_udf, remove_udf, execute_udf)
- Node statistics (node_stats)

#### Phase 4: Security and Observability
- Audit logging with configurable output
- Rate limiting with token bucket algorithm
- Input validation and sanitization
- Dangerous operation detection for UDF code

#### Phase 5: Open-Source Release
- GitHub Actions CI/CD workflow
- Unit tests (172 test cases)
- Docker support with multi-stage build
- Contributing guidelines (CONTRIBUTING.md)
- Security policy (SECURITY.md)
- API documentation (docs/API.md)
- Example configurations for various use cases
- WebSocket transport support

### Transport Protocols
- **stdio**: Standard input/output (default)
- **sse**: Server-Sent Events over HTTP
- **websocket**: HTTP-based WebSocket simulation

### Tools (22 total)
| Category | Tools |
|----------|-------|
| Schema | list_namespaces, describe_namespace, list_sets, describe_set |
| Read | get_record, batch_get, query_records, scan_set |
| Write | put_record, delete_record, batch_write, operate |
| Index | create_index, drop_index, list_indexes |
| UDF | list_udfs, register_udf, remove_udf, execute_udf |
| Cluster | cluster_info, node_stats |
| Admin | truncate_set |

### Security
- Role-based access control
- TLS encryption support
- Audit logging
- Rate limiting
- Input validation
- Confirmation flags for destructive operations
