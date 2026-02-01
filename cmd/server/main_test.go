package main

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		setEnv       bool
		defaultValue string
		want         string
	}{
		{
			name:         "returns environment variable value when set",
			envKey:       "TEST_MCP_HOST",
			envValue:     "0.0.0.0",
			setEnv:       true,
			defaultValue: "127.0.0.1",
			want:         "0.0.0.0",
		},
		{
			name:         "returns default when environment variable is empty",
			envKey:       "TEST_MCP_PORT_EMPTY",
			envValue:     "",
			setEnv:       true,
			defaultValue: "3000",
			want:         "3000",
		},
		{
			name:         "returns default when environment variable not set",
			envKey:       "TEST_MCP_UNSET_VAR",
			envValue:     "",
			setEnv:       false,
			defaultValue: "default_value",
			want:         "default_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing value
			os.Unsetenv(tt.envKey)

			if tt.setEnv {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			got := getEnv(tt.envKey, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv(%q, %q) = %q, want %q", tt.envKey, tt.defaultValue, got, tt.want)
			}
		})
	}
}
