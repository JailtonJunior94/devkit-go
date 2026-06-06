package job

import "context"

type Adapter struct {
	name          string
	schedule      string
	run           func(ctx context.Context) error
	overlapPolicy OverlapPolicy
}

func NewAdapter(name, schedule string, run func(ctx context.Context) error) *Adapter {
	return &Adapter{
		name:          name,
		schedule:      schedule,
		run:           run,
		overlapPolicy: OverlapSkip,
	}
}

func NewAdapterWithPolicy(name, schedule string, run func(ctx context.Context) error, policy OverlapPolicy) *Adapter {
	return &Adapter{
		name:          name,
		schedule:      schedule,
		run:           run,
		overlapPolicy: policy,
	}
}

func (a *Adapter) Name() string                  { return a.name }
func (a *Adapter) Schedule() string              { return a.schedule }
func (a *Adapter) Run(ctx context.Context) error { return a.run(ctx) }
func (a *Adapter) OverlapPolicy() OverlapPolicy  { return a.overlapPolicy }
