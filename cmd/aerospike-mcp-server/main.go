// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dringdahl0320/aerospike-mcp-server/internal/aerospike"
	"github.com/dringdahl0320/aerospike-mcp-server/internal/mcp"
	"github.com/dringdahl0320/aerospike-mcp-server/pkg/config"
)

var (
	version   = "0.1.0"
	buildTime = "unknown"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("aerospike-mcp-server version %s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutdown signal received, closing connections...")
		cancel()
	}()

	// Initialize Aerospike client
	asClient, err := aerospike.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Aerospike: %v", err)
	}
	defer asClient.Close()

	log.Printf("Connected to Aerospike cluster: %s", asClient.ClusterName())

	// Create and run MCP server
	server := mcp.NewServer(asClient, cfg)
	if err := server.Run(ctx); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
