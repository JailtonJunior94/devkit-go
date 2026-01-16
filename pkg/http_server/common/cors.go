package common

import (
	"errors"
	"strings"
)

// ParseOrigins splits comma-separated origins and validates the configuration.
// Returns an error if wildcard (*) is combined with other origins.
func ParseOrigins(origins string) ([]string, error) {
	trimmed := strings.TrimSpace(origins)

	if trimmed == "" {
		return []string{}, nil
	}

	// Handle wildcard as single value
	if trimmed == "*" {
		return []string{"*"}, nil
	}

	// Split by comma and trim each part
	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))
	hasWildcard := false

	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}

		// Check for wildcard
		if p == "*" {
			hasWildcard = true
		}

		result = append(result, p)
	}

	// SECURITY: Wildcard cannot be combined with other origins
	// This prevents misconfigurations where someone might think
	// "*,https://example.com" would allow all plus specific origin
	if hasWildcard && len(result) > 1 {
		return nil, errors.New("wildcard (*) cannot be combined with other origins")
	}

	return result, nil
}

// IsOriginAllowed checks if the given origin is in the allowed list.
func IsOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}

	// Check for wildcard
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		return true
	}

	// Check exact match
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
}
