package observability

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultRequestIDHeader           = "x-request-id"
	defaultCorrelationIDHeader       = "correlation-id"
	defaultShutdownTimeout           = 15 * time.Second
	defaultBenchmarkRegressionBudget = 5.0
)

var defaultShutdownFlushOrder = []string{
	"tracer_provider",
	"meter_provider",
	"logger_provider",
}

var (
	ErrInvalidConfig        = errors.New("invalid observability config")
	ErrInvalidHeaderName    = errors.New("invalid propagation header name")
	ErrCardinalityViolation = errors.New("high-cardinality label not allowed")
	ErrShutdownFailed       = errors.New("observability shutdown failed")
	ErrBenchmarkRegression  = errors.New("observability benchmark regression")
)

type contractError struct {
	message string
	causes  []error
}

func (e *contractError) Error() string {
	return e.message
}

func (e *contractError) Unwrap() []error {
	return e.causes
}

type ServiceDescriptor struct {
	name        string
	version     string
	environment string
}

func NewServiceDescriptor(name, version, environment string) (ServiceDescriptor, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return ServiceDescriptor{}, NewInvalidConfigError("service name is required")
	}

	normalizedVersion := strings.TrimSpace(version)
	if normalizedVersion == "" {
		return ServiceDescriptor{}, NewInvalidConfigError("service version is required")
	}

	normalizedEnvironment := strings.TrimSpace(environment)
	if normalizedEnvironment == "" {
		return ServiceDescriptor{}, NewInvalidConfigError("service environment is required")
	}

	return ServiceDescriptor{
		name:        normalizedName,
		version:     normalizedVersion,
		environment: normalizedEnvironment,
	}, nil
}

func (d ServiceDescriptor) Name() string        { return d.name }
func (d ServiceDescriptor) Version() string     { return d.version }
func (d ServiceDescriptor) Environment() string { return d.environment }

type PropagationHeaders struct {
	requestIDHeader     string
	correlationIDHeader string
}

func DefaultPropagationHeaders() PropagationHeaders {
	return PropagationHeaders{
		requestIDHeader:     defaultRequestIDHeader,
		correlationIDHeader: defaultCorrelationIDHeader,
	}
}

func NewPropagationHeaders(requestIDHeader, correlationIDHeader string) (PropagationHeaders, error) {
	headers := DefaultPropagationHeaders()

	if normalized := normalizeHeaderName(requestIDHeader); normalized != "" {
		if !isValidHeaderName(normalized) {
			return PropagationHeaders{}, NewInvalidHeaderNameError(strings.TrimSpace(requestIDHeader))
		}
		headers.requestIDHeader = normalized
	}

	if normalized := normalizeHeaderName(correlationIDHeader); normalized != "" {
		if !isValidHeaderName(normalized) {
			return PropagationHeaders{}, NewInvalidHeaderNameError(strings.TrimSpace(correlationIDHeader))
		}
		headers.correlationIDHeader = normalized
	}

	return headers, nil
}

func (h PropagationHeaders) RequestIDHeader() string     { return h.requestIDHeader }
func (h PropagationHeaders) CorrelationIDHeader() string { return h.correlationIDHeader }

type ShutdownPolicy struct {
	timeout    time.Duration
	flushOrder []string
}

func DefaultShutdownPolicy() ShutdownPolicy {
	return ShutdownPolicy{
		timeout:    defaultShutdownTimeout,
		flushOrder: append([]string(nil), defaultShutdownFlushOrder...),
	}
}

func NewShutdownPolicy(timeout time.Duration, flushOrder []string) (ShutdownPolicy, error) {
	policy := DefaultShutdownPolicy()

	if timeout < 0 {
		return ShutdownPolicy{}, NewInvalidConfigError("shutdown timeout must be positive")
	}
	if timeout > 0 {
		policy.timeout = timeout
	}

	if len(flushOrder) == 0 {
		return policy, nil
	}

	normalizedOrder := make([]string, 0, len(flushOrder))
	for _, step := range flushOrder {
		normalizedStep := strings.TrimSpace(step)
		if normalizedStep == "" {
			return ShutdownPolicy{}, NewInvalidConfigError("shutdown flush order contains an empty step")
		}
		normalizedOrder = append(normalizedOrder, normalizedStep)
	}

	policy.flushOrder = normalizedOrder
	return policy, nil
}

func (p ShutdownPolicy) Timeout() time.Duration { return p.timeout }

func (p ShutdownPolicy) FlushOrder() []string {
	return append([]string(nil), p.flushOrder...)
}

type BenchmarkBudget struct {
	name                 string
	maxRegressionPercent float64
	baselineFile         string
}

func NewBenchmarkBudget(name string, maxRegressionPercent float64, baselineFile string) (BenchmarkBudget, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" {
		return BenchmarkBudget{}, NewInvalidConfigError("benchmark name is required")
	}

	normalizedBaseline := strings.TrimSpace(baselineFile)
	if normalizedBaseline == "" {
		return BenchmarkBudget{}, NewInvalidConfigError("benchmark baseline file is required")
	}

	normalizedBudget := maxRegressionPercent
	if normalizedBudget == 0 {
		normalizedBudget = defaultBenchmarkRegressionBudget
	}
	if normalizedBudget <= 0 || normalizedBudget > 100 {
		return BenchmarkBudget{}, NewInvalidConfigError("benchmark regression percent must be between 0 and 100")
	}

	return BenchmarkBudget{
		name:                 normalizedName,
		maxRegressionPercent: normalizedBudget,
		baselineFile:         normalizedBaseline,
	}, nil
}

func (b BenchmarkBudget) Name() string                  { return b.name }
func (b BenchmarkBudget) MaxRegressionPercent() float64 { return b.maxRegressionPercent }
func (b BenchmarkBudget) BaselineFile() string          { return b.baselineFile }

func (b BenchmarkBudget) ValidateRegression(actualPercent float64) error {
	if actualPercent <= b.maxRegressionPercent {
		return nil
	}
	return NewBenchmarkRegressionError(b.name, actualPercent)
}

func NewInvalidConfigError(detail string) error {
	message := "observability: invalid config"
	if normalized := strings.TrimSpace(detail); normalized != "" {
		message += ": " + normalized
	}
	return &contractError{
		message: message,
		causes:  []error{ErrInvalidConfig},
	}
}

func NewInvalidHeaderNameError(header string) error {
	message := "observability: invalid propagation header name"
	if normalized := strings.TrimSpace(header); normalized != "" {
		message += ": " + normalized
	}
	return &contractError{
		message: message,
		causes:  []error{ErrInvalidHeaderName},
	}
}

func NewCardinalityViolationError(label string) error {
	message := "observability: cardinality violation"
	if normalized := strings.TrimSpace(label); normalized != "" {
		message += ": " + normalized + " is not allowed"
	}
	return &contractError{
		message: message,
		causes:  []error{ErrCardinalityViolation},
	}
}

func NewShutdownError(detail string, err error) error {
	message := "observability: shutdown failed"
	if normalized := strings.TrimSpace(detail); normalized != "" {
		message += ": " + normalized
	}
	causes := []error{ErrShutdownFailed}
	if err != nil {
		message += ": " + err.Error()
		causes = append(causes, err)
	}
	return &contractError{
		message: message,
		causes:  causes,
	}
}

func NewBenchmarkRegressionError(name string, regressionPercent float64) error {
	message := "observability: benchmark regression"
	if normalized := strings.TrimSpace(name); normalized != "" {
		message += ": " + normalized + " exceeded " + fmt.Sprintf("%.2f%%", regressionPercent)
	}
	return &contractError{
		message: message,
		causes:  []error{ErrBenchmarkRegression},
	}
}

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

type CardinalityValidator struct {
	mu            sync.RWMutex
	blockedLabels map[string]bool
	enabled       bool
}

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

func NewCardinalityValidatorWithCustomLabels(enabled bool, customBlockedLabels []string) *CardinalityValidator {
	blockedMap := make(map[string]bool, len(HighCardinalityLabels)+len(customBlockedLabels))

	for _, label := range HighCardinalityLabels {
		blockedMap[label] = true
	}
	for _, label := range customBlockedLabels {
		blockedMap[strings.ToLower(label)] = true
	}

	return &CardinalityValidator{
		blockedLabels: blockedMap,
		enabled:       enabled,
	}
}

func (v *CardinalityValidator) Validate(fields []Field) error {
	if !v.enabled {
		return nil
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, field := range fields {
		for blocked := range v.blockedLabels {
			if strings.EqualFold(field.Key, blocked) {
				return NewCardinalityViolationError(field.Key)
			}
		}
	}

	return nil
}

func (v *CardinalityValidator) AddBlockedLabel(label string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.blockedLabels[strings.ToLower(label)] = true
}

func (v *CardinalityValidator) RemoveBlockedLabel(label string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.blockedLabels, strings.ToLower(label))
}

func (v *CardinalityValidator) IsBlocked(label string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	for blocked := range v.blockedLabels {
		if strings.EqualFold(label, blocked) {
			return true
		}
	}
	return false
}

func normalizeHeaderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isValidHeaderName(name string) bool {
	if name == "" {
		return false
	}

	for i := range len(name) {
		c := name[i]
		if isAlphaNumeric(c) {
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}

	return true
}

func isAlphaNumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
