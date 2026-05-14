package otel

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

func TestNewProviderPreservesPublicSignature(t *testing.T) {
	t.Parallel()

	var newProvider func(context.Context, *Config, ...Option) (*Provider, error) = NewProvider
	var _ observability.Observability = (*Provider)(nil)
	if newProvider == nil {
		t.Fatal("NewProvider signature must remain available")
	}
}
