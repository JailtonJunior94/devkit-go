package consumer

import "context"

type Adapter struct {
	name       string
	technology string
	runner     Runner
}

func NewAdapter(name, technology string, runner Runner) *Adapter {
	return &Adapter{
		name:       name,
		technology: technology,
		runner:     runner,
	}
}

func (a *Adapter) Name() string                    { return a.name }
func (a *Adapter) Technology() string              { return a.technology }
func (a *Adapter) Start(ctx context.Context) error { return a.runner.Start(ctx) }
func (a *Adapter) Stop(ctx context.Context) error  { return a.runner.Stop(ctx) }
