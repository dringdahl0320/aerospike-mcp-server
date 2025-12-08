# Contributing to Aerospike MCP Server

Thank you for your interest in contributing to the Aerospike MCP Server! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. Please be respectful and constructive in all interactions.

## How to Contribute

### Reporting Bugs

1. **Search existing issues** to avoid duplicates
2. **Use the bug report template** when creating a new issue
3. **Include:**
   - Go version (`go version`)
   - Aerospike server version
   - Operating system
   - Steps to reproduce
   - Expected vs actual behavior
   - Relevant logs or error messages

### Suggesting Features

1. **Search existing issues** for similar requests
2. **Use the feature request template**
3. **Describe:**
   - The use case
   - Proposed solution
   - Alternatives considered
   - Impact on existing functionality

### Submitting Pull Requests

1. **Fork the repository** and create a feature branch
2. **Follow the coding standards** (see below)
3. **Write tests** for new functionality
4. **Update documentation** as needed
5. **Submit a PR** with a clear description

## Development Setup

### Prerequisites

- Go 1.21 or later
- Make
- golangci-lint (for linting)
- Access to an Aerospike cluster (local or remote)

### Getting Started

```bash
# Clone the repository
git clone https://github.com/dringdahl0320/aerospike-mcp-server.git
cd aerospike-mcp-server

# Install dependencies
go mod download

# Build
make build

# Run tests
make test

# Run linter
make lint
```

### Running Locally

```bash
# Create a config file
cp examples/config.dev.json config.json
# Edit config.json with your Aerospike connection details

# Run the server
make run
```

## Coding Standards

### Go Style

- Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines
- Use `gofmt` for formatting (run `make fmt`)
- Follow naming conventions:
  - Exported functions/types: `CamelCase`
  - Unexported functions/types: `camelCase`
  - Constants: `CamelCase` or `SCREAMING_SNAKE_CASE`

### Code Organization

```
aerospike-mcp-server/
├── cmd/                    # Application entry points
│   └── aerospike-mcp-server/
├── internal/               # Private packages
│   ├── aerospike/         # Aerospike client wrapper
│   ├── audit/             # Audit logging
│   ├── mcp/               # MCP protocol implementation
│   ├── resources/         # MCP resources
│   └── tools/             # MCP tools
├── pkg/                    # Public packages
│   └── config/            # Configuration
├── docs/                   # Documentation
└── examples/               # Example configurations
```

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(tools): add batch_write tool for bulk operations
fix(aerospike): handle connection timeout gracefully
docs(api): update rate limiting documentation
```

### Testing

- Write unit tests for new functionality
- Maintain or improve test coverage
- Use table-driven tests where appropriate
- Mock external dependencies

```go
func TestValidateNamespace(t *testing.T) {
    tests := []struct {
        name      string
        namespace string
        wantErr   bool
    }{
        {"valid", "test", false},
        {"empty", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateNamespace(tt.namespace)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Documentation

- Add godoc comments for exported functions and types
- Update README.md for user-facing changes
- Update docs/API.md for API changes
- Include examples in documentation

## Pull Request Process

1. **Create a feature branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following the coding standards

3. **Run tests and linting:**
   ```bash
   make test
   make lint
   ```

4. **Commit your changes:**
   ```bash
   git commit -m "feat(scope): description"
   ```

5. **Push to your fork:**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request** on GitHub

### PR Requirements

- [ ] Tests pass (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated
- [ ] Commit messages follow conventions
- [ ] PR description explains the changes

## Release Process

Releases are automated via GitHub Actions when a tag is pushed:

```bash
git tag v0.2.0
git push origin v0.2.0
```

This triggers:
1. Running all tests
2. Building binaries for all platforms
3. Creating a GitHub release with artifacts

## Getting Help

- **GitHub Issues**: For bugs and feature requests
- **Discussions**: For questions and general discussion
- **Email**: [maintainer@example.com]

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
