package vos

import (
	"errors"

	"golang.org/x/text/currency"
	"golang.org/x/text/message"
)

type Money struct {
	value float64
}

func NewMoney(value float64) Money {
	return Money{value: value}
}

func (m Money) Add(other Money) Money {
	return Money{value: m.value + other.value}
}

func (m Money) Sub(other Money) Money {
	return Money{value: m.value - other.value}
}

func (m Money) Mul(factor float64) Money {
	return Money{value: m.value * factor}
}

func (m Money) Div(divisor float64) (Money, error) {
	if divisor == 0 {
		return Money{}, errors.New("division by zero")
	}
	return Money{value: m.value / divisor}, nil
}

func (m Money) String() string {
	p := message.NewPrinter(message.MatchLanguage("pt-BR"))
	return p.Sprintf("%s%.2f", currency.Symbol(currency.BRL), m.value)
}

func (m Money) Equals(other Money) bool {
	return m.value == other.value
}

func (m Money) LessThan(other Money) bool {
	return m.value < other.value
}

func (m Money) GreaterThan(other Money) bool {
	return m.value > other.value
}

func (m Money) Money() float64 {
	return m.value
}
