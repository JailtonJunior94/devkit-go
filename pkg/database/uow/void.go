package uow

import "github.com/JailtonJunior94/devkit-go/pkg/database/manager"

func NewVoid(mgr manager.Manager, opts ...Option) UnitOfWork[struct{}] {
	return New[struct{}](mgr, opts...)
}
