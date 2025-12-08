// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Validator provides input validation for MCP operations.
type Validator struct {
	maxKeyLength       int
	maxBinNameLength   int
	maxNamespaceLength int
	maxSetNameLength   int
	maxBatchSize       int
	maxRecordSize      int
}

// ValidatorConfig holds validator configuration.
type ValidatorConfig struct {
	MaxKeyLength       int `json:"max_key_length"`
	MaxBinNameLength   int `json:"max_bin_name_length"`
	MaxNamespaceLength int `json:"max_namespace_length"`
	MaxSetNameLength   int `json:"max_set_name_length"`
	MaxBatchSize       int `json:"max_batch_size"`
	MaxRecordSize      int `json:"max_record_size"`
}

// DefaultValidatorConfig returns default validation configuration.
func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{
		MaxKeyLength:       1024,
		MaxBinNameLength:   15, // Aerospike limit
		MaxNamespaceLength: 31, // Aerospike limit
		MaxSetNameLength:   63, // Aerospike limit
		MaxBatchSize:       5000,
		MaxRecordSize:      1024 * 1024, // 1MB
	}
}

// NewValidator creates a new validator.
func NewValidator(cfg ValidatorConfig) *Validator {
	return &Validator{
		maxKeyLength:       cfg.MaxKeyLength,
		maxBinNameLength:   cfg.MaxBinNameLength,
		maxNamespaceLength: cfg.MaxNamespaceLength,
		maxSetNameLength:   cfg.MaxSetNameLength,
		maxBatchSize:       cfg.MaxBatchSize,
		maxRecordSize:      cfg.MaxRecordSize,
	}
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateNamespace validates a namespace name.
func (v *Validator) ValidateNamespace(namespace string) error {
	if namespace == "" {
		return ValidationError{Field: "namespace", Message: "cannot be empty"}
	}

	if len(namespace) > v.maxNamespaceLength {
		return ValidationError{
			Field:   "namespace",
			Message: fmt.Sprintf("exceeds maximum length of %d", v.maxNamespaceLength),
		}
	}

	if !isValidIdentifier(namespace) {
		return ValidationError{
			Field:   "namespace",
			Message: "contains invalid characters (must be alphanumeric, underscore, or hyphen)",
		}
	}

	return nil
}

// ValidateSetName validates a set name.
func (v *Validator) ValidateSetName(setName string) error {
	if setName == "" {
		return nil // Set name is optional
	}

	if len(setName) > v.maxSetNameLength {
		return ValidationError{
			Field:   "set_name",
			Message: fmt.Sprintf("exceeds maximum length of %d", v.maxSetNameLength),
		}
	}

	if !isValidIdentifier(setName) {
		return ValidationError{
			Field:   "set_name",
			Message: "contains invalid characters (must be alphanumeric, underscore, or hyphen)",
		}
	}

	return nil
}

// ValidateKey validates a record key.
func (v *Validator) ValidateKey(key string) error {
	if key == "" {
		return ValidationError{Field: "key", Message: "cannot be empty"}
	}

	if len(key) > v.maxKeyLength {
		return ValidationError{
			Field:   "key",
			Message: fmt.Sprintf("exceeds maximum length of %d", v.maxKeyLength),
		}
	}

	if !utf8.ValidString(key) {
		return ValidationError{Field: "key", Message: "must be valid UTF-8"}
	}

	return nil
}

// ValidateBinName validates a bin name.
func (v *Validator) ValidateBinName(binName string) error {
	if binName == "" {
		return ValidationError{Field: "bin_name", Message: "cannot be empty"}
	}

	if len(binName) > v.maxBinNameLength {
		return ValidationError{
			Field:   "bin_name",
			Message: fmt.Sprintf("exceeds maximum length of %d", v.maxBinNameLength),
		}
	}

	if !isValidIdentifier(binName) {
		return ValidationError{
			Field:   "bin_name",
			Message: "contains invalid characters (must be alphanumeric or underscore)",
		}
	}

	return nil
}

// ValidateBins validates bin names in a map.
func (v *Validator) ValidateBins(bins map[string]interface{}) error {
	for name := range bins {
		if err := v.ValidateBinName(name); err != nil {
			return err
		}
	}
	return nil
}

// ValidateBatchSize validates a batch operation size.
func (v *Validator) ValidateBatchSize(size int) error {
	if size <= 0 {
		return ValidationError{Field: "batch_size", Message: "must be positive"}
	}

	if size > v.maxBatchSize {
		return ValidationError{
			Field:   "batch_size",
			Message: fmt.Sprintf("exceeds maximum of %d", v.maxBatchSize),
		}
	}

	return nil
}

// ValidateIndexName validates an index name.
func (v *Validator) ValidateIndexName(indexName string) error {
	if indexName == "" {
		return ValidationError{Field: "index_name", Message: "cannot be empty"}
	}

	if len(indexName) > 256 {
		return ValidationError{
			Field:   "index_name",
			Message: "exceeds maximum length of 256",
		}
	}

	if !isValidIdentifier(indexName) {
		return ValidationError{
			Field:   "index_name",
			Message: "contains invalid characters",
		}
	}

	return nil
}

// ValidateUDFCode validates UDF Lua code.
func (v *Validator) ValidateUDFCode(code string) error {
	if code == "" {
		return ValidationError{Field: "code", Message: "cannot be empty"}
	}

	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"os.execute",
		"io.popen",
		"loadfile",
		"dofile",
	}

	codeLower := strings.ToLower(code)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(codeLower, pattern) {
			return ValidationError{
				Field:   "code",
				Message: fmt.Sprintf("contains potentially dangerous function: %s", pattern),
			}
		}
	}

	return nil
}

// ValidateModuleName validates a UDF module name.
func (v *Validator) ValidateModuleName(moduleName string) error {
	if moduleName == "" {
		return ValidationError{Field: "module_name", Message: "cannot be empty"}
	}

	if len(moduleName) > 128 {
		return ValidationError{
			Field:   "module_name",
			Message: "exceeds maximum length of 128",
		}
	}

	// Must end with .lua
	if !strings.HasSuffix(strings.ToLower(moduleName), ".lua") {
		return ValidationError{
			Field:   "module_name",
			Message: "must end with .lua extension",
		}
	}

	return nil
}

// SanitizeString removes potentially dangerous characters.
func SanitizeString(s string) string {
	// Remove null bytes and control characters
	var result strings.Builder
	for _, r := range s {
		if r >= 32 && r != 127 {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// isValidIdentifier checks if a string is a valid identifier.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, s)
	return matched
}
