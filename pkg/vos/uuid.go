package vos

import (
	"errors"

	"github.com/google/uuid"
)

var (
	ErrInvalidUUID = errors.New("invalid UUID")
)

type UUID struct {
	Value uuid.UUID
}

func NewUUID() (UUID, error) {
	uuid, err := uuid.NewV7()
	if err != nil {
		return UUID{}, err
	}

	vo := UUID{
		Value: uuid,
	}

	if err := vo.Validate(); err != nil {
		return UUID{}, err
	}
	return vo, nil
}

func NewUUIDFromString(value string) (UUID, error) {
	uuidValue, err := uuid.Parse(value)
	if err != nil {
		return UUID{}, err
	}

	vo := UUID{
		Value: uuidValue,
	}

	if err := vo.Validate(); err != nil {
		return UUID{}, err
	}
	return vo, nil
}

func NewFromUUID(value uuid.UUID) (UUID, error) {
	vo := UUID{
		Value: value,
	}

	if err := vo.Validate(); err != nil {
		return UUID{}, err
	}
	return vo, nil
}

func (vo *UUID) Validate() error {
	if vo.Value == uuid.Nil {
		return ErrInvalidUUID
	}
	return nil
}

func (vo *UUID) String() string {
	return vo.Value.String()
}

func (vo *UUID) IsEmpty() bool {
	return vo.Value == uuid.Nil
}

func (vo *UUID) UUID() uuid.UUID {
	return vo.Value
}

func (vo *UUID) SafeUUID() *uuid.UUID {
	if vo == nil {
		return nil
	}

	if vo.IsEmpty() {
		return nil
	}

	if vo.Value == uuid.Nil {
		return nil
	}
	return &vo.Value
}
