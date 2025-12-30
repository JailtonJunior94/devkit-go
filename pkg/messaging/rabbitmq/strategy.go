package rabbitmq

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ConnectionStrategy define a interface para estratégias de conexão RabbitMQ.
// Implementa o Strategy Pattern para desacoplar configuração de conexão da lógica de negócio.
//
// Estratégias disponíveis:
//   - PlainStrategy: Conexão AMQP simples sem TLS (desenvolvimento local)
//   - TLSStrategy: Conexão AMQPS com TLS e certificados customizados
//   - CloudStrategy: Conexão otimizada para CloudAMQP/K8s (padrão para produção)
type ConnectionStrategy interface {
	// Dial estabelece conexão com RabbitMQ usando a estratégia específica
	Dial(config Config) (*amqp.Connection, error)

	// Name retorna o nome da estratégia para logs
	Name() string
}

// PlainStrategy implementa conexão AMQP sem TLS.
// Adequado para desenvolvimento local com Docker.
//
// Uso:
//
//	strategy := &PlainStrategy{
//	    Host:     "localhost",
//	    Port:     5672,
//	    Username: "guest",
//	    Password: "guest",
//	    VHost:    "/",
//	}
type PlainStrategy struct {
	Host     string
	Port     int
	Username string
	Password string
	VHost    string
}

// Dial estabelece conexão AMQP sem TLS.
func (s *PlainStrategy) Dial(config Config) (*amqp.Connection, error) {
	if s.Host == "" {
		return nil, fmt.Errorf("plain strategy: host is required")
	}

	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		s.Username,
		s.Password,
		s.Host,
		s.Port,
		s.VHost,
	)

	amqpConfig := amqp.Config{
		Heartbeat: config.Heartbeat,
		Locale:    "en_US",
	}

	conn, err := amqp.DialConfig(url, amqpConfig)
	if err != nil {
		return nil, fmt.Errorf("plain strategy: failed to connect: %w", err)
	}

	return conn, nil
}

func (s *PlainStrategy) Name() string {
	return "plain"
}

// TLSStrategy implementa conexão AMQPS com TLS e certificados.
// Adequado para ambientes com requisitos de segurança customizados.
//
// Uso:
//
//	strategy := &TLSStrategy{
//	    Host:           "rabbitmq.example.com",
//	    Port:           5671,
//	    Username:       "user",
//	    Password:       "pass",
//	    VHost:          "/",
//	    CACertPath:     "/path/to/ca.pem",
//	    ClientCertPath: "/path/to/cert.pem",
//	    ClientKeyPath:  "/path/to/key.pem",
//	}
type TLSStrategy struct {
	Host           string
	Port           int
	Username       string
	Password       string
	VHost          string
	CACertPath     string
	ClientCertPath string
	ClientKeyPath  string
	ServerName     string // Para verificação SNI
}

// Dial estabelece conexão AMQPS com TLS.
func (s *TLSStrategy) Dial(config Config) (*amqp.Connection, error) {
	if s.Host == "" {
		return nil, fmt.Errorf("tls strategy: host is required")
	}

	tlsConfig, err := s.buildTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("tls strategy: %w", err)
	}

	url := fmt.Sprintf("amqps://%s:%s@%s:%d/%s",
		s.Username,
		s.Password,
		s.Host,
		s.Port,
		s.VHost,
	)

	amqpConfig := amqp.Config{
		Heartbeat:       config.Heartbeat,
		Locale:          "en_US",
		TLSClientConfig: tlsConfig,
	}

	conn, err := amqp.DialConfig(url, amqpConfig)
	if err != nil {
		return nil, fmt.Errorf("tls strategy: failed to connect: %w", err)
	}

	return conn, nil
}

func (s *TLSStrategy) Name() string {
	return "tls"
}

// buildTLSConfig constrói configuração TLS a partir de certificados.
func (s *TLSStrategy) buildTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// ServerName para verificação SNI
	if s.ServerName != "" {
		tlsConfig.ServerName = s.ServerName
	}

	// Carrega CA certificate se fornecido
	if s.CACertPath != "" {
		caCert, err := os.ReadFile(s.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, ErrInvalidCertificate
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Carrega client certificate e key se fornecidos
	if s.ClientCertPath != "" && s.ClientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(s.ClientCertPath, s.ClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// CloudStrategy implementa conexão otimizada para CloudAMQP e RabbitMQ em Kubernetes.
// Esta é a estratégia padrão recomendada para produção.
//
// Características:
//   - TLS habilitado por padrão
//   - Heartbeat configurável
//   - Suporte a variáveis de ambiente
//   - Compatível com CloudAMQP, AWS MQ, Azure Service Bus
//   - Pronto para Kubernetes com secrets
//
// Uso:
//
//	strategy := &CloudStrategy{
//	    URL: os.Getenv("RABBITMQ_URL"), // amqps://user:pass@host:port/vhost
//	}
type CloudStrategy struct {
	// URL completa de conexão (amqp:// ou amqps://)
	// Formato: amqps://username:password@host:port/vhost
	URL string

	// TLSConfig customizada (opcional)
	// Se não fornecida, usa TLS padrão seguro
	TLSConfig *tls.Config
}

// Dial estabelece conexão usando URL completa com TLS habilitado.
func (s *CloudStrategy) Dial(config Config) (*amqp.Connection, error) {
	if s.URL == "" {
		return nil, ErrMissingURL
	}

	tlsConfig := s.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	amqpConfig := amqp.Config{
		Heartbeat:       config.Heartbeat,
		Locale:          "en_US",
		TLSClientConfig: tlsConfig,
		Dial:            amqp.DefaultDial(config.ConnectionTimeout),
	}

	conn, err := amqp.DialConfig(s.URL, amqpConfig)
	if err != nil {
		return nil, fmt.Errorf("cloud strategy: failed to connect: %w", err)
	}

	return conn, nil
}

func (s *CloudStrategy) Name() string {
	return "cloud"
}

// NewPlainStrategy cria estratégia para desenvolvimento local.
func NewPlainStrategy(host, username, password, vhost string) *PlainStrategy {
	return &PlainStrategy{
		Host:     host,
		Port:     5672,
		Username: username,
		Password: password,
		VHost:    vhost,
	}
}

// NewTLSStrategy cria estratégia com TLS customizado.
func NewTLSStrategy(host, username, password, vhost, caCertPath, clientCertPath, clientKeyPath string) *TLSStrategy {
	return &TLSStrategy{
		Host:           host,
		Port:           5671,
		Username:       username,
		Password:       password,
		VHost:          vhost,
		CACertPath:     caCertPath,
		ClientCertPath: clientCertPath,
		ClientKeyPath:  clientKeyPath,
	}
}

// NewCloudStrategy cria estratégia para ambientes cloud (padrão).
func NewCloudStrategy(url string) *CloudStrategy {
	return &CloudStrategy{
		URL: url,
	}
}

// DefaultCloudStrategy cria CloudStrategy a partir de variável de ambiente.
// Procura por RABBITMQ_URL primeiro, depois CLOUDAMQP_URL (Heroku).
func DefaultCloudStrategy() (*CloudStrategy, error) {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = os.Getenv("CLOUDAMQP_URL")
	}

	if url == "" {
		return nil, ErrMissingURL
	}

	return &CloudStrategy{
		URL: url,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}, nil
}
