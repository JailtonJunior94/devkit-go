package httpclient

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MakeRequestSuite struct {
	suite.Suite

	ctx context.Context
}

type Address struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Unidade     string `json:"unidade"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	Uf          string `json:"uf"`
	Estado      string `json:"estado"`
	Regiao      string `json:"regiao"`
	Ibge        string `json:"ibge"`
	Gia         string `json:"gia"`
	Ddd         string `json:"ddd"`
	Siafi       string `json:"siafi"`
}

func TestMakeRequestSuite(t *testing.T) {
	suite.Run(t, new(MakeRequestSuite))
}

func (s *MakeRequestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *MakeRequestSuite) TestMakeRequest() {
	type (
		args struct {
			zipCode string
		}
	)

	scenarios := []struct {
		name     string
		args     args
		expected func(address *Address, err error)
	}{{
		name: "should return an error when the request fails",
		args: args{zipCode: "06503015"},
		expected: func(address *Address, err error) {
			s.NoError(err)
			s.NotNil(address)
			s.Equal(address.Cep, "06503-015")
		},
	}}

	for _, scenario := range scenarios {
		s.T().Run(scenario.name, func(t *testing.T) {
			client := NewHttpClient()
			address, _, err := MakeRequest[Address, any](s.ctx, client, "GET", fmt.Sprintf("https://viacep.com.br/ws/%s/json/", scenario.args.zipCode), nil, nil)
			scenario.expected(address, err)
		})
	}
}
