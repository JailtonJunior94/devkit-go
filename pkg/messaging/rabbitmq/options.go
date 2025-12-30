package rabbitmq

import "time"

// Option é uma função que modifica a configuração do Client.
// Segue o padrão functional options para APIs flexíveis e extensíveis.
type Option func(*Client)

// WithConfig define configuração customizada.
// Sobrescreve valores padrão.
func WithConfig(cfg Config) Option {
	return func(c *Client) {
		c.config = cfg
	}
}

// WithConnectionStrategy define a estratégia de conexão.
// Se não fornecida, usa CloudStrategy como padrão.
//
// Estratégias disponíveis:
//   - PlainStrategy: Desenvolvimento local sem TLS
//   - TLSStrategy: TLS com certificados customizados
//   - CloudStrategy: Produção com TLS (padrão)
func WithConnectionStrategy(strategy ConnectionStrategy) Option {
	return func(c *Client) {
		c.strategy = strategy
	}
}

// WithURL define a URL de conexão.
// Sobrescreve a URL da config.
func WithURL(url string) Option {
	return func(c *Client) {
		c.config.URL = url
	}
}

// WithHeartbeat define o intervalo de heartbeat.
func WithHeartbeat(interval time.Duration) Option {
	return func(c *Client) {
		c.config.Heartbeat = interval
	}
}

// WithConnectionTimeout define timeout para conexão.
func WithConnectionTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.config.ConnectionTimeout = timeout
	}
}

// WithPublishTimeout define timeout para publish.
func WithPublishTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.config.PublishTimeout = timeout
	}
}

// WithReconnectConfig define configuração completa de reconexão.
func WithReconnectConfig(timeout, initialInterval, maxInterval time.Duration) Option {
	return func(c *Client) {
		c.config.ReconnectTimeout = timeout
		c.config.ReconnectInitialInterval = initialInterval
		c.config.ReconnectMaxInterval = maxInterval
	}
}

// WithAutoReconnect habilita/desabilita reconexão automática.
func WithAutoReconnect(enabled bool) Option {
	return func(c *Client) {
		c.config.EnableAutoReconnect = enabled
	}
}

// WithPublisherConfirms habilita/desabilita publisher confirms.
// Recomendado: true para produção (garante entrega).
func WithPublisherConfirms(enabled bool) Option {
	return func(c *Client) {
		c.config.EnablePublisherConfirms = enabled
	}
}

// WithDefaultPrefetchCount define prefetch count padrão para consumers.
func WithDefaultPrefetchCount(count int) Option {
	return func(c *Client) {
		c.config.DefaultPrefetchCount = count
	}
}

// WithServiceName define nome do serviço.
func WithServiceName(name string) Option {
	return func(c *Client) {
		c.config.ServiceName = name
	}
}

// WithServiceVersion define versão do serviço.
func WithServiceVersion(version string) Option {
	return func(c *Client) {
		c.config.ServiceVersion = version
	}
}

// WithEnvironment define ambiente de execução.
func WithEnvironment(env string) Option {
	return func(c *Client) {
		c.config.Environment = env
	}
}

// WithPlainConnection cria cliente com PlainStrategy.
// Atalho para desenvolvimento local.
func WithPlainConnection(host, username, password, vhost string) Option {
	return func(c *Client) {
		c.strategy = NewPlainStrategy(host, username, password, vhost)
	}
}

// WithTLSConnection cria cliente com TLSStrategy.
// Atalho para conexão TLS customizada.
func WithTLSConnection(host, username, password, vhost, caCert, clientCert, clientKey string) Option {
	return func(c *Client) {
		c.strategy = NewTLSStrategy(host, username, password, vhost, caCert, clientCert, clientKey)
	}
}

// WithCloudConnection cria cliente com CloudStrategy.
// Atalho para conexão cloud (CloudAMQP, AWS MQ, etc).
func WithCloudConnection(url string) Option {
	return func(c *Client) {
		c.strategy = NewCloudStrategy(url)
	}
}
