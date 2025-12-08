// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

// Package mcp implements the Model Context Protocol server for Aerospike.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/onchain-media/aerospike-mcp-server/internal/aerospike"
	"github.com/onchain-media/aerospike-mcp-server/internal/audit"
	"github.com/onchain-media/aerospike-mcp-server/internal/resources"
	"github.com/onchain-media/aerospike-mcp-server/internal/tools"
	"github.com/onchain-media/aerospike-mcp-server/pkg/config"
)

const (
	MCPVersion    = "2024-11-05"
	ServerName    = "aerospike-mcp-server"
	ServerVersion = "0.1.0"
)

// Server implements the MCP protocol server.
type Server struct {
	client      *aerospike.Client
	config      *config.Config
	tools       *tools.Registry
	resources   *resources.Registry
	auditLogger *audit.Logger
	rateLimiter *audit.RateLimiter
	validator   *audit.Validator
	mu          sync.RWMutex
}

// NewServer creates a new MCP server instance.
func NewServer(client *aerospike.Client, cfg *config.Config) *Server {
	// Initialize audit logger
	auditCfg := audit.Config{
		Enabled:    cfg.Audit.Enabled,
		FilePath:   cfg.Audit.FilePath,
		BufferSize: cfg.Audit.BufferSize,
	}
	auditLogger, err := audit.NewLogger(auditCfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize audit logger: %v", err)
	}

	// Initialize rate limiter
	rateLimitCfg := audit.RateLimitConfig{
		Enabled:        cfg.Audit.RateLimitEnabled,
		RequestsPerSec: cfg.Audit.RateLimitRPS,
		BurstSize:      cfg.Audit.RateLimitBurst,
	}
	rateLimiter := audit.NewRateLimiter(rateLimitCfg)

	// Initialize validator
	validator := audit.NewValidator(audit.DefaultValidatorConfig())

	s := &Server{
		client:      client,
		config:      cfg,
		auditLogger: auditLogger,
		rateLimiter: rateLimiter,
		validator:   validator,
	}

	// Initialize tool registry
	s.tools = tools.NewRegistry(client, cfg)

	// Initialize resource registry
	s.resources = resources.NewRegistry(client, cfg)

	return s
}

// Run starts the MCP server with the configured transport.
func (s *Server) Run(ctx context.Context) error {
	// Log server start
	if s.auditLogger != nil {
		s.auditLogger.Log(audit.Event{
			Level:     audit.LevelInfo,
			Category:  audit.CategorySystem,
			Operation: "server_start",
			Success:   true,
			Details: map[string]interface{}{
				"transport": s.config.Transport,
				"role":      s.config.Role,
			},
		})
	}

	// Run transport
	var err error
	switch s.config.Transport {
	case "stdio":
		err = s.runStdio(ctx)
	case "sse":
		err = s.runSSE(ctx)
	case "websocket":
		err = s.runWebSocket(ctx)
	default:
		err = fmt.Errorf("unsupported transport: %s", s.config.Transport)
	}

	// Log server shutdown
	if s.auditLogger != nil {
		s.auditLogger.Log(audit.Event{
			Level:     audit.LevelInfo,
			Category:  audit.CategorySystem,
			Operation: "server_shutdown",
			Success:   err == nil || err == context.Canceled,
			Error:     errorString(err),
		})
		s.auditLogger.Close()
	}

	return err
}

// runStdio runs the server using stdio transport.
func (s *Server) runStdio(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	log.Println("MCP server started (stdio transport)")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read JSON-RPC message
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("reading input: %w", err)
			}

			// Process message
			response := s.handleMessage(ctx, line)
			if response != nil {
				responseBytes, err := json.Marshal(response)
				if err != nil {
					log.Printf("Error marshaling response: %v", err)
					continue
				}
				responseBytes = append(responseBytes, '\n')
				if _, err := writer.Write(responseBytes); err != nil {
					return fmt.Errorf("writing response: %w", err)
				}
			}
		}
	}
}

// runSSE runs the server using Server-Sent Events transport.
func (s *Server) runSSE(ctx context.Context) error {
	port := s.config.Port
	if port == 0 {
		port = 8080
	}
	sseServer := NewSSEServer(s, port)
	return sseServer.Run(ctx)
}

// runWebSocket runs the server using WebSocket transport.
func (s *Server) runWebSocket(ctx context.Context) error {
	port := s.config.Port
	if port == 0 {
		port = 8080
	}

	wsServer := NewWebSocketServer(s, port)
	return wsServer.Run(ctx)
}

// ============================================================================
// JSON-RPC Types
// ============================================================================

// Request represents a JSON-RPC request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents a JSON-RPC error.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// ============================================================================
// Message Handling
// ============================================================================

// handleMessage processes a JSON-RPC message and returns a response.
func (s *Server) handleMessage(ctx context.Context, message []byte) *Response {
	var req Request
	if err := json.Unmarshal(message, &req); err != nil {
		return &Response{
			JSONRPC: "2.0",
			Error: &Error{
				Code:    ParseError,
				Message: "Parse error",
				Data:    err.Error(),
			},
		}
	}

	if req.JSONRPC != "2.0" {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    InvalidRequest,
				Message: "Invalid Request",
				Data:    "jsonrpc must be '2.0'",
			},
		}
	}

	// Route to appropriate handler
	result, err := s.routeMethod(ctx, req.Method, req.Params)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   err,
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// routeMethod routes a method call to the appropriate handler.
func (s *Server) routeMethod(ctx context.Context, method string, params json.RawMessage) (interface{}, *Error) {
	switch method {
	// Lifecycle methods
	case "initialize":
		return s.handleInitialize(ctx, params)
	case "initialized":
		return nil, nil // Notification, no response needed
	case "shutdown":
		return nil, nil

	// Tool methods
	case "tools/list":
		return s.handleToolsList(ctx)
	case "tools/call":
		return s.handleToolsCall(ctx, params)

	// Resource methods
	case "resources/list":
		return s.handleResourcesList(ctx)
	case "resources/read":
		return s.handleResourcesRead(ctx, params)

	// Prompts methods (optional)
	case "prompts/list":
		return s.handlePromptsList(ctx)

	default:
		return nil, &Error{
			Code:    MethodNotFound,
			Message: "Method not found",
			Data:    method,
		}
	}
}

// ============================================================================
// MCP Protocol Handlers
// ============================================================================

// InitializeParams represents the initialize request parameters.
type InitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Roots *struct {
			ListChanged bool `json:"listChanged"`
		} `json:"roots,omitempty"`
		Sampling *struct{} `json:"sampling,omitempty"`
	} `json:"capabilities"`
	ClientInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

// InitializeResult represents the initialize response.
type InitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Tools     *ToolsCapability     `json:"tools,omitempty"`
		Resources *ResourcesCapability `json:"resources,omitempty"`
		Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	} `json:"capabilities"`
	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

func (s *Server) handleInitialize(_ context.Context, params json.RawMessage) (*InitializeResult, *Error) {
	var initParams InitializeParams
	if params != nil {
		if err := json.Unmarshal(params, &initParams); err != nil {
			return nil, &Error{
				Code:    InvalidParams,
				Message: "Invalid params",
				Data:    err.Error(),
			}
		}
	}

	log.Printf("Client connected: %s %s", initParams.ClientInfo.Name, initParams.ClientInfo.Version)

	result := &InitializeResult{
		ProtocolVersion: MCPVersion,
	}
	result.Capabilities.Tools = &ToolsCapability{}
	result.Capabilities.Resources = &ResourcesCapability{}
	result.Capabilities.Prompts = &PromptsCapability{}
	result.ServerInfo.Name = ServerName
	result.ServerInfo.Version = ServerVersion

	return result, nil
}

// ============================================================================
// Tools Handlers
// ============================================================================

// ToolsListResult represents the tools/list response.
type ToolsListResult struct {
	Tools []tools.ToolDefinition `json:"tools"`
}

func (s *Server) handleToolsList(_ context.Context) (*ToolsListResult, *Error) {
	return &ToolsListResult{
		Tools: s.tools.List(),
	}, nil
}

// ToolsCallParams represents the tools/call request parameters.
type ToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolsCallResult represents the tools/call response.
type ToolsCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in tool results.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (*ToolsCallResult, *Error) {
	startTime := time.Now()

	var callParams ToolsCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, &Error{
			Code:    InvalidParams,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	// Check rate limit for write operations
	if isWriteOperation(callParams.Name) {
		if !s.rateLimiter.Allow() {
			if s.auditLogger != nil {
				s.auditLogger.Log(audit.Event{
					Level:     audit.LevelWarning,
					Category:  audit.CategoryWrite,
					Operation: callParams.Name,
					Success:   false,
					Error:     "rate limit exceeded",
				})
			}
			return &ToolsCallResult{
				Content: []ContentBlock{
					{Type: "text", Text: "Error: rate limit exceeded, please try again later"},
				},
				IsError: true,
			}, nil
		}
	}

	result, err := s.tools.Call(ctx, callParams.Name, callParams.Arguments)
	duration := time.Since(startTime)

	// Audit log the operation
	if s.auditLogger != nil {
		category := audit.CategoryRead
		if isWriteOperation(callParams.Name) {
			category = audit.CategoryWrite
		}
		if isAdminOperation(callParams.Name) {
			category = audit.CategoryAdmin
		}

		s.auditLogger.Log(audit.Event{
			Level:     audit.LevelAudit,
			Category:  category,
			Operation: callParams.Name,
			Duration:  duration,
			Success:   err == nil,
			Error:     errorString(err),
		})
	}

	if err != nil {
		return &ToolsCallResult{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Convert result to JSON string
	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return &ToolsCallResult{
		Content: []ContentBlock{
			{Type: "text", Text: string(resultJSON)},
		},
	}, nil
}

// isWriteOperation returns true if the operation modifies data.
func isWriteOperation(op string) bool {
	writeOps := map[string]bool{
		"put_record":    true,
		"delete_record": true,
		"batch_write":   true,
		"operate":       true,
	}
	return writeOps[op]
}

// isAdminOperation returns true if the operation is administrative.
func isAdminOperation(op string) bool {
	adminOps := map[string]bool{
		"create_index": true,
		"drop_index":   true,
		"truncate_set": true,
		"register_udf": true,
		"remove_udf":   true,
	}
	return adminOps[op]
}

// errorString returns error string or empty if nil.
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ============================================================================
// Resources Handlers
// ============================================================================

// ResourcesListResult represents the resources/list response.
type ResourcesListResult struct {
	Resources []resources.ResourceDefinition `json:"resources"`
}

func (s *Server) handleResourcesList(_ context.Context) (*ResourcesListResult, *Error) {
	return &ResourcesListResult{
		Resources: s.resources.List(),
	}, nil
}

// ResourcesReadParams represents the resources/read request parameters.
type ResourcesReadParams struct {
	URI string `json:"uri"`
}

// ResourcesReadResult represents the resources/read response.
type ResourcesReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent represents a resource content block.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

func (s *Server) handleResourcesRead(ctx context.Context, params json.RawMessage) (*ResourcesReadResult, *Error) {
	var readParams ResourcesReadParams
	if err := json.Unmarshal(params, &readParams); err != nil {
		return nil, &Error{
			Code:    InvalidParams,
			Message: "Invalid params",
			Data:    err.Error(),
		}
	}

	content, mimeType, err := s.resources.Read(ctx, readParams.URI)
	if err != nil {
		return nil, &Error{
			Code:    InternalError,
			Message: "Resource read failed",
			Data:    err.Error(),
		}
	}

	return &ResourcesReadResult{
		Contents: []ResourceContent{
			{
				URI:      readParams.URI,
				MimeType: mimeType,
				Text:     content,
			},
		},
	}, nil
}

// ============================================================================
// Prompts Handlers
// ============================================================================

// PromptsListResult represents the prompts/list response.
type PromptsListResult struct {
	Prompts []interface{} `json:"prompts"`
}

func (s *Server) handlePromptsList(_ context.Context) (*PromptsListResult, *Error) {
	// No prompts defined for now
	return &PromptsListResult{
		Prompts: []interface{}{},
	}, nil
}
