package auth

import (
	"crypto/tls"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// Confluent implements Strategy for Confluent Cloud authentication.
// This is the recommended strategy for production environments using Confluent Kafka.
//
// Confluent Cloud uses:
//   - SASL_SSL security protocol
//   - SCRAM-SHA-512 mechanism (default) or PLAIN
//   - TLS 1.2+ encryption
//   - API Key/Secret authentication
//
// Por quê: Confluent Cloud exige SASL_SSL para todas as conexões, garantindo
// segurança em trânsito e autenticação forte. SCRAM-SHA-512 é preferível por
// ser mais seguro que PLAIN, mas PLAIN é suportado para compatibilidade.
type Confluent struct{}

// Configure creates a dialer configured for Confluent Cloud.
//
// Parâmetros:
//   - config: Configuração de autenticação contendo:
//     * Username: API Key do Confluent Cloud
//     * Password: API Secret do Confluent Cloud
//     * Algorithm: ScramAlgorithm (SHA256 ou SHA512). Se vazio, usa SHA512.
//     * TLSConfig: Configuração TLS customizada (opcional)
//     * InsecureSkipVerify: NUNCA usar true em produção
//
// Retorna:
//   - *kafka.Dialer configurado para Confluent Cloud
//   - error se a configuração falhar
//
// Por quê: Confluent Cloud exige configurações específicas diferentes de
// Kafka auto-gerenciado. Esta strategy encapsula essas diferenças.
//
// Exemplo:
//
//	client, err := kafka.NewClient(
//	    kafka.WithBrokers("pkc-xxxxx.us-east-1.aws.confluent.cloud:9092"),
//	    kafka.WithAuthConfluent("YOUR_API_KEY", "YOUR_API_SECRET"),
//	)
func (cf *Confluent) Configure(config *Config) (*kafka.Dialer, error) {
	// Confluent Cloud default: SCRAM-SHA-512
	// Mais seguro que PLAIN, recomendado para produção
	algorithm := config.Algorithm
	if algorithm == "" {
		algorithm = ScramSHA512
	}

	mechanism, err := scram.Mechanism(scram.SHA512, config.Username, config.Password)
	if err != nil {
		// Fallback para SHA256 se SHA512 falhar
		if algorithm == ScramSHA256 {
			mechanism, err = scram.Mechanism(scram.SHA256, config.Username, config.Password)
		}
		if err != nil {
			return nil, err
		}
	}

	// TLS Configuration para Confluent Cloud
	// MinVersion TLS 1.2 é obrigatório pela maioria dos providers cloud
	tlsConfig := config.TLSConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			// Confluent Cloud usa certificados válidos
			// InsecureSkipVerify deve ser false (padrão)
			InsecureSkipVerify: config.InsecureSkipVerify,
		}
	}

	// Configuração do Dialer otimizada para Confluent Cloud
	return &kafka.Dialer{
		SASLMechanism: mechanism,
		TLS:           tlsConfig,
		Timeout:       10 * time.Second,  // Timeout adequado para conexões cloud
		DualStack:     true,               // Suporta IPv4 e IPv6
		ClientID:      "devkit-go-client", // Identificação do cliente
	}, nil
}

// Name returns the strategy name.
func (cf *Confluent) Name() string {
	return string(StrategyConfluent)
}
