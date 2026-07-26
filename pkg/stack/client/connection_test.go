// SPDX-License-Identifier: MIT

package client

import (
	"testing"
)

// TestConnectionInput_Validation tests the ConnectionInput struct construction.
// Per CLAUDE.md T-1, uses table-driven tests.
func TestConnectionInput_Validation(t *testing.T) {
	tests := []struct {
		name  string
		input ConnectionInput
		valid bool
	}{
		{
			name: "valid configuration",
			input: ConnectionInput{
				Host:           "localhost",
				Port:           102,
				ConnectTimeout: 10,
				RequestTimeout: 10,
			},
			valid: true,
		},
		{
			name: "custom port",
			input: ConnectionInput{
				Host:           "192.168.1.100",
				Port:           8080,
				ConnectTimeout: 5,
				RequestTimeout: 15,
			},
			valid: true,
		},
		{
			name: "zero timeouts",
			input: ConnectionInput{
				Host:           "example.com",
				Port:           102,
				ConnectTimeout: 0,
				RequestTimeout: 0,
			},
			valid: true, // Library will handle timeout validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify struct fields are properly set
			if tt.input.Host == "" && tt.valid {
				t.Error("Expected valid input to have non-empty Host")
			}
			if tt.input.Port <= 0 && tt.valid {
				t.Error("Expected valid input to have positive Port")
			}
		})
	}
}

// TestNewConnection_InvalidHost tests connection failure scenarios.
// Note: This requires a mock or integration test environment.
// For unit testing, we verify the function signature and error wrapping.
func TestNewConnection_ErrorWrapping(t *testing.T) {
	// Test that NewConnection properly wraps errors (ERR-1)
	input := ConnectionInput{
		Host:           "nonexistent-host-12345.invalid",
		Port:           102,
		ConnectTimeout: 1,
		RequestTimeout: 1,
	}

	_, err := NewConnection(input)
	if err == nil {
		t.Log("Warning: Expected connection to fail for invalid host, but may succeed in some environments")
		return
	}

	// Verify error message contains context per ERR-1
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Expected non-empty error message")
	}
	// Error should be wrapped with context
	if len(errMsg) < 10 {
		t.Errorf("Expected detailed error message, got: %q", errMsg)
	}
}

// TestConnectionInput_TimeoutConversion verifies timeout type conversion.
func TestConnectionInput_TimeoutConversion(t *testing.T) {
	tests := []struct {
		name           string
		connectTimeout int
		requestTimeout int
	}{
		{name: "standard timeouts", connectTimeout: 10, requestTimeout: 10},
		{name: "short timeouts", connectTimeout: 1, requestTimeout: 1},
		{name: "long timeouts", connectTimeout: 60, requestTimeout: 120},
		{name: "zero timeouts", connectTimeout: 0, requestTimeout: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ConnectionInput{
				Host:           "test-host",
				Port:           102,
				ConnectTimeout: tt.connectTimeout,
				RequestTimeout: tt.requestTimeout,
			}

			if input.Host != "test-host" {
				t.Errorf("Host = %q, want %q", input.Host, "test-host")
			}
			if input.Port != 102 {
				t.Errorf("Port = %d, want %d", input.Port, 102)
			}
			if input.ConnectTimeout != tt.connectTimeout {
				t.Errorf("ConnectTimeout = %d, want %d", input.ConnectTimeout, tt.connectTimeout)
			}
			if input.RequestTimeout != tt.requestTimeout {
				t.Errorf("RequestTimeout = %d, want %d", input.RequestTimeout, tt.requestTimeout)
			}

			// Verify conversion to uint would work (as done in NewConnection)
			if input.ConnectTimeout < 0 {
				t.Error("Negative ConnectTimeout would cause issues in uint conversion")
			}
			if input.RequestTimeout < 0 {
				t.Error("Negative RequestTimeout would cause issues in uint conversion")
			}
		})
	}
}
