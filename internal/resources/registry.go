// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

// Package resources implements MCP resource definitions and handlers for Aerospike.
package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/onchain-media/aerospike-mcp-server/internal/aerospike"
	"github.com/onchain-media/aerospike-mcp-server/pkg/config"
)

// ResourceDefinition represents an MCP resource definition.
type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Registry manages available MCP resources.
type Registry struct {
	client *aerospike.Client
	config *config.Config
}

// NewRegistry creates a new resource registry.
func NewRegistry(client *aerospike.Client, cfg *config.Config) *Registry {
	return &Registry{
		client: client,
		config: cfg,
	}
}

// List returns all available resource definitions.
func (r *Registry) List() []ResourceDefinition {
	resources := []ResourceDefinition{
		{
			URI:         "aerospike://cluster/info",
			Name:        "Cluster Info",
			Description: "Cluster topology and status",
			MimeType:    "application/json",
		},
	}

	// Add namespace resources dynamically
	namespaces, err := r.client.ListNamespaces(context.Background())
	if err == nil {
		for _, ns := range namespaces {
			resources = append(resources,
				ResourceDefinition{
					URI:         fmt.Sprintf("aerospike://ns/%s", ns.Name),
					Name:        fmt.Sprintf("Namespace: %s", ns.Name),
					Description: "Namespace configuration",
					MimeType:    "application/json",
				},
				ResourceDefinition{
					URI:         fmt.Sprintf("aerospike://ns/%s/sets", ns.Name),
					Name:        fmt.Sprintf("Sets in %s", ns.Name),
					Description: "Set listing with statistics",
					MimeType:    "application/json",
				},
				ResourceDefinition{
					URI:         fmt.Sprintf("aerospike://ns/%s/indexes", ns.Name),
					Name:        fmt.Sprintf("Indexes in %s", ns.Name),
					Description: "Secondary index definitions",
					MimeType:    "application/json",
				},
			)
		}
	}

	// Add UDF resource
	resources = append(resources, ResourceDefinition{
		URI:         "aerospike://udfs",
		Name:        "UDF Modules",
		Description: "Registered UDF modules",
		MimeType:    "application/json",
	})

	return resources
}

// Read retrieves the content of a resource by URI.
func (r *Registry) Read(ctx context.Context, uri string) (string, string, error) {
	// Parse the URI
	if !strings.HasPrefix(uri, "aerospike://") {
		return "", "", fmt.Errorf("invalid URI scheme: %s", uri)
	}

	path := strings.TrimPrefix(uri, "aerospike://")

	// Route to appropriate handler
	switch {
	case path == "cluster/info":
		return r.readClusterInfo(ctx)

	case strings.HasPrefix(path, "ns/"):
		return r.readNamespaceResource(ctx, path)

	case path == "udfs":
		return r.readUDFs(ctx)

	case strings.HasPrefix(path, "schema/"):
		return r.readSchema(ctx, path)

	default:
		return "", "", fmt.Errorf("unknown resource: %s", uri)
	}
}

// readClusterInfo returns cluster topology information.
func (r *Registry) readClusterInfo(ctx context.Context) (string, string, error) {
	info, err := r.client.GetClusterInfo(ctx)
	if err != nil {
		return "", "", err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", "", err
	}

	return string(data), "application/json", nil
}

// readNamespaceResource handles namespace-related resources.
func (r *Registry) readNamespaceResource(ctx context.Context, path string) (string, string, error) {
	// Parse path: ns/{name}, ns/{name}/sets, ns/{name}/indexes
	parts := strings.Split(strings.TrimPrefix(path, "ns/"), "/")
	if len(parts) == 0 {
		return "", "", fmt.Errorf("invalid namespace path: %s", path)
	}

	namespace := parts[0]

	if len(parts) == 1 {
		// Return namespace info
		info, err := r.client.DescribeNamespace(ctx, namespace)
		if err != nil {
			return "", "", err
		}
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return "", "", err
		}
		return string(data), "application/json", nil
	}

	switch parts[1] {
	case "sets":
		sets, err := r.client.ListSets(ctx, namespace)
		if err != nil {
			return "", "", err
		}
		data, err := json.MarshalIndent(sets, "", "  ")
		if err != nil {
			return "", "", err
		}
		return string(data), "application/json", nil

	case "indexes":
		indexes, err := r.client.ListIndexes(ctx, namespace)
		if err != nil {
			return "", "", err
		}
		data, err := json.MarshalIndent(indexes, "", "  ")
		if err != nil {
			return "", "", err
		}
		return string(data), "application/json", nil

	default:
		return "", "", fmt.Errorf("unknown namespace resource: %s", parts[1])
	}
}

// readUDFs returns registered UDF modules.
func (r *Registry) readUDFs(ctx context.Context) (string, string, error) {
	udfs, err := r.client.ListUDFs(ctx)
	if err != nil {
		return "", "", err
	}

	data, err := json.MarshalIndent(map[string]interface{}{
		"udfs": udfs,
	}, "", "  ")
	if err != nil {
		return "", "", err
	}

	return string(data), "application/json", nil
}

// readSchema returns inferred schema for a set.
func (r *Registry) readSchema(ctx context.Context, path string) (string, string, error) {
	// Parse path: schema/{ns}/{set}
	re := regexp.MustCompile(`^schema/([^/]+)/([^/]+)$`)
	matches := re.FindStringSubmatch(path)
	if len(matches) != 3 {
		return "", "", fmt.Errorf("invalid schema path: %s", path)
	}

	namespace := matches[1]
	setName := matches[2]

	// Sample a few records to infer schema
	records, err := r.client.ScanSet(ctx, namespace, setName, nil, 10, 0)
	if err != nil {
		return "", "", err
	}

	// Build schema from sampled records
	schema := inferSchema(records)

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", "", err
	}

	return string(data), "application/json", nil
}

// BinSchema represents inferred schema for a bin.
type BinSchema struct {
	Name     string      `json:"name"`
	Types    []string    `json:"types"`
	Nullable bool        `json:"nullable"`
	Sample   interface{} `json:"sample,omitempty"`
}

// SetSchema represents inferred schema for a set.
type SetSchema struct {
	Namespace  string      `json:"namespace"`
	Set        string      `json:"set"`
	Bins       []BinSchema `json:"bins"`
	SampleSize int         `json:"sample_size"`
}

// inferSchema builds a schema from sampled records.
func inferSchema(records []*aerospike.Record) *SetSchema {
	if len(records) == 0 {
		return &SetSchema{Bins: []BinSchema{}}
	}

	binTypes := make(map[string]map[string]bool)
	binSamples := make(map[string]interface{})
	binNullable := make(map[string]bool)

	for _, rec := range records {
		if rec == nil {
			continue
		}
		for binName, value := range rec.Bins {
			if _, ok := binTypes[binName]; !ok {
				binTypes[binName] = make(map[string]bool)
				binSamples[binName] = value
			}
			if value == nil {
				binNullable[binName] = true
			} else {
				binTypes[binName][getTypeName(value)] = true
			}
		}
	}

	bins := make([]BinSchema, 0, len(binTypes))
	for name, types := range binTypes {
		typeList := make([]string, 0, len(types))
		for t := range types {
			typeList = append(typeList, t)
		}
		bins = append(bins, BinSchema{
			Name:     name,
			Types:    typeList,
			Nullable: binNullable[name],
			Sample:   binSamples[name],
		})
	}

	schema := &SetSchema{
		Bins:       bins,
		SampleSize: len(records),
	}

	if len(records) > 0 && records[0] != nil {
		schema.Namespace = records[0].Namespace
		schema.Set = records[0].Set
	}

	return schema
}

// getTypeName returns the type name for a value.
func getTypeName(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case int, int32, int64:
		return "integer"
	case float32, float64:
		return "float"
	case bool:
		return "boolean"
	case []interface{}:
		return "list"
	case map[string]interface{}, map[interface{}]interface{}:
		return "map"
	case []byte:
		return "bytes"
	default:
		return fmt.Sprintf("%T", v)
	}
}
