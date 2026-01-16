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

// With adds or updates a header value.
// This allows customization of the security headers.
//
// Example:
//
//	headers := common.DefaultSecurityHeaders()
//	headers = headers.With("Content-Security-Policy", "default-src 'self' https://cdn.example.com")
//	headers = headers.With("X-Custom-Header", "value")
func (s SecurityHeaders) With(key, value string) SecurityHeaders {
	s.headers[key] = value
	return s
}

// Without removes a header.
// Useful when you need to disable a specific security header.
//
// Example:
//
//	headers := common.DefaultSecurityHeaders()
//	headers = headers.Without("X-XSS-Protection") // Remove legacy header
func (s SecurityHeaders) Without(key string) SecurityHeaders {
	delete(s.headers, key)
	return s
}

// Apply applies all security headers to an http.ResponseWriter.
// This should be called before writing the response body.
func (s SecurityHeaders) Apply(w http.ResponseWriter) {
	for k, v := range s.headers {
		// Only set non-empty headers
		if v != "" || k == "X-Powered-By" || k == "Server" {
			w.Header().Set(k, v)
		}
	}
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
