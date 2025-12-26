package encrypt

import "golang.org/x/crypto/bcrypt"

const (
	// DefaultBcryptCost é o custo padrão do bcrypt para ambientes de produção.
	// Recomendado: 10-14 (10 é um bom equilíbrio entre segurança e performance).
	// Custo 10 = ~100ms em hardware moderno
	// Custo 12 = ~400ms em hardware moderno
	// Custo 14 = ~1.6s em hardware moderno
	DefaultBcryptCost = 10

	// MinBcryptCost é o custo mínimo aceitável (apenas para testes).
	MinBcryptCost = bcrypt.MinCost // 4

	// MaxBcryptCost é o custo máximo do bcrypt.
	MaxBcryptCost = bcrypt.MaxCost // 31
)

// HashAdapter define a interface para operações de hash de senhas.
// Implementações devem usar algoritmos criptograficamente seguros.
type HashAdapter interface {
	// GenerateHash gera um hash seguro da string fornecida.
	// Retorna o hash ou erro se a operação falhar.
	GenerateHash(str string) (string, error)

	// CheckHash verifica se a string corresponde ao hash.
	// Retorna true se a string corresponder ao hash, false caso contrário.
	CheckHash(hash, str string) bool
}

// hashAdapter implementa HashAdapter usando bcrypt.
type hashAdapter struct {
	cost int
}

// NewHashAdapter cria um novo HashAdapter com custo padrão (10).
// Para produção, use cost 10-14 dependendo dos requisitos de segurança.
func NewHashAdapter() HashAdapter {
	return &hashAdapter{
		cost: DefaultBcryptCost,
	}
}

// NewHashAdapterWithCost cria um novo HashAdapter com custo customizado.
// Use apenas para casos especiais (ex: testes com MinCost, ou alta segurança com cost 12+).
//
// Recomendações:
//   - Desenvolvimento/Testes: MinCost (4) para velocidade
//   - Produção padrão: 10 (~100ms)
//   - Alta segurança: 12-14 (~400ms-1.6s)
func NewHashAdapterWithCost(cost int) HashAdapter {
	// Garante que o cost está dentro dos limites do bcrypt
	if cost < MinBcryptCost {
		cost = MinBcryptCost
	}
	if cost > MaxBcryptCost {
		cost = MaxBcryptCost
	}

	return &hashAdapter{
		cost: cost,
	}
}

// GenerateHash gera um hash bcrypt da string com o custo configurado.
// O hash resultante inclui o salt e pode ser armazenado diretamente no banco de dados.
func (h *hashAdapter) GenerateHash(str string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(str), h.cost)
	return string(bytes), err
}

// CheckHash verifica se a string corresponde ao hash bcrypt.
// É seguro contra timing attacks pois bcrypt.CompareHashAndPassword usa constant-time comparison.
func (h *hashAdapter) CheckHash(hash, str string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(str))
	return err == nil
}
