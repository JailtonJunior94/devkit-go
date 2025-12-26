package vos

import (
	"errors"
	"fmt"

	"golang.org/x/text/currency"
	"golang.org/x/text/message"
)

var (
	// ErrDivisionByZero é retornado quando se tenta dividir por zero.
	ErrDivisionByZero = errors.New("division by zero")
)

// Money representa um valor monetário com precisão de centavos.
// Internamente armazena o valor em centavos (int64) para evitar problemas
// de precisão de ponto flutuante. É seguro para cálculos financeiros.
//
// Exemplo:
//
//	m := NewMoney(10.50)  // R$ 10,50
//	m2 := NewMoneyFromCents(1050)  // R$ 10,50 (equivalente)
type Money struct {
	// cents armazena o valor em centavos para garantir precisão
	cents int64
}

// NewMoney cria um Money a partir de um valor em reais (float64).
// O valor é convertido para centavos internamente.
//
// Exemplo: NewMoney(10.50) representa R$ 10,50
func NewMoney(value float64) Money {
	return Money{cents: int64(value * 100)}
}

// NewMoneyFromCents cria um Money a partir de um valor em centavos.
// Recomendado quando a precisão é crítica.
//
// Exemplo: NewMoneyFromCents(1050) representa R$ 10,50
func NewMoneyFromCents(cents int64) Money {
	return Money{cents: cents}
}

// Add adiciona dois valores monetários.
func (m Money) Add(other Money) Money {
	return Money{cents: m.cents + other.cents}
}

// Sub subtrai um valor monetário de outro.
func (m Money) Sub(other Money) Money {
	return Money{cents: m.cents - other.cents}
}

// Mul multiplica o valor monetário por um fator.
// Note que o resultado é arredondado para o centavo mais próximo.
func (m Money) Mul(factor float64) Money {
	return Money{cents: int64(float64(m.cents) * factor)}
}

// Div divide o valor monetário por um divisor.
// Retorna erro se o divisor for zero.
// Note que o resultado é arredondado para o centavo mais próximo.
func (m Money) Div(divisor float64) (Money, error) {
	if divisor == 0 {
		return Money{}, ErrDivisionByZero
	}
	return Money{cents: int64(float64(m.cents) / divisor)}, nil
}

// String retorna a representação em string formatada do valor monetário.
// Formato: R$ 10,50
func (m Money) String() string {
	p := message.NewPrinter(message.MatchLanguage("pt-BR"))
	return p.Sprintf("%s%.2f", currency.Symbol(currency.BRL), m.Float())
}

// Equals verifica se dois valores monetários são iguais.
// Usa comparação exata de inteiros, não há problemas de precisão de float.
func (m Money) Equals(other Money) bool {
	return m.cents == other.cents
}

// LessThan verifica se este valor é menor que outro.
func (m Money) LessThan(other Money) bool {
	return m.cents < other.cents
}

// GreaterThan verifica se este valor é maior que outro.
func (m Money) GreaterThan(other Money) bool {
	return m.cents > other.cents
}

// LessThanOrEqual verifica se este valor é menor ou igual a outro.
func (m Money) LessThanOrEqual(other Money) bool {
	return m.cents <= other.cents
}

// GreaterThanOrEqual verifica se este valor é maior ou igual a outro.
func (m Money) GreaterThanOrEqual(other Money) bool {
	return m.cents >= other.cents
}

// Float retorna o valor em reais como float64.
// Use apenas para exibição, não para cálculos.
func (m Money) Float() float64 {
	return float64(m.cents) / 100.0
}

// Cents retorna o valor em centavos.
// Útil para armazenamento em banco de dados.
func (m Money) Cents() int64 {
	return m.cents
}

// IsZero verifica se o valor é zero.
func (m Money) IsZero() bool {
	return m.cents == 0
}

// IsNegative verifica se o valor é negativo.
func (m Money) IsNegative() bool {
	return m.cents < 0
}

// IsPositive verifica se o valor é positivo.
func (m Money) IsPositive() bool {
	return m.cents > 0
}

// Abs retorna o valor absoluto.
func (m Money) Abs() Money {
	if m.cents < 0 {
		return Money{cents: -m.cents}
	}
	return m
}

// Negate retorna o valor negado.
func (m Money) Negate() Money {
	return Money{cents: -m.cents}
}

// MarshalJSON implementa json.Marshaler.
// Serializa como float64 em reais (ex: 10.50)
func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.2f", m.Float())), nil
}

// UnmarshalJSON implementa json.Unmarshaler.
// Aceita float64 em reais (ex: 10.50)
func (m *Money) UnmarshalJSON(data []byte) error {
	var value float64
	if _, err := fmt.Sscanf(string(data), "%f", &value); err != nil {
		return err
	}
	m.cents = int64(value * 100)
	return nil
}
