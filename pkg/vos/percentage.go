package vos

import (
	"errors"
	"fmt"
)

type Percentage struct {
	value float64
}

func NewPercentage(value float64) Percentage {
	return Percentage{value: value}
}

func (p Percentage) Add(other Percentage) Percentage {
	return Percentage{value: p.value + other.value}
}

func (p Percentage) Sub(other Percentage) Percentage {
	return Percentage{value: p.value - other.value}
}

func (p Percentage) Mul(factor float64) Percentage {
	return Percentage{value: p.value * factor}
}

func (p Percentage) Div(divisor float64) (Percentage, error) {
	if divisor == 0 {
		return Percentage{}, errors.New("division by zero")
	}
	return Percentage{value: p.value / divisor}, nil
}

func (p Percentage) String() string {
	return fmt.Sprintf("%.2f%%", p.value)
}

func (p Percentage) Equals(other Percentage) bool {
	return p.value == other.value
}

func (p Percentage) LessThan(other Percentage) bool {
	return p.value < other.value
}

func (p Percentage) GreaterThan(other Percentage) bool {
	return p.value > other.value
}

func (p Percentage) Percentage() float64 {
	return p.value
}
