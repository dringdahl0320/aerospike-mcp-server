// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// WebSocketServer handles WebSocket transport for MCP.
// Note: This is a simplified implementation using long-polling simulation.
// For production, consider using gorilla/websocket or nhooyr/websocket.
type WebSocketServer struct {
	server  *Server
	port    int
	clients map[string]*WSClient
	mu      sync.RWMutex
}

// WSClient represents a connected WebSocket client.
type WSClient struct {
	id       string
	messages chan []byte
	done     chan struct{}
	lastPing time.Time
}

// NewWebSocketServer creates a new WebSocket server.
func NewWebSocketServer(server *Server, port int) *WebSocketServer {
	return &WebSocketServer{
		server:  server,
		port:    port,
		clients: make(map[string]*WSClient),
	}
}

// Run starts the WebSocket HTTP server.
func (s *WebSocketServer) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	// WebSocket endpoint (simulated with HTTP for simplicity)
	mux.HandleFunc("/ws", s.handleWebSocket)

	// HTTP fallback for message handling
	mux.HandleFunc("/ws/send", s.handleSend)
	mux.HandleFunc("/ws/receive", s.handleReceive)

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", s.port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("WebSocket server listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Cleanup goroutine
	go s.cleanupStaleClients(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return httpServer.Shutdown(shutdownCtx)
}

// handleWebSocket handles WebSocket connection establishment.
func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Generate client ID
	clientID := uuid.New().String()

	// Create client
	client := &WSClient{
		id:       clientID,
		messages: make(chan []byte, 100),
		done:     make(chan struct{}),
		lastPing: time.Now(),
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	// Return client ID for subsequent requests
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"client_id": clientID,
		"status":    "connected",
		"message":   "Use /ws/send to send messages and /ws/receive to poll for responses",
	})
}

// handleSend handles incoming messages from clients.
func (s *WebSocketServer) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "X-Client-ID header required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	client, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Update last ping
	client.lastPing = time.Now()

	// Read request body
	requestData, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}

	// Process request
	ctx := r.Context()
	response := s.server.handleMessage(ctx, requestData)

	// Send response
	responseData, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	// Queue response for receive endpoint
	select {
	case client.messages <- responseData:
	default:
		// Buffer full, drop oldest message
		select {
		case <-client.messages:
		default:
		}
		client.messages <- responseData
	}

	// Also return response directly
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(responseData)
}

// handleReceive handles long-polling for responses.
func (s *WebSocketServer) handleReceive(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		http.Error(w, "X-Client-ID header required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	client, ok := s.clients[clientID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Update last ping
	client.lastPing = time.Now()

	// Wait for message with timeout
	timeout := time.After(30 * time.Second)

	select {
	case msg := <-client.messages:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(msg)
	case <-timeout:
		// No message, return empty response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "no_messages"})
	case <-r.Context().Done():
		return
	}
}

// handleHealth returns server health status.
func (s *WebSocketServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientCount := len(s.clients)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"transport": "websocket",
		"clients":   clientCount,
		"server":    ServerName,
		"version":   ServerVersion,
	})
}

// cleanupStaleClients removes clients that haven't pinged recently.
func (s *WebSocketServer) cleanupStaleClients(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for id, client := range s.clients {
				if now.Sub(client.lastPing) > 5*time.Minute {
					close(client.done)
					delete(s.clients, id)
					log.Printf("Cleaned up stale client: %s", id)
				}
			}
			s.mu.Unlock()
		}
	}
}

// Disconnect disconnects a client.
func (s *WebSocketServer) Disconnect(clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if client, ok := s.clients[clientID]; ok {
		close(client.done)
		delete(s.clients, clientID)
	}
}
