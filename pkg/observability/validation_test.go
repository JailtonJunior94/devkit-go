package observability

import (
	"testing"
)

func TestCardinalityValidator_Validate(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			validator := NewCardinalityValidator(tt.enabled)
			err := validator.Validate(tt.fields)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestCardinalityValidator_CustomLabels(t *testing.T) {
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
		t.Run(tt.label, func(t *testing.T) {
			result := validator.IsBlocked(tt.label)
			if result != tt.expected {
				t.Errorf("IsBlocked(%s) = %v, want %v", tt.label, result, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexAny(s, substr) >= 0))
}

func indexAny(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
