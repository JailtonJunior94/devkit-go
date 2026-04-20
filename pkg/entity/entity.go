package entity

import (
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/vos"
)

// Base provides identity and audit timestamps shared by all domain entities.
// Embed this struct in aggregate roots and entities.
type Base struct {
	ID        vos.UUID
	CreatedAt time.Time
	UpdatedAt vos.NullableTime
	DeletedAt vos.NullableTime
}

// SetID replaces the entity's identifier.
func (b *Base) SetID(id vos.UUID) {
	b.ID = id
}
