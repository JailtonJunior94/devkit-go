package entity

import (
	"time"

	"github.com/jailtonjunior94/financial/pkg/vos"
)

type Base struct {
	ID        vos.UUID
	CreatedAt time.Time
	UpdatedAt vos.NullableTime
	DeletedAt vos.NullableTime
}

func (b *Base) SetID(id vos.UUID) {
	b.ID = id
}
