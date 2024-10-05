package httpclient

import (
	"net/http"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewHttpClient() HttpClient {
	return &http.Client{}
}
