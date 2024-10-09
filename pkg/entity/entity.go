package entity

import (
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/vos"
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
