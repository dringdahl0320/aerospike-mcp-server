# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

1. **Do NOT** open a public GitHub issue for security vulnerabilities
2. Email security concerns to: [security@example.com]
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Resolution Target**: Within 30 days (depending on severity)

### Severity Classification

| Severity | Description | Response Time |
|----------|-------------|---------------|
| Critical | Remote code execution, auth bypass | Immediate |
| High | Data exposure, privilege escalation | 24 hours |
| Medium | Denial of service, information leak | 7 days |
| Low | Minor issues, hardening suggestions | 30 days |

## Security Best Practices

### Configuration

1. **Use TLS**: Always enable TLS for production deployments
   ```json
   {
     "tls": {
       "enabled": true,
       "ca_file": "/path/to/ca.pem"
     }
   }
   ```

2. **Use Environment Variables for Secrets**: Never hardcode passwords
   ```json
   {
     "password_env": "AEROSPIKE_PASSWORD"
   }
   ```

3. **Principle of Least Privilege**: Use the minimum required role
   - `read-only` for read-only workloads
   - `read-write` only when writes are needed
   - `admin` only for administrative tasks

4. **Rate Limiting**: Enable rate limiting in production
   ```json
   {
     "audit": {
       "rate_limit_enabled": true,
       "rate_limit_rps": 100
     }
   }
   ```

5. **Audit Logging**: Enable audit logging for compliance
   ```json
   {
     "audit": {
       "enabled": true,
       "file_path": "/var/log/aerospike-mcp.log"
     }
   }
   ```

### Deployment

1. Run as non-root user
2. Use read-only file systems where possible
3. Limit network exposure (use private networks)
4. Regularly update dependencies
5. Monitor audit logs for suspicious activity

### Network Security

1. Use firewall rules to restrict access
2. Deploy behind a reverse proxy for SSE transport
3. Use network policies in Kubernetes environments

## Known Limitations

- WebSocket transport is not yet implemented
- UDF execution does not sandbox Lua code (rely on Aerospike's sandboxing)
- Batch operations are processed sequentially (not atomic across all records)

## Dependency Security

We regularly update dependencies. Check `go.sum` for current versions.

Run `go mod tidy` and `go get -u ./...` to update dependencies.

## Security Audit

This project has not yet undergone a formal security audit. Use in production at your own risk.
