// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"strings"
	"testing"
)

func TestValidateNamespace(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())

	tests := []struct {
		name      string
		namespace string
		wantErr   bool
	}{
		{"valid", "test_namespace", false},
		{"valid with hyphen", "test-namespace", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 32), true},
		{"invalid chars", "test@namespace", true},
		{"with spaces", "test namespace", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateNamespace(tt.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNamespace(%s) error = %v, wantErr %v", tt.namespace, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSetName(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())

	tests := []struct {
		name    string
		setName string
		wantErr bool
	}{
		{"valid", "users", false},
		{"empty (optional)", "", false},
		{"valid with underscore", "user_profiles", false},
		{"too long", strings.Repeat("a", 64), true},
		{"invalid chars", "users!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateSetName(tt.setName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSetName(%s) error = %v, wantErr %v", tt.setName, err, tt.wantErr)
			}
		})
	}
}

func TestValidateKey(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid", "user123", false},
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 1025), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey(%s) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBinName(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())

	tests := []struct {
		name    string
		binName string
		wantErr bool
	}{
		{"valid", "name", false},
		{"valid with underscore", "first_name", false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", 16), true},  // 16 chars > 15 limit
		{"at limit", strings.Repeat("a", 15), false}, // 15 chars = limit
		{"invalid chars", "name@field", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateBinName(tt.binName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBinName(%s) error = %v, wantErr %v", tt.binName, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBins(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())

	validBins := map[string]interface{}{
		"name":  "John",
		"age":   30,
		"email": "john@example.com",
	}

	if err := v.ValidateBins(validBins); err != nil {
		t.Errorf("ValidateBins should pass for valid bins: %v", err)
	}

	invalidBins := map[string]interface{}{
		"valid_name": "value",
		"invalid@":   "value",
	}

	if err := v.ValidateBins(invalidBins); err == nil {
		t.Error("ValidateBins should fail for invalid bin names")
	}
}

func TestValidateBatchSize(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())

	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{"valid", 100, false},
		{"at limit", 5000, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"too large", 5001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateBatchSize(tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBatchSize(%d) error = %v, wantErr %v", tt.size, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUDFCode(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())

	tests := []struct {
		name    string
		code    string
		wantErr bool
	}{
		{"valid lua", "function hello() return 'world' end", false},
		{"empty", "", true},
		{"dangerous os.execute", "os.execute('rm -rf /')", true},
		{"dangerous io.popen", "io.popen('ls')", true},
		{"dangerous loadfile", "loadfile('/etc/passwd')", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateUDFCode(tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUDFCode error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateModuleName(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())

	tests := []struct {
		name       string
		moduleName string
		wantErr    bool
	}{
		{"valid", "mymodule.lua", false},
		{"valid uppercase", "MyModule.LUA", false},
		{"empty", "", true},
		{"no extension", "mymodule", true},
		{"wrong extension", "mymodule.py", true},
		{"too long", strings.Repeat("a", 125) + ".lua", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateModuleName(tt.moduleName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModuleName(%s) error = %v, wantErr %v", tt.moduleName, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal string", "hello world", "hello world"},
		{"with null byte", "hello\x00world", "helloworld"},
		{"with control chars", "hello\x01\x02world", "helloworld"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
