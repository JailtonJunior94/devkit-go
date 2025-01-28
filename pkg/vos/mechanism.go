package vos

type Mechanism string

const (
	Plain     Mechanism = "PLAIN"
	Scram     Mechanism = "SCRAM"
	PlainText Mechanism = "PLAINTEXT"
)

func (c Mechanism) String() string {
	return string(c)
}
