package observability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceDescriptor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		serviceName string
		version     string
		environment string
		wantErr     error
		wantMessage string
	}{
		{
			name:        "normalizes valid values",
			serviceName: "  checkout-api  ",
			version:     "  1.2.3  ",
			environment: "  production  ",
		},
		{
			name:        "rejects empty service name",
			serviceName: "   ",
			version:     "1.2.3",
			environment: "production",
			wantErr:     ErrInvalidConfig,
			wantMessage: "observability: invalid config: service name is required",
		},
		{
			name:        "rejects empty version",
			serviceName: "checkout-api",
			version:     " ",
			environment: "production",
			wantErr:     ErrInvalidConfig,
			wantMessage: "observability: invalid config: service version is required",
		},
		{
			name:        "rejects empty environment",
			serviceName: "checkout-api",
			version:     "1.2.3",
			environment: "\t",
			wantErr:     ErrInvalidConfig,
			wantMessage: "observability: invalid config: service environment is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			descriptor, err := NewServiceDescriptor(tt.serviceName, tt.version, tt.environment)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected errors.Is(%v), got %v", tt.wantErr, err)
				}
				if err.Error() != tt.wantMessage {
					t.Fatalf("unexpected message: %q", err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if descriptor.Name() != "checkout-api" {
				t.Fatalf("unexpected name: %q", descriptor.Name())
			}
			if descriptor.Version() != "1.2.3" {
				t.Fatalf("unexpected version: %q", descriptor.Version())
			}
			if descriptor.Environment() != "production" {
				t.Fatalf("unexpected environment: %q", descriptor.Environment())
			}
		})
	}
}

func TestPropagationHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		requestIDHeader     string
		correlationIDHeader string
		wantRequestID       string
		wantCorrelationID   string
		wantErr             error
		wantMessage         string
	}{
		{
			name:              "applies defaults",
			wantRequestID:     defaultRequestIDHeader,
			wantCorrelationID: defaultCorrelationIDHeader,
		},
		{
			name:                "normalizes custom values",
			requestIDHeader:     " X-Request-ID ",
			correlationIDHeader: " Correlation-ID ",
			wantRequestID:       "x-request-id",
			wantCorrelationID:   "correlation-id",
		},
		{
			name:            "rejects invalid request id header",
			requestIDHeader: "x request id",
			wantErr:         ErrInvalidHeaderName,
			wantMessage:     "observability: invalid propagation header name: x request id",
		},
		{
			name:                "rejects invalid correlation id header",
			correlationIDHeader: "correlation id",
			wantErr:             ErrInvalidHeaderName,
			wantMessage:         "observability: invalid propagation header name: correlation id",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			headers, err := NewPropagationHeaders(tt.requestIDHeader, tt.correlationIDHeader)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected errors.Is(%v), got %v", tt.wantErr, err)
				}
				if err.Error() != tt.wantMessage {
					t.Fatalf("unexpected message: %q", err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if headers.RequestIDHeader() != tt.wantRequestID {
				t.Fatalf("unexpected request header: %q", headers.RequestIDHeader())
			}
			if headers.CorrelationIDHeader() != tt.wantCorrelationID {
				t.Fatalf("unexpected correlation header: %q", headers.CorrelationIDHeader())
			}
		})
	}
}

func TestShutdownPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		timeout     time.Duration
		flushOrder  []string
		wantTimeout time.Duration
		wantOrder   []string
		wantErr     error
		wantMessage string
	}{
		{
			name:        "uses defaults",
			wantTimeout: defaultShutdownTimeout,
			wantOrder:   defaultShutdownFlushOrder,
		},
		{
			name:        "accepts custom values",
			timeout:     5 * time.Second,
			flushOrder:  []string{"exporters", "providers"},
			wantTimeout: 5 * time.Second,
			wantOrder:   []string{"exporters", "providers"},
		},
		{
			name:        "rejects negative timeout",
			timeout:     -1 * time.Second,
			wantErr:     ErrInvalidConfig,
			wantMessage: "observability: invalid config: shutdown timeout must be positive",
		},
		{
			name:        "rejects empty flush step",
			flushOrder:  []string{"exporters", " "},
			wantErr:     ErrInvalidConfig,
			wantMessage: "observability: invalid config: shutdown flush order contains an empty step",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			policy, err := NewShutdownPolicy(tt.timeout, tt.flushOrder)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected errors.Is(%v), got %v", tt.wantErr, err)
				}
				if err.Error() != tt.wantMessage {
					t.Fatalf("unexpected message: %q", err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if policy.Timeout() != tt.wantTimeout {
				t.Fatalf("unexpected timeout: %s", policy.Timeout())
			}

			gotOrder := policy.FlushOrder()
			if len(gotOrder) != len(tt.wantOrder) {
				t.Fatalf("unexpected order size: %d", len(gotOrder))
			}
			for i := range gotOrder {
				if gotOrder[i] != tt.wantOrder[i] {
					t.Fatalf("unexpected order at %d: %q", i, gotOrder[i])
				}
			}

			if len(gotOrder) > 0 {
				gotOrder[0] = "mutated"
				if policy.FlushOrder()[0] == "mutated" {
					t.Fatal("FlushOrder should return a defensive copy")
				}
			}
		})
	}
}

func TestBenchmarkBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		budgetName           string
		maxRegressionPercent float64
		baselineFile         string
		regression           float64
		wantBudget           float64
		wantErr              error
		wantMessage          string
	}{
		{
			name:         "uses default regression budget",
			budgetName:   "logger_hot_path",
			baselineFile: "testdata/logger.txt",
			regression:   defaultBenchmarkRegressionBudget,
			wantBudget:   defaultBenchmarkRegressionBudget,
		},
		{
			name:                 "accepts custom budget and regression below threshold",
			budgetName:           "logger_hot_path",
			maxRegressionPercent: 8,
			baselineFile:         "testdata/logger.txt",
			regression:           7.5,
			wantBudget:           8,
		},
		{
			name:                 "rejects missing benchmark name",
			maxRegressionPercent: 5,
			baselineFile:         "testdata/logger.txt",
			wantErr:              ErrInvalidConfig,
			wantMessage:          "observability: invalid config: benchmark name is required",
		},
		{
			name:                 "rejects invalid regression budget",
			budgetName:           "logger_hot_path",
			maxRegressionPercent: 101,
			baselineFile:         "testdata/logger.txt",
			wantErr:              ErrInvalidConfig,
			wantMessage:          "observability: invalid config: benchmark regression percent must be between 0 and 100",
		},
		{
			name:                 "rejects missing baseline file",
			budgetName:           "logger_hot_path",
			maxRegressionPercent: 5,
			baselineFile:         " ",
			wantErr:              ErrInvalidConfig,
			wantMessage:          "observability: invalid config: benchmark baseline file is required",
		},
		{
			name:                 "returns regression error above threshold",
			budgetName:           "logger_hot_path",
			maxRegressionPercent: 5,
			baselineFile:         "testdata/logger.txt",
			regression:           8,
			wantBudget:           5,
			wantErr:              ErrBenchmarkRegression,
			wantMessage:          "observability: benchmark regression: logger_hot_path exceeded 8.00%",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			budget, err := NewBenchmarkBudget(tt.budgetName, tt.maxRegressionPercent, tt.baselineFile)

			if tt.wantMessage != "" && tt.wantErr == ErrInvalidConfig {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected errors.Is(%v), got %v", tt.wantErr, err)
				}
				if err.Error() != tt.wantMessage {
					t.Fatalf("unexpected message: %q", err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected constructor error: %v", err)
			}

			if budget.Name() != tt.budgetName {
				t.Fatalf("unexpected name: %q", budget.Name())
			}
			if budget.MaxRegressionPercent() != tt.wantBudget {
				t.Fatalf("unexpected budget: %v", budget.MaxRegressionPercent())
			}
			if budget.BaselineFile() != tt.baselineFile {
				t.Fatalf("unexpected baseline file: %q", budget.BaselineFile())
			}

			err = budget.ValidateRegression(tt.regression)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected errors.Is(%v), got %v", tt.wantErr, err)
				}
				if err.Error() != tt.wantMessage {
					t.Fatalf("unexpected message: %q", err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected regression validation error: %v", err)
			}
		})
	}
}

func TestContractErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		target      error
		wantMessage string
		alsoIs      error
	}{
		{
			name:        "invalid config",
			err:         NewInvalidConfigError("service name is required"),
			target:      ErrInvalidConfig,
			wantMessage: "observability: invalid config: service name is required",
		},
		{
			name:        "invalid header name",
			err:         NewInvalidHeaderNameError("x request id"),
			target:      ErrInvalidHeaderName,
			wantMessage: "observability: invalid propagation header name: x request id",
		},
		{
			name:        "cardinality violation",
			err:         NewCardinalityViolationError("request_id"),
			target:      ErrCardinalityViolation,
			wantMessage: "observability: cardinality violation: request_id is not allowed",
		},
		{
			name:        "shutdown error keeps underlying cause",
			err:         NewShutdownError("trace exporter flush", context.DeadlineExceeded),
			target:      ErrShutdownFailed,
			wantMessage: "observability: shutdown failed: trace exporter flush: context deadline exceeded",
			alsoIs:      context.DeadlineExceeded,
		},
		{
			name:        "benchmark regression",
			err:         NewBenchmarkRegressionError("logger_hot_path", 8),
			target:      ErrBenchmarkRegression,
			wantMessage: "observability: benchmark regression: logger_hot_path exceeded 8.00%",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !errors.Is(tt.err, tt.target) {
				t.Fatalf("expected errors.Is(%v), got %v", tt.target, tt.err)
			}
			if tt.alsoIs != nil && !errors.Is(tt.err, tt.alsoIs) {
				t.Fatalf("expected errors.Is(%v), got %v", tt.alsoIs, tt.err)
			}
			if tt.err.Error() != tt.wantMessage {
				t.Fatalf("unexpected message: %q", tt.err.Error())
			}
		})
	}
}

func TestCardinalityValidator_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		enabled       bool
		fields        []Field
		expectError   bool
		errorContains string
	}{
		{
			name:    "disabled validator allows all labels",
			enabled: false,
			fields: []Field{
				String("user_id", "12345"),
				String("session_id", "abc-123"),
			},
			expectError: false,
		},
		{
			name:    "enabled validator blocks user_id",
			enabled: true,
			fields: []Field{
				String("user_id", "12345"),
			},
			expectError:   true,
			errorContains: "user_id",
		},
		{
			name:    "enabled validator blocks session_id",
			enabled: true,
			fields: []Field{
				String("session_id", "abc-123"),
			},
			expectError:   true,
			errorContains: "session_id",
		},
		{
			name:    "enabled validator blocks trace_id",
			enabled: true,
			fields: []Field{
				String("trace_id", "trace-123"),
			},
			expectError:   true,
			errorContains: "trace_id",
		},
		{
			name:    "enabled validator blocks request_id",
			enabled: true,
			fields: []Field{
				String("request_id", "req-456"),
			},
			expectError:   true,
			errorContains: "request_id",
		},
		{
			name:    "enabled validator allows low-cardinality labels",
			enabled: true,
			fields: []Field{
				String("status", "success"),
				String("method", "GET"),
				String("user_type", "premium"),
			},
			expectError: false,
		},
		{
			name:    "case insensitive blocking",
			enabled: true,
			fields: []Field{
				String("User_Id", "12345"), // Mixed case
			},
			expectError:   true,
			errorContains: "User_Id",
		},
		{
			name:    "blocks ip_address",
			enabled: true,
			fields: []Field{
				String("ip_address", "192.168.1.1"),
			},
			expectError:   true,
			errorContains: "ip_address",
		},
		{
			name:    "blocks email",
			enabled: true,
			fields: []Field{
				String("email", "user@example.com"),
			},
			expectError:   true,
			errorContains: "email",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			validator := NewCardinalityValidator(tt.enabled)
			err := validator.Validate(tt.fields)

			if tt.expectError {
				if !errors.Is(err, ErrCardinalityViolation) {
					t.Fatalf("expected errors.Is(%v), got %v", ErrCardinalityViolation, err)
				}
				if tt.errorContains != "" && !stringsContains(err.Error(), tt.errorContains) {
					t.Fatalf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}
		})
	}
}

func TestCardinalityValidator_CustomLabels(t *testing.T) {
	t.Parallel()

	validator := NewCardinalityValidatorWithCustomLabels(true, []string{
		"customer_id",
		"order_id",
	})

	t.Run("blocks custom label customer_id", func(t *testing.T) {
		err := validator.Validate([]Field{
			String("customer_id", "CUST-123"),
		})
		if err == nil {
			t.Error("expected error for custom blocked label customer_id")
		}
	})

	t.Run("blocks custom label order_id", func(t *testing.T) {
		err := validator.Validate([]Field{
			String("order_id", "ORD-456"),
		})
		if err == nil {
			t.Error("expected error for custom blocked label order_id")
		}
	})

	t.Run("allows non-blocked labels", func(t *testing.T) {
		err := validator.Validate([]Field{
			String("status", "completed"),
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCardinalityValidator_AddRemoveBlockedLabel(t *testing.T) {
	t.Parallel()

	validator := NewCardinalityValidator(true)

	// Add custom blocked label
	validator.AddBlockedLabel("custom_id")

	t.Run("blocks newly added label", func(t *testing.T) {
		err := validator.Validate([]Field{
			String("custom_id", "123"),
		})
		if err == nil {
			t.Error("expected error for newly blocked label")
		}
	})

	// Remove the label
	validator.RemoveBlockedLabel("custom_id")

	t.Run("allows label after removal", func(t *testing.T) {
		err := validator.Validate([]Field{
			String("custom_id", "123"),
		})
		if err != nil {
			t.Errorf("unexpected error after removing block: %v", err)
		}
	})
}

func TestCardinalityValidator_IsBlocked(t *testing.T) {
	t.Parallel()

	validator := NewCardinalityValidator(true)

	tests := []struct {
		label    string
		expected bool
	}{
		{"user_id", true},
		{"session_id", true},
		{"status", false},
		{"method", false},
		{"USER_ID", true}, // Case insensitive
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.label, func(t *testing.T) {
			t.Parallel()

			result := validator.IsBlocked(tt.label)
			if result != tt.expected {
				t.Errorf("IsBlocked(%s) = %v, want %v", tt.label, result, tt.expected)
			}
		})
	}
}

func stringsContains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
