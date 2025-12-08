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

// SSEServer handles Server-Sent Events transport for MCP.
type SSEServer struct {
	server  *Server
	port    int
	clients map[string]*SSEClient
	mu      sync.RWMutex
}

// SSEClient represents a connected SSE client.
type SSEClient struct {
	id       string
	messages chan []byte
	done     chan struct{}
}

// NewSSEServer creates a new SSE server.
func NewSSEServer(server *Server, port int) *SSEServer {
	return &SSEServer{
		server:  server,
		port:    port,
		clients: make(map[string]*SSEClient),
	}
}

// Run starts the SSE HTTP server.
func (s *SSEServer) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	// SSE endpoint for receiving events
	mux.HandleFunc("/sse", s.handleSSE)

	// Message endpoint for sending requests
	mux.HandleFunc("/message", s.handleMessage)

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", s.port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		log.Printf("SSE server listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return httpServer.Shutdown(shutdownCtx)
}

// handleSSE handles new SSE connections.
func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create new client
	clientID := uuid.New().String()
	client := &SSEClient{
		id:       clientID,
		messages: make(chan []byte, 100),
		done:     make(chan struct{}),
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	log.Printf("SSE client connected: %s", clientID)

	// Send initial endpoint event with message URL
	messageURL := fmt.Sprintf("/message?sessionId=%s", clientID)
	initialEvent := fmt.Sprintf("event: endpoint\ndata: %s\n\n", messageURL)
	if _, err := w.Write([]byte(initialEvent)); err != nil {
		log.Printf("Error sending initial event: %v", err)
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Send messages to client
	defer func() {
		s.mu.Lock()
		delete(s.clients, clientID)
		s.mu.Unlock()
		close(client.done)
		log.Printf("SSE client disconnected: %s", clientID)
	}()

	for {
		select {
		case msg := <-client.messages:
			// Format as SSE event
			event := fmt.Sprintf("event: message\ndata: %s\n\n", string(msg))
			if _, err := w.Write([]byte(event)); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

		case <-r.Context().Done():
			return
		}
	}
}

// handleMessage handles incoming JSON-RPC messages.
func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session ID
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Missing sessionId", http.StatusBadRequest)
		return
	}

	// Find client
	s.mu.RLock()
	client, ok := s.clients[sessionID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Process message
	response := s.server.handleMessage(r.Context(), body)

	// Send response via SSE
	if response != nil {
		responseBytes, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
			return
		}

		select {
		case client.messages <- responseBytes:
			// Message sent
		default:
			log.Printf("Client message buffer full: %s", sessionID)
		}
	}

	// Respond with accepted
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Accepted"))
}

// handleHealth returns server health status.
func (s *SSEServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	s.mu.RLock()
	clientCount := len(s.clients)
	s.mu.RUnlock()

	response := map[string]interface{}{
		"status":  "healthy",
		"clients": clientCount,
		"server":  ServerName,
		"version": ServerVersion,
	}

	json.NewEncoder(w).Encode(response)
}
