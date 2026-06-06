package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/worker/consumer"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	startErr error
	stopErr  error
	started  bool
	stopped  bool
}

func (m *mockRunner) Start(_ context.Context) error {
	m.started = true
	return m.startErr
}

func (m *mockRunner) Stop(_ context.Context) error {
	m.stopped = true
	return m.stopErr
}

func TestAdapter_NameAndTechnology(t *testing.T) {
	a := consumer.NewAdapter("my-consumer", "kafka", &mockRunner{})
	require.Equal(t, "my-consumer", a.Name())
	require.Equal(t, "kafka", a.Technology())
}

func TestAdapter_DelegatesStart(t *testing.T) {
	r := &mockRunner{}
	a := consumer.NewAdapter("test", "kafka", r)
	require.NoError(t, a.Start(context.Background()))
	require.True(t, r.started)
}

func TestAdapter_DelegatesStop(t *testing.T) {
	expected := errors.New("stop error")
	r := &mockRunner{stopErr: expected}
	a := consumer.NewAdapter("test", "kafka", r)
	require.ErrorIs(t, a.Stop(context.Background()), expected)
}
