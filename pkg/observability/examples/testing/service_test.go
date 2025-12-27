package testing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
)

// UserService is the service we want to test.
type UserService struct {
	obs observability.Observability
}

func NewUserService(obs observability.Observability) *UserService {
	return &UserService{obs: obs}
}

type User struct {
	ID    string
	Name  string
	Email string
}

// CreateUser demonstrates a service method with full observability.
func (s *UserService) CreateUser(ctx context.Context, name, email string) (*User, error) {
	// Start tracing
	ctx, span := s.obs.Tracer().Start(ctx, "UserService.CreateUser",
		observability.WithAttributes(
			observability.String("name", name),
			observability.String("email", email),
		),
	)
	defer span.End()

	// Log operation
	s.obs.Logger().Info(ctx, "creating user",
		observability.String("name", name),
		observability.String("email", email),
	)

	// Validate input
	if name == "" {
		err := errors.New("name is required")
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, "validation failed")

		s.obs.Logger().Error(ctx, "validation failed",
			observability.Error(err),
		)

		// Increment error counter
		errorCounter := s.obs.Metrics().Counter(
			"user.create.errors",
			"Total user creation errors",
			"1",
		)
		errorCounter.Increment(ctx,
			observability.String("error_type", "validation"),
		)

		return nil, err
	}

	// Create user
	user := &User{
		ID:    "user-123",
		Name:  name,
		Email: email,
	}

	span.SetStatus(observability.StatusCodeOK, "user created")
	span.AddEvent("user_created",
		observability.String("user_id", user.ID),
	)

	s.obs.Logger().Info(ctx, "user created successfully",
		observability.String("user_id", user.ID),
	)

	// Increment success counter
	successCounter := s.obs.Metrics().Counter(
		"user.create.success",
		"Total users created successfully",
		"1",
	)
	successCounter.Increment(ctx)

	return user, nil
}

// TestUserServiceCreateUser_Success demonstrates testing with fake provider.
func TestUserServiceCreateUser_Success(t *testing.T) {
	// Setup fake observability provider
	fakeObs := fake.NewProvider()

	// Create service with fake provider
	service := NewUserService(fakeObs)

	// Execute operation
	ctx := context.Background()
	user, err := service.CreateUser(ctx, "John Doe", "john@example.com")

	// Assert no error
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Assert user created
	if user == nil {
		t.Fatal("expected user to be created")
	}

	if user.Name != "John Doe" {
		t.Errorf("expected name %q, got %q", "John Doe", user.Name)
	}

	// Assert logs were captured
	fakeLogger := fakeObs.Logger().(*fake.FakeLogger)
	logEntries := fakeLogger.GetEntries()

	if len(logEntries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(logEntries))
	}

	// Verify first log (creating user)
	if logEntries[0].Message != "creating user" {
		t.Errorf("expected first log message %q, got %q", "creating user", logEntries[0].Message)
	}

	// Verify second log (user created)
	if logEntries[1].Message != "user created successfully" {
		t.Errorf("expected second log message %q, got %q", "user created successfully", logEntries[1].Message)
	}

	// Assert metrics were recorded
	fakeMetrics := fakeObs.Metrics().(*fake.FakeMetrics)

	successCounter := fakeMetrics.GetCounter("user.create.success")
	if successCounter == nil {
		t.Fatal("expected success counter to be created")
	}

	values := successCounter.GetValues()
	if len(values) != 1 {
		t.Fatalf("expected 1 counter value, got %d", len(values))
	}

	if values[0].Value != 1 {
		t.Errorf("expected counter value 1, got %d", values[0].Value)
	}

	// Assert spans were captured
	fakeTracer := fakeObs.Tracer().(*fake.FakeTracer)
	spans := fakeTracer.GetSpans()

	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "UserService.CreateUser" {
		t.Errorf("expected span name %q, got %q", "UserService.CreateUser", span.Name)
	}

	if span.Status != observability.StatusCodeOK {
		t.Errorf("expected span status OK, got %v", span.Status)
	}

	// Verify span has correct attributes
	hasNameAttr := false
	hasEmailAttr := false
	for _, attr := range span.Attributes {
		if attr.Key == "name" && attr.Value == "John Doe" {
			hasNameAttr = true
		}
		if attr.Key == "email" && attr.Value == "john@example.com" {
			hasEmailAttr = true
		}
	}

	if !hasNameAttr {
		t.Error("expected span to have 'name' attribute")
	}

	if !hasEmailAttr {
		t.Error("expected span to have 'email' attribute")
	}

	// Verify span events
	if len(span.Events) != 1 {
		t.Fatalf("expected 1 span event, got %d", len(span.Events))
	}

	event := span.Events[0]
	if event.Name != "user_created" {
		t.Errorf("expected event name %q, got %q", "user_created", event.Name)
	}
}

// TestUserServiceCreateUser_ValidationError demonstrates testing error cases.
func TestUserServiceCreateUser_ValidationError(t *testing.T) {
	// Setup fake observability provider
	fakeObs := fake.NewProvider()
	service := NewUserService(fakeObs)

	// Execute operation with invalid input
	ctx := context.Background()
	user, err := service.CreateUser(ctx, "", "john@example.com")

	// Assert error occurred
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if user != nil {
		t.Errorf("expected nil user, got %+v", user)
	}

	// Assert error log was captured
	fakeLogger := fakeObs.Logger().(*fake.FakeLogger)
	logEntries := fakeLogger.GetEntries()

	// Find error log entry
	var errorLogFound bool
	for _, entry := range logEntries {
		if entry.Level == observability.LogLevelError {
			errorLogFound = true

			if entry.Message != "validation failed" {
				t.Errorf("expected error log message %q, got %q", "validation failed", entry.Message)
			}

			// Verify error field is present
			var hasErrorField bool
			for _, field := range entry.Fields {
				if field.Key == "error" {
					hasErrorField = true
					break
				}
			}

			if !hasErrorField {
				t.Error("expected error log to have 'error' field")
			}
		}
	}

	if !errorLogFound {
		t.Error("expected to find error log entry")
	}

	// Assert error metric was recorded
	fakeMetrics := fakeObs.Metrics().(*fake.FakeMetrics)

	errorCounter := fakeMetrics.GetCounter("user.create.errors")
	if errorCounter == nil {
		t.Fatal("expected error counter to be created")
	}

	values := errorCounter.GetValues()
	if len(values) != 1 {
		t.Fatalf("expected 1 counter value, got %d", len(values))
	}

	// Verify error counter has correct fields
	var hasErrorTypeField bool
	for _, field := range values[0].Fields {
		if field.Key == "error_type" && field.Value == "validation" {
			hasErrorTypeField = true
			break
		}
	}

	if !hasErrorTypeField {
		t.Error("expected error counter to have 'error_type' field with value 'validation'")
	}

	// Assert span recorded error
	fakeTracer := fakeObs.Tracer().(*fake.FakeTracer)
	spans := fakeTracer.GetSpans()

	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Status != observability.StatusCodeError {
		t.Errorf("expected span status ERROR, got %v", span.Status)
	}

	if span.RecordedErr == nil {
		t.Fatal("expected error to be recorded in span")
	}

	if span.RecordedErr.Error() != "name is required" {
		t.Errorf("expected error message %q, got %q", "name is required", span.RecordedErr.Error())
	}
}

// TestMultipleOperations demonstrates testing multiple operations and resetting fake provider.
func TestMultipleOperations(t *testing.T) {
	fakeObs := fake.NewProvider()
	service := NewUserService(fakeObs)
	ctx := context.Background()

	// First operation
	t.Run("first operation", func(t *testing.T) {
		_, err := service.CreateUser(ctx, "Alice", "alice@example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		fakeTracer := fakeObs.Tracer().(*fake.FakeTracer)
		spans := fakeTracer.GetSpans()
		if len(spans) != 1 {
			t.Errorf("expected 1 span, got %d", len(spans))
		}
	})

	// Reset fake provider
	fakeTracer := fakeObs.Tracer().(*fake.FakeTracer)
	fakeLogger := fakeObs.Logger().(*fake.FakeLogger)

	fakeTracer.Reset()
	fakeLogger.Reset()

	// Second operation (after reset)
	t.Run("second operation after reset", func(t *testing.T) {
		_, err := service.CreateUser(ctx, "Bob", "bob@example.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		spans := fakeTracer.GetSpans()
		if len(spans) != 1 {
			t.Errorf("expected 1 span after reset, got %d", len(spans))
		}

		logEntries := fakeLogger.GetEntries()
		if len(logEntries) != 2 {
			t.Errorf("expected 2 log entries after reset, got %d", len(logEntries))
		}
	})
}

// BenchmarkUserServiceCreateUser demonstrates benchmarking with noop provider.
func BenchmarkUserServiceCreateUser(b *testing.B) {
	// Use noop provider for benchmarks to avoid overhead
	// In real scenarios, you would import "github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	fakeObs := fake.NewProvider() // Using fake for this example
	service := NewUserService(fakeObs)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.CreateUser(ctx, "John Doe", "john@example.com")
	}
}
