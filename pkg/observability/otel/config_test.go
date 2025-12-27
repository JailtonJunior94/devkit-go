package otel

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSecurityConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "production with insecure should fail",
			config: &Config{
				Environment: "production",
				Insecure:    true,
			},
			wantErr: true,
			errMsg:  "insecure connections are not allowed in production environment",
		},
		{
			name: "prod with insecure should fail",
			config: &Config{
				Environment: "prod",
				Insecure:    true,
			},
			wantErr: true,
			errMsg:  "insecure connections are not allowed in production environment",
		},
		{
			name: "development with insecure is ok",
			config: &Config{
				Environment: "development",
				Insecure:    true,
			},
			wantErr: false,
		},
		{
			name: "test with insecure is ok",
			config: &Config{
				Environment: "test",
				Insecure:    true,
			},
			wantErr: false,
		},
		{
			name: "production with secure is ok",
			config: &Config{
				Environment: "production",
				Insecure:    false,
			},
			wantErr: false,
		},
		{
			name: "TLS version too low should fail",
			config: &Config{
				Environment: "production",
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS10,
				},
			},
			wantErr: true,
			errMsg:  "minimum TLS version must be 1.2 or higher",
		},
		{
			name: "TLS version 1.2 is ok",
			config: &Config{
				Environment: "production",
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
			wantErr: false,
		},
		{
			name: "TLS version 1.3 is ok",
			config: &Config{
				Environment: "production",
				TLSConfig: &tls.Config{
					MinVersion: tls.VersionTLS13,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecurityConfig(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeProtocol(t *testing.T) {
	tests := []struct {
		input    string
		expected OTLPProtocol
	}{
		{"grpc", ProtocolGRPC},
		{"GRPC", ProtocolGRPC},
		{"http", ProtocolHTTP},
		{"HTTP", ProtocolHTTP},
		{"http/protobuf", ProtocolHTTP},
		{"", ProtocolGRPC},        // default
		{"invalid", ProtocolGRPC}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeProtocol(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	serviceName := "test-service"
	config := DefaultConfig(serviceName)

	assert.Equal(t, serviceName, config.ServiceName)
	assert.Equal(t, "unknown", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
	assert.Equal(t, ProtocolGRPC, config.OTLPProtocol)
	assert.Equal(t, 1.0, config.TraceSampleRate)
	assert.False(t, config.Insecure) // Should be secure by default
}
