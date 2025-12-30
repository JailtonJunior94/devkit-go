package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	}{
		{
			name: "should return an error when the request fails",
			args: args{zipCode: "06503015"},
			expected: func(address *Address, err error) {
				s.Require().NoError(err)
				s.NotNil(address)
				s.Equal("06503-015", address.Cep)
			},
		},
	}

	for _, scenario := range scenarios {
		s.T().Run(scenario.name, func(t *testing.T) {
			client := NewHTTPClient()
			address, _, err := MakeRequest[Address, any](
				s.ctx,
				client,
				"GET",
				fmt.Sprintf("https://viacep.com.br/ws/%s/json/",
					scenario.args.zipCode,
				),
				nil,
				nil,
			)
			scenario.expected(address, err)
		})
	}
}

func TestRetryableTransport(t *testing.T) {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			http.Error(w, "Temporary error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	retryPolicy := func(err error, resp *http.Response) bool {
		return resp != nil && resp.StatusCode >= 500
	}

	client := NewHTTPClientRetryable(
		WithMaxRetries(3),
		WithRetryPolicy(retryPolicy),
		WithBackoff(100*time.Millisecond),
	)

	req, err := http.NewRequest("GET", server.URL, nil)
	assert.NoError(t, err)

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer func() {
		_ = resp.Body.Close()
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
