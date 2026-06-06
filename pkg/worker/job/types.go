package job

import "context"

type OverlapPolicy int

const (
	OverlapSkip OverlapPolicy = iota + 1
	OverlapAllow
)

type Runner interface {
	Name() string
	Schedule() string
	Run(ctx context.Context) error
	OverlapPolicy() OverlapPolicy
}
