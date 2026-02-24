package observability

import (
	"fmt"
	"strings"
	"sync"
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
// It is safe for concurrent use.
type CardinalityValidator struct {
	mu            sync.RWMutex
	blockedLabels map[string]bool
	enabled       bool
}

// NewCardinalityValidator creates a new validator with default blocked labels.
func NewCardinalityValidator(enabled bool) *CardinalityValidator {
	blockedMap := make(map[string]bool, len(HighCardinalityLabels))
	for _, label := range HighCardinalityLabels {
		blockedMap[label] = true
	}

	return &CardinalityValidator{
		blockedLabels: blockedMap,
		enabled:       enabled,
	}
}

// NewCardinalityValidatorWithCustomLabels creates a validator with default blocked labels
// plus additional custom labels. Custom labels are added on top of the defaults.
func NewCardinalityValidatorWithCustomLabels(enabled bool, customBlockedLabels []string) *CardinalityValidator {
	blockedMap := make(map[string]bool, len(HighCardinalityLabels)+len(customBlockedLabels))

	// Always include the default high-cardinality labels
	for _, label := range HighCardinalityLabels {
		blockedMap[label] = true
	}

	// Add custom labels on top of defaults
	for _, label := range customBlockedLabels {
		blockedMap[strings.ToLower(label)] = true
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

	v.mu.RLock()
	defer v.mu.RUnlock()

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
	v.mu.Lock()
	defer v.mu.Unlock()
	v.blockedLabels[strings.ToLower(label)] = true
}

// RemoveBlockedLabel removes a label from the blocklist.
func (v *CardinalityValidator) RemoveBlockedLabel(label string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.blockedLabels, strings.ToLower(label))
}

// IsBlocked checks if a specific label is blocked.
func (v *CardinalityValidator) IsBlocked(label string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.blockedLabels[strings.ToLower(label)]
}
