package common

import "net/http"

// SecurityHeaders holds HTTP security headers configuration.
// These headers protect against common web vulnerabilities.
type SecurityHeaders struct {
	headers map[string]string
}

// DefaultSecurityHeaders returns a SecurityHeaders with secure defaults.
// This configuration follows OWASP recommendations and modern security best practices.
//
// Headers included:
//   - X-Frame-Options: Prevents clickjacking attacks
//   - X-Content-Type-Options: Prevents MIME-sniffing attacks
//   - X-XSS-Protection: Legacy XSS protection (modern browsers use CSP instead)
//   - Strict-Transport-Security: Enforces HTTPS connections
//   - Content-Security-Policy: Prevents XSS and data injection attacks
//   - Referrer-Policy: Controls referrer information leakage
//   - Permissions-Policy: Restricts browser features
//
// CSP Policy: Restrictive by default (only allows same-origin content)
// You may need to customize CSP for your specific application needs.
func DefaultSecurityHeaders() SecurityHeaders {
	return SecurityHeaders{
		headers: map[string]string{
			// Frame protection
			"X-Frame-Options": "DENY",

			// MIME-sniffing protection
			"X-Content-Type-Options": "nosniff",

			// XSS protection (legacy, CSP is preferred)
			"X-XSS-Protection": "1; mode=block",

			// HSTS: Force HTTPS for 1 year, include subdomains, allow preload
			"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",

			// Content Security Policy (restrictive by default)
			// This prevents XSS, data injection, and other code injection attacks
			"Content-Security-Policy": "default-src 'self'; " +
				"script-src 'self'; " +
				"style-src 'self' 'unsafe-inline'; " + // Allow inline styles (common requirement)
				"img-src 'self' data: https:; " + // Allow images from same origin, data URIs, and HTTPS
				"font-src 'self'; " +
				"connect-src 'self'; " + // API calls only to same origin
				"frame-ancestors 'none'; " + // Equivalent to X-Frame-Options: DENY
				"base-uri 'self'; " + // Prevent base tag injection
				"form-action 'self'", // Forms can only submit to same origin

			// Referrer Policy: Send full URL to same origin, origin only to others
			"Referrer-Policy": "strict-origin-when-cross-origin",

			// Permissions Policy: Disable dangerous browser features
			// This prevents access to sensitive APIs
			"Permissions-Policy": "geolocation=(), camera=(), microphone=(), " +
				"payment=(), usb=(), magnetometer=(), gyroscope=(), " +
				"accelerometer=(), ambient-light-sensor=()",

			// Remove server identification headers
			"X-Powered-By": "",
			"Server":       "",
		},
	}
}

// With returns a new SecurityHeaders with the given header value added or
// updated. The receiver is not mutated, ensuring concurrent-safe sharing
// of a default configuration across servers.
//
// Example:
//
//	headers := common.DefaultSecurityHeaders()
//	headers = headers.With("Content-Security-Policy", "default-src 'self' https://cdn.example.com")
//	headers = headers.With("X-Custom-Header", "value")
func (s SecurityHeaders) With(key, value string) SecurityHeaders {
	clone := s.cloneHeaders()
	clone[key] = value
	return SecurityHeaders{headers: clone}
}

// Without returns a new SecurityHeaders with the given header removed.
// The receiver is not mutated.
//
// Example:
//
//	headers := common.DefaultSecurityHeaders()
//	headers = headers.Without("X-XSS-Protection") // Remove legacy header
func (s SecurityHeaders) Without(key string) SecurityHeaders {
	clone := s.cloneHeaders()
	delete(clone, key)
	return SecurityHeaders{headers: clone}
}

// Apply writes all configured security headers to w.
// Headers configured with an empty value are removed via Header().Del,
// which is the right behavior for stripping server identification headers
// (Server, X-Powered-By).
func (s SecurityHeaders) Apply(w http.ResponseWriter) {
	header := w.Header()
	for k, v := range s.headers {
		if v == "" {
			header.Del(k)
			continue
		}
		header.Set(k, v)
	}
}

// cloneHeaders returns an independent copy of the underlying map. It is
// safe for use even when the receiver was zero-initialized.
func (s SecurityHeaders) cloneHeaders() map[string]string {
	clone := make(map[string]string, len(s.headers)+1)
	for k, v := range s.headers {
		clone[k] = v
	}
	return clone
}

// ToMap returns a copy of the headers map.
// Useful for debugging or logging security configuration.
func (s SecurityHeaders) ToMap() map[string]string {
	result := make(map[string]string, len(s.headers))
	for k, v := range s.headers {
		result[k] = v
	}
	return result
}
