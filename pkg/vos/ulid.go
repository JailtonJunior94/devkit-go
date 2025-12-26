package vos

import (
	"crypto/rand"
	"errors"

	"github.com/oklog/ulid/v2"
)

var (
	// ErrInvalidULID é retornado quando um ULID é inválido (zero value).
	ErrInvalidULID = errors.New("invalid ULID")
)

// ULID representa um Universally Unique Lexicographically Sortable Identifier.
// É thread-safe e pode ser usado em ambientes concorrentes.
type ULID struct {
	Value ulid.ULID
}

// NewULID cria um novo ULID usando crypto/rand como fonte de entropia.
// É thread-safe e adequado para uso em ambientes concorrentes com múltiplos pods.
// Retorna erro se falhar ao gerar o ULID ou se a validação falhar.
func NewULID() (ULID, error) {
	// Usa crypto/rand que é thread-safe ao invés de math/rand
	id, err := ulid.New(ulid.Now(), rand.Reader)
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

// NewULIDFromString cria um ULID a partir de uma string.
// Retorna erro se a string não for um ULID válido.
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

// Validate verifica se o ULID é válido (não é zero value).
func (u ULID) Validate() error {
	if u.Value.Compare(ulid.ULID{}) == 0 {
		return ErrInvalidULID
	}
	return nil
}

// String retorna a representação em string do ULID.
func (u ULID) String() string {
	return u.Value.String()
}
