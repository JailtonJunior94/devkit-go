package rabbitmq

import "errors"

// Erros do cliente RabbitMQ.
// Seguindo o padrão de erros customizados do projeto.
var (
	// ErrClientClosed indica que o cliente foi fechado.
	ErrClientClosed = errors.New("rabbitmq: client is closed")

	// ErrConnectionClosed indica que a conexão foi fechada.
	ErrConnectionClosed = errors.New("rabbitmq: connection is closed")

	// ErrChannelClosed indica que o channel foi fechado.
	ErrChannelClosed = errors.New("rabbitmq: channel is closed")

	// ErrNoConnection indica que não há conexão ativa.
	ErrNoConnection = errors.New("rabbitmq: no active connection")

	// ErrReconnecting indica que o cliente está em processo de reconexão.
	ErrReconnecting = errors.New("rabbitmq: client is reconnecting")

	// ErrPublishTimeout indica timeout em operação de publish.
	ErrPublishTimeout = errors.New("rabbitmq: publish timeout")

	// ErrPublishConfirmFailed indica falha na confirmação de publish.
	ErrPublishConfirmFailed = errors.New("rabbitmq: publish confirm failed")

	// ErrInvalidStrategy indica que a strategy fornecida é inválida.
	ErrInvalidStrategy = errors.New("rabbitmq: invalid connection strategy")

	// ErrMissingURL indica que a URL não foi fornecida.
	ErrMissingURL = errors.New("rabbitmq: connection URL is required")

	// ErrMissingTLSConfig indica que a configuração TLS é obrigatória.
	ErrMissingTLSConfig = errors.New("rabbitmq: TLS configuration is required")

	// ErrInvalidCertificate indica certificado TLS inválido.
	ErrInvalidCertificate = errors.New("rabbitmq: invalid TLS certificate")
)
