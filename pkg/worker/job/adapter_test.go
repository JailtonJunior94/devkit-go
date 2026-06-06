package job_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/worker/job"
	"github.com/stretchr/testify/require"
)

func TestNewAdapter_DefaultPolicyIsSkip(t *testing.T) {
	a := job.NewAdapter("test", "* * * * *", func(ctx context.Context) error { return nil })
	require.Equal(t, job.OverlapSkip, a.OverlapPolicy())
}

func TestNewAdapterWithPolicy_AllowPolicy(t *testing.T) {
	a := job.NewAdapterWithPolicy("test", "* * * * *", func(ctx context.Context) error { return nil }, job.OverlapAllow)
	require.Equal(t, job.OverlapAllow, a.OverlapPolicy())
}

func TestAdapter_RunDelegates(t *testing.T) {
	expected := errors.New("run error")
	a := job.NewAdapter("test", "* * * * *", func(ctx context.Context) error { return expected })
	require.ErrorIs(t, a.Run(context.Background()), expected)
}

func TestAdapter_Name(t *testing.T) {
	a := job.NewAdapter("my-job", "* * * * *", func(ctx context.Context) error { return nil })
	require.Equal(t, "my-job", a.Name())
}

func TestAdapter_Schedule(t *testing.T) {
	a := job.NewAdapter("test", "0 * * * *", func(ctx context.Context) error { return nil })
	require.Equal(t, "0 * * * *", a.Schedule())
}
