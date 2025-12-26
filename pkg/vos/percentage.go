package vos

import (
	"errors"
	"fmt"
)

var (
	// ErrDivisionByZeroPercentage é retornado quando se tenta dividir por zero.
	ErrDivisionByZeroPercentage = errors.New("division by zero")
)

// Percentage representa um valor percentual com precisão de 4 casas decimais.
// Internamente armazena o valor em basis points (1 bp = 0.0001% = 1/10000) para evitar
// problemas de precisão de ponto flutuante. É seguro para cálculos precisos.
//
// Exemplo:
//
//	p := NewPercentage(10.5)  // 10.50%
//	p2 := NewPercentageFromBasisPoints(105000)  // 10.5000% (equivalente)
type Percentage struct {
	// basisPoints armazena o valor em basis points (1/10000) para garantir precisão
	// 10000 basis points = 1%
	basisPoints int64
}

// NewPercentage cria um Percentage a partir de um valor percentual (float64).
// O valor é convertido para basis points internamente para garantir precisão.
//
// Exemplo: NewPercentage(10.5) representa 10.50%
func NewPercentage(value float64) Percentage {
	return Percentage{basisPoints: int64(value * 10000)}
}

// NewPercentageFromBasisPoints cria um Percentage a partir de basis points.
// Recomendado quando a precisão é crítica.
// 1 basis point = 0.0001%
//
// Exemplo: NewPercentageFromBasisPoints(105000) representa 10.5000%
func NewPercentageFromBasisPoints(basisPoints int64) Percentage {
	return Percentage{basisPoints: basisPoints}
}

// Add adiciona dois valores percentuais.
func (p Percentage) Add(other Percentage) Percentage {
	return Percentage{basisPoints: p.basisPoints + other.basisPoints}
}

// Sub subtrai um valor percentual de outro.
func (p Percentage) Sub(other Percentage) Percentage {
	return Percentage{basisPoints: p.basisPoints - other.basisPoints}
}

// Mul multiplica o valor percentual por um fator.
// Note que o resultado é arredondado para o basis point mais próximo.
func (p Percentage) Mul(factor float64) Percentage {
	return Percentage{basisPoints: int64(float64(p.basisPoints) * factor)}
}

// Div divide o valor percentual por um divisor.
// Retorna erro se o divisor for zero.
// Note que o resultado é arredondado para o basis point mais próximo.
func (p Percentage) Div(divisor float64) (Percentage, error) {
	if divisor == 0 {
		return Percentage{}, ErrDivisionByZeroPercentage
	}
	return Percentage{basisPoints: int64(float64(p.basisPoints) / divisor)}, nil
}

// String retorna a representação em string formatada do valor percentual.
// Formato: 10.50%
func (p Percentage) String() string {
	return fmt.Sprintf("%.4f%%", p.Float())
}

// Equals verifica se dois valores percentuais são iguais.
// Usa comparação exata de inteiros, não há problemas de precisão de float.
func (p Percentage) Equals(other Percentage) bool {
	return p.basisPoints == other.basisPoints
}

// LessThan verifica se este valor é menor que outro.
func (p Percentage) LessThan(other Percentage) bool {
	return p.basisPoints < other.basisPoints
}

// GreaterThan verifica se este valor é maior que outro.
func (p Percentage) GreaterThan(other Percentage) bool {
	return p.basisPoints > other.basisPoints
}

// LessThanOrEqual verifica se este valor é menor ou igual a outro.
func (p Percentage) LessThanOrEqual(other Percentage) bool {
	return p.basisPoints <= other.basisPoints
}

// GreaterThanOrEqual verifica se este valor é maior ou igual a outro.
func (p Percentage) GreaterThanOrEqual(other Percentage) bool {
	return p.basisPoints >= other.basisPoints
}

// Float retorna o valor percentual como float64.
// Use apenas para exibição, não para cálculos.
func (p Percentage) Float() float64 {
	return float64(p.basisPoints) / 10000.0
}

// BasisPoints retorna o valor em basis points.
// Útil para armazenamento em banco de dados.
// 1 basis point = 0.0001%
func (p Percentage) BasisPoints() int64 {
	return p.basisPoints
}

// IsZero verifica se o valor é zero.
func (p Percentage) IsZero() bool {
	return p.basisPoints == 0
}

// IsNegative verifica se o valor é negativo.
func (p Percentage) IsNegative() bool {
	return p.basisPoints < 0
}

// IsPositive verifica se o valor é positivo.
func (p Percentage) IsPositive() bool {
	return p.basisPoints > 0
}

// Abs retorna o valor absoluto.
func (p Percentage) Abs() Percentage {
	if p.basisPoints < 0 {
		return Percentage{basisPoints: -p.basisPoints}
	}
	return p
}

// Negate retorna o valor negado.
func (p Percentage) Negate() Percentage {
	return Percentage{basisPoints: -p.basisPoints}
}

// Apply aplica a porcentagem a um valor.
// Exemplo: Percentage(10%).Apply(100) = 10
func (p Percentage) Apply(value float64) float64 {
	return value * p.Float() / 100.0
}

// ApplyToMoney aplica a porcentagem a um Money e retorna um Money.
// Exemplo: Percentage(10%).ApplyToMoney(Money(100)) = Money(10)
func (p Percentage) ApplyToMoney(m Money) Money {
	cents := int64(float64(m.Cents()) * p.Float() / 100.0)
	return NewMoneyFromCents(cents)
}

// MarshalJSON implementa json.Marshaler.
// Serializa como float64 (ex: 10.50 para 10.50%)
func (p Percentage) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.4f", p.Float())), nil
}

// UnmarshalJSON implementa json.Unmarshaler.
// Aceita float64 (ex: 10.50 para 10.50%)
func (p *Percentage) UnmarshalJSON(data []byte) error {
	var value float64
	if _, err := fmt.Sscanf(string(data), "%f", &value); err != nil {
		return err
	}
	p.basisPoints = int64(value * 10000)
	return nil
}
