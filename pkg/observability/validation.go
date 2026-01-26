package observability

import (
	"fmt"
	"strings"
)

// HighCardinalityLabels contains a list of label keys that should not be used in metrics
// as they can cause cardinality explosion in Prometheus/OpenTelemetry backends.
var HighCardinalityLabels = []string{
	"user_id",
	"session_id",
	"trace_id",
	"span_id",
	"request_id",
	"transaction_id",
	"correlation_id",
	"ip_address",
	"email",
	"phone",
	"uuid",
	"guid",
}

// CardinalityValidator validates metric labels to prevent high cardinality issues.
type CardinalityValidator struct {
	blockedLabels map[string]bool
	enabled       bool
}

// NewCardinalityValidator creates a new validator with default blocked labels.
func NewCardinalityValidator(enabled bool) *CardinalityValidator {
	blockedMap := make(map[string]bool)
	for _, label := range HighCardinalityLabels {
		blockedMap[label] = true
	}

	return &CardinalityValidator{
		blockedLabels: blockedMap,
		enabled:       enabled,
	}
}

// NewCardinalityValidatorWithCustomLabels creates a validator with custom blocked labels.
func NewCardinalityValidatorWithCustomLabels(enabled bool, customBlockedLabels []string) *CardinalityValidator {
	blockedMap := make(map[string]bool)
	for _, label := range customBlockedLabels {
		blockedMap[label] = true
	}

	return &CardinalityValidator{
		blockedLabels: blockedMap,
		enabled:       enabled,
	}
}

// Validate checks if any field contains a high-cardinality label.
// Returns an error if validation is enabled and a blocked label is found.
func (v *CardinalityValidator) Validate(fields []Field) error {
	if !v.enabled {
		return nil
	}

	for _, field := range fields {
		normalizedKey := strings.ToLower(field.Key)
		if v.blockedLabels[normalizedKey] {
			return fmt.Errorf(
				"high-cardinality label '%s' is not allowed in metrics; use low-cardinality alternatives like type, category, or status",
				field.Key,
			)
		}
	}

	return nil
}

// AddBlockedLabel adds a new label to the blocklist.
func (v *CardinalityValidator) AddBlockedLabel(label string) {
	v.blockedLabels[strings.ToLower(label)] = true
}

// RemoveBlockedLabel removes a label from the blocklist.
func (v *CardinalityValidator) RemoveBlockedLabel(label string) {
	delete(v.blockedLabels, strings.ToLower(label))
}

// IsBlocked checks if a specific label is blocked.
func (v *CardinalityValidator) IsBlocked(label string) bool {
	return v.blockedLabels[strings.ToLower(label)]
}
