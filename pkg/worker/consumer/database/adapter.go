package database

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/worker/consumer"
)

const technology = "database"

type Adapter struct {
	name   string
	runner consumer.Runner
}

func NewAdapter(name string, runner consumer.Runner) *Adapter {
	return &Adapter{
		name:   name,
		runner: runner,
	}
}

func (a *Adapter) Name() string                    { return a.name }
func (a *Adapter) Technology() string              { return technology }
func (a *Adapter) Start(ctx context.Context) error { return a.runner.Start(ctx) }
func (a *Adapter) Stop(ctx context.Context) error  { return a.runner.Stop(ctx) }
