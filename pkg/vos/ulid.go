package vos

import (
	"errors"
	"math/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	ErrInvalidULID = errors.New("invalid ULID")
)

type ULID struct {
	Value ulid.ULID
}

func NewULID() (ULID, error) {
	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	id, err := ulid.New(ulid.Timestamp(time.Now()), entropy)
	if err != nil {
		return ULID{}, err
	}

	vo := ULID{
		Value: id,
	}

	if err := vo.Validate(); err != nil {
		return ULID{}, err
	}
	return vo, nil
}

func NewULIDFromString(value string) (ULID, error) {
	ulidValue, err := ulid.Parse(value)
	if err != nil {
		return ULID{}, err
	}

	vo := ULID{
		Value: ulidValue,
	}

	if err := vo.Validate(); err != nil {
		return ULID{}, err
	}
	return vo, nil
}

func (u ULID) Validate() error {
	if u.Value.Compare(ulid.ULID{}) == 0 {
		return ErrInvalidULID
	}
	return nil
}

func (u ULID) String() string {
	return u.Value.String()
}
