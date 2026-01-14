package kafka

import (
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/messaging/kafka/auth"
)

const (
	defaultDialTimeout        = 10 * time.Second
	defaultConnectTimeout     = 30 * time.Second
	defaultMaxRetries         = 3
	defaultRetryBackoff       = 2 * time.Second
	defaultMaxRetryBackoff    = 30 * time.Second
	defaultHealthCheckTimeout = 5 * time.Second
	defaultReconnectInterval  = 5 * time.Second
	defaultMaxReconnectDelay  = 2 * time.Minute
)

// config holds the internal configuration for the Kafka client.
type config struct {
	// Kafka broker addresses.
	brokers []string

	// Authentication strategy.
	authStrategy auth.Strategy
	authConfig   *auth.Config

	// Logger for structured logging.
	logger Logger

	// Connection settings.
	dialTimeout       time.Duration
	connectTimeout    time.Duration
	reconnectEnabled  bool
	reconnectInterval time.Duration
	maxReconnectDelay time.Duration

	// Retry settings.
	maxRetries      int
	retryBackoff    time.Duration
	maxRetryBackoff time.Duration

	// Health check settings.
	healthCheckTimeout time.Duration
	healthCheckEnabled bool

	// Producer settings.
	producerBatchSize    int
	producerBatchTimeout time.Duration
	producerMaxAttempts  int
	producerAsync        bool
	producerCompression  int
	producerRequiredAcks int

	// Consumer settings.
	consumerGroupID        string
	consumerTopics         []string
	consumerMinBytes       int
	consumerMaxBytes       int
	consumerCommitInterval time.Duration
	consumerStartOffset    int64
	consumerMaxWait        time.Duration

	// DLQ settings.
	dlqConfig *DLQConfig

	// OpenTelemetry instrumentation (optional).
	instrumentation *Instrumentation
}

// defaultConfig returns a config instance with sensible production-ready defaults.
//
// Por quê cada padrão foi escolhido:
//   - authStrategy (Confluent): Mais seguro, adequado para produção cloud
//   - dialTimeout (10s): Tempo razoável para conexões em cloud/datacenter
//   - connectTimeout (30s): Permite retries automáticos na inicialização
//   - maxRetries (3): Balanceia resiliência vs fail-fast
//   - retryBackoff (2s): Evita sobrecarga do broker em falhas
//   - healthCheckEnabled (true): Detecta falhas precocemente
//   - reconnectEnabled (true): Auto-recuperação de conexões perdidas
//   - producerRequiredAcks (-1/all): Máxima durabilidade, mensagens não se perdem
//   - producerBatchSize (100): Balance entre throughput e latência
//   - consumerStartOffset (-1/newest): Não reprocessa mensagens antigas no início
func defaultConfig() *config {
	return &config{
		authStrategy:           auth.NewStrategy(auth.StrategyConfluent),
		authConfig:             &auth.Config{},
		logger:                 NewNoopLogger(),
		dialTimeout:            defaultDialTimeout,
		connectTimeout:         defaultConnectTimeout,
		maxRetries:             defaultMaxRetries,
		retryBackoff:           defaultRetryBackoff,
		maxRetryBackoff:        defaultMaxRetryBackoff,
		healthCheckTimeout:     defaultHealthCheckTimeout,
		healthCheckEnabled:     true,
		reconnectEnabled:       true,
		reconnectInterval:      defaultReconnectInterval,
		maxReconnectDelay:      defaultMaxReconnectDelay,
		producerBatchSize:      100,
		producerBatchTimeout:   time.Second,
		producerMaxAttempts:    3,
		producerAsync:          false,
		producerCompression:    0,
		producerRequiredAcks:   -1,
		consumerMinBytes:       1e3,
		consumerMaxBytes:       1e7,
		consumerCommitInterval: time.Second,
		consumerStartOffset:    -1,
		consumerMaxWait:        500 * time.Millisecond,
		dlqConfig:              defaultDLQConfig(),
	}
}
