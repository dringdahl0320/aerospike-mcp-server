// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	cfg := Config{
		Enabled:    true,
		BufferSize: 10,
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("Logger is nil")
	}

	if !logger.enabled {
		t.Error("Logger should be enabled")
	}
}

func TestLogEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		writer:  &buf,
		enabled: true,
		buffer:  make([]Event, 0, 10),
		bufSize: 10,
	}

	event := Event{
		Level:     LevelAudit,
		Category:  CategoryWrite,
		Operation: "put_record",
		Namespace: "test",
		Success:   true,
	}

	logger.Log(event)

	if buf.Len() == 0 {
		t.Error("No output written to buffer")
	}

	// Parse the JSON output
	var logged Event
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &logged); err != nil {
		t.Fatalf("Failed to parse logged event: %v", err)
	}

	if logged.Operation != "put_record" {
		t.Errorf("Expected operation 'put_record', got '%s'", logged.Operation)
	}

	if logged.Namespace != "test" {
		t.Errorf("Expected namespace 'test', got '%s'", logged.Namespace)
	}
}

func TestLogRead(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		writer:  &buf,
		enabled: true,
		buffer:  make([]Event, 0, 10),
		bufSize: 10,
	}

	ctx := context.Background()
	logger.LogRead(ctx, "get_record", "test_ns", "test_set", "key123", 1, time.Millisecond, nil)

	var logged Event
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &logged); err != nil {
		t.Fatalf("Failed to parse logged event: %v", err)
	}

	if logged.Category != CategoryRead {
		t.Errorf("Expected category READ, got %s", logged.Category)
	}

	if !logged.Success {
		t.Error("Expected success to be true")
	}
}

func TestLogWrite(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		writer:  &buf,
		enabled: true,
		buffer:  make([]Event, 0, 10),
		bufSize: 10,
	}

	ctx := context.Background()
	ctx = WithUser(ctx, "test_user")
	ctx = WithClientID(ctx, "client_123")

	logger.LogWrite(ctx, "put_record", "test_ns", "test_set", "key123", 1, time.Millisecond, nil)

	var logged Event
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &logged); err != nil {
		t.Fatalf("Failed to parse logged event: %v", err)
	}

	if logged.User != "test_user" {
		t.Errorf("Expected user 'test_user', got '%s'", logged.User)
	}

	if logged.ClientID != "client_123" {
		t.Errorf("Expected client_id 'client_123', got '%s'", logged.ClientID)
	}
}

func TestGetRecentEvents(t *testing.T) {
	logger := &Logger{
		writer:  &bytes.Buffer{},
		enabled: true,
		buffer:  make([]Event, 0, 10),
		bufSize: 10,
	}

	// Log 5 events
	for i := 0; i < 5; i++ {
		logger.Log(Event{Operation: "op" + string(rune('0'+i))})
	}

	events := logger.GetRecentEvents(3)
	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}
}

func TestDisabledLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		writer:  &buf,
		enabled: false,
		buffer:  make([]Event, 0, 10),
		bufSize: 10,
	}

	logger.Log(Event{Operation: "test"})

	if buf.Len() != 0 {
		t.Error("Disabled logger should not write output")
	}
}
