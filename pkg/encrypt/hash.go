package encrypt

import "golang.org/x/crypto/bcrypt"

type (
	HashAdapter interface {
		GenerateHash(str string) (string, error)
		CheckHash(hash, str string) bool
	}

	hashAdapter struct {
	}
)

func NewHashAdapter() HashAdapter {
	return &hashAdapter{}
}

func (h *hashAdapter) GenerateHash(str string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(str), 5)
	return string(bytes), err
}

func (h *hashAdapter) CheckHash(hash, str string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(str))
	return err == nil
}
