package worker

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/worker/job"
)

type Job = job.Runner

type Consumer interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
