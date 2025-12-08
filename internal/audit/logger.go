// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

// Package audit provides audit logging for all database operations.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Level represents the severity level of an audit event.
type Level string

const (
	LevelInfo    Level = "INFO"
	LevelWarning Level = "WARNING"
	LevelError   Level = "ERROR"
	LevelAudit   Level = "AUDIT"
)

// Category represents the category of an audit event.
type Category string

const (
	CategoryRead   Category = "READ"
	CategoryWrite  Category = "WRITE"
	CategoryAdmin  Category = "ADMIN"
	CategoryAuth   Category = "AUTH"
	CategorySystem Category = "SYSTEM"
)

// Event represents an audit log event.
type Event struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       Level                  `json:"level"`
	Category    Category               `json:"category"`
	Operation   string                 `json:"operation"`
	Namespace   string                 `json:"namespace,omitempty"`
	Set         string                 `json:"set,omitempty"`
	Key         string                 `json:"key,omitempty"`
	User        string                 `json:"user,omitempty"`
	ClientID    string                 `json:"client_id,omitempty"`
	Duration    time.Duration          `json:"duration_ns"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	RecordCount int                    `json:"record_count,omitempty"`
}

// Logger provides audit logging functionality.
type Logger struct {
	mu       sync.Mutex
	writer   io.Writer
	enabled  bool
	minLevel Level
	buffer   []Event
	bufSize  int
}

// Config holds audit logger configuration.
type Config struct {
	Enabled    bool   `json:"enabled"`
	FilePath   string `json:"file_path,omitempty"`
	BufferSize int    `json:"buffer_size"`
	MinLevel   Level  `json:"min_level"`
}

// DefaultConfig returns default audit configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:    true,
		BufferSize: 100,
		MinLevel:   LevelInfo,
	}
}

// NewLogger creates a new audit logger.
func NewLogger(cfg Config) (*Logger, error) {
	var writer io.Writer = os.Stderr

	if cfg.FilePath != "" {
		file, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("opening audit log file: %w", err)
		}
		writer = file
	}

	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = 100
	}

	return &Logger{
		writer:   writer,
		enabled:  cfg.Enabled,
		minLevel: cfg.MinLevel,
		buffer:   make([]Event, 0, bufSize),
		bufSize:  bufSize,
	}, nil
}

// Log records an audit event.
func (l *Logger) Log(event Event) {
	if !l.enabled {
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Write to output
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Audit log marshal error: %v", err)
		return
	}

	l.writer.Write(append(data, '\n'))

	// Buffer for potential batch operations
	l.buffer = append(l.buffer, event)
	if len(l.buffer) >= l.bufSize {
		l.buffer = l.buffer[1:] // Keep buffer size limited
	}
}

// LogRead logs a read operation.
func (l *Logger) LogRead(ctx context.Context, operation, namespace, set, key string, recordCount int, duration time.Duration, err error) {
	event := Event{
		Level:       LevelInfo,
		Category:    CategoryRead,
		Operation:   operation,
		Namespace:   namespace,
		Set:         set,
		Key:         key,
		Duration:    duration,
		Success:     err == nil,
		RecordCount: recordCount,
	}

	if err != nil {
		event.Error = err.Error()
		event.Level = LevelError
	}

	// Extract context values
	if user := ctx.Value(ContextKeyUser); user != nil {
		event.User = user.(string)
	}
	if clientID := ctx.Value(ContextKeyClientID); clientID != nil {
		event.ClientID = clientID.(string)
	}

	l.Log(event)
}

// LogWrite logs a write operation.
func (l *Logger) LogWrite(ctx context.Context, operation, namespace, set, key string, recordCount int, duration time.Duration, err error) {
	event := Event{
		Level:       LevelAudit,
		Category:    CategoryWrite,
		Operation:   operation,
		Namespace:   namespace,
		Set:         set,
		Key:         key,
		Duration:    duration,
		Success:     err == nil,
		RecordCount: recordCount,
	}

	if err != nil {
		event.Error = err.Error()
		event.Level = LevelError
	}

	// Extract context values
	if user := ctx.Value(ContextKeyUser); user != nil {
		event.User = user.(string)
	}
	if clientID := ctx.Value(ContextKeyClientID); clientID != nil {
		event.ClientID = clientID.(string)
	}

	l.Log(event)
}

// LogAdmin logs an administrative operation.
func (l *Logger) LogAdmin(ctx context.Context, operation string, details map[string]interface{}, duration time.Duration, err error) {
	event := Event{
		Level:     LevelAudit,
		Category:  CategoryAdmin,
		Operation: operation,
		Duration:  duration,
		Success:   err == nil,
		Details:   details,
	}

	if err != nil {
		event.Error = err.Error()
		event.Level = LevelError
	}

	// Extract context values
	if user := ctx.Value(ContextKeyUser); user != nil {
		event.User = user.(string)
	}
	if clientID := ctx.Value(ContextKeyClientID); clientID != nil {
		event.ClientID = clientID.(string)
	}

	l.Log(event)
}

// LogAuth logs an authentication event.
func (l *Logger) LogAuth(ctx context.Context, operation string, success bool, details map[string]interface{}) {
	level := LevelAudit
	if !success {
		level = LevelWarning
	}

	event := Event{
		Level:     level,
		Category:  CategoryAuth,
		Operation: operation,
		Success:   success,
		Details:   details,
	}

	l.Log(event)
}

// GetRecentEvents returns the most recent buffered events.
func (l *Logger) GetRecentEvents(count int) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	if count > len(l.buffer) {
		count = len(l.buffer)
	}

	start := len(l.buffer) - count
	events := make([]Event, count)
	copy(events, l.buffer[start:])
	return events
}

// Close closes the audit logger.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if closer, ok := l.writer.(io.Closer); ok && l.writer != os.Stderr && l.writer != os.Stdout {
		return closer.Close()
	}
	return nil
}

// Context keys for audit information
type contextKey string

const (
	ContextKeyUser     contextKey = "audit_user"
	ContextKeyClientID contextKey = "audit_client_id"
)

// WithUser adds user information to context.
func WithUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, ContextKeyUser, user)
}

// WithClientID adds client ID to context.
func WithClientID(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, ContextKeyClientID, clientID)
}
