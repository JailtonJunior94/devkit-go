package uow

import "github.com/JailtonJunior94/devkit-go/pkg/database/manager"

// NewVoid cria um UnitOfWork[struct{}] para fluxos transacionais que não produzem
// um valor de retorno tipado. Equivalente a New[struct{}](mgr, opts...).
func NewVoid(mgr manager.Manager, opts ...Option) UnitOfWork[struct{}] {
	return New[struct{}](mgr, opts...)
}
