package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Database representa uma conexão gerenciada com PostgreSQL.
// É thread-safe e projetada para uso em produção.
// Não deve ser copiada após criação - sempre use ponteiros.
type Database struct {
	db     *sql.DB
	mu     sync.RWMutex // Protege o estado durante operações de shutdown
	closed bool         // Indica se a conexão foi encerrada
}

// New cria uma nova instância de Database com a URI fornecida.
// A conexão é estabelecida imediatamente e testada com Ping.
//
// Parâmetros:
//   - uri: string de conexão PostgreSQL (ex: "postgres://user:pass@host:port/dbname?sslmode=disable")
//   - opts: opções funcionais para configurar pool e timeouts
//
// Retorna erro se:
//   - A URI estiver vazia
//   - Falhar ao abrir a conexão
//   - Falhar no ping inicial
//
// Exemplo:
//
//	db, err := postgres.New(
//	    "postgres://user:pass@localhost:5432/mydb",
//	    postgres.WithMaxOpenConns(25),
//	    postgres.WithConnMaxLifetime(5 * time.Minute),
//	)
func New(uri string, opts ...Option) (*Database, error) {
	if uri == "" {
		return nil, fmt.Errorf("postgres: URI não pode estar vazia")
	}

	// Abre a conexão usando o driver pgx
	db, err := sql.Open("pgx", uri)
	if err != nil {
		return nil, fmt.Errorf("postgres: falha ao abrir conexão: %w", err)
	}

	// Cria a instância com valores padrão seguros para produção
	d := &Database{
		db:     db,
		closed: false,
	}

	// Aplica configurações padrão do pool ANTES das opções customizadas
	// Isso garante que tenhamos valores sensatos mesmo sem options
	d.applyDefaultPoolConfig()

	// Aplica opções customizadas do usuário
	for _, opt := range opts {
		opt(d)
	}

	// Testa a conexão imediatamente - fail-fast
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.db.PingContext(ctx); err != nil {
		// Se o ping falhar, fecha a conexão para evitar leak
		_ = db.Close()
		return nil, fmt.Errorf("postgres: falha ao pingar banco: %w", err)
	}

	return d, nil
}

// applyDefaultPoolConfig configura valores padrão do pool de conexões.
// Cada configuração é justificada para prevenir problemas em produção.
func (d *Database) applyDefaultPoolConfig() {
	// MaxOpenConns: Limite máximo de conexões abertas (incluindo em uso + idle)
	// Por quê: Previne exaustão de recursos no servidor PostgreSQL.
	// Valor padrão: 25 conexões é adequado para a maioria das aplicações.
	// Se sua app tem alto throughput, ajuste via WithMaxOpenConns().
	d.db.SetMaxOpenConns(25)

	// MaxIdleConns: Conexões idle mantidas no pool
	// Por quê: Manter conexões idle reduz latência ao evitar handshake a cada requisição.
	// Porém, conexões idle consomem memória. 25% do MaxOpenConns é um bom balanço.
	// Valor padrão: 25% das conexões máximas (aprox. 6 conexões).
	d.db.SetMaxIdleConns(6)

	// ConnMaxLifetime: Tempo máximo de vida de uma conexão
	// Por quê: Força rotação de conexões para prevenir:
	//   - Memory leaks em drivers
	//   - Conexões "stale" após mudanças de rede/firewall
	//   - Acúmulo de problemas em conexões antigas
	// Valor padrão: 5 minutos é seguro e evita overhead excessivo de reconnect.
	d.db.SetConnMaxLifetime(5 * time.Minute)

	// ConnMaxIdleTime: Tempo máximo que uma conexão idle pode permanecer no pool
	// Por quê: Libera recursos quando a aplicação está ociosa.
	// Previne manter conexões abertas indefinidamente em períodos de baixa carga.
	// Valor padrão: 2 minutos - conexões idle por mais tempo são fechadas.
	d.db.SetConnMaxIdleTime(2 * time.Minute)
}

// DB retorna a instância *sql.DB subjacente.
// Esta instância é thread-safe e pode ser usada diretamente em repositories.
//
// IMPORTANTE: Não chame Close() diretamente no *sql.DB retornado.
// Use sempre o método Shutdown() desta struct para garantir graceful shutdown.
//
// Retorna nil se a conexão já foi fechada.
func (d *Database) DB() *sql.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return nil
	}

	return d.db
}

// Ping verifica se a conexão com o banco está ativa.
// Respeita o contexto para cancelamento/timeout.
//
// Use em:
//   - Health checks (endpoints /health, /ready, /live)
//   - Validação periódica de conectividade
//   - Após reconexão
//
// É thread-safe e pode ser chamado concorrentemente.
//
// Retorna erro se:
//   - O contexto for cancelado/timeout
//   - A conexão estiver fechada
//   - O banco não responder
func (d *Database) Ping(ctx context.Context) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return fmt.Errorf("postgres: conexão já foi fechada")
	}

	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres: falha no ping: %w", err)
	}

	return nil
}

// Shutdown encerra a conexão com o banco de forma graciosa.
// O context é verificado ANTES de iniciar o Close(), mas Close() em si é bloqueante.
//
// Comportamento:
//   - Verifica se context já expirou ANTES de iniciar Close()
//   - Marca a conexão como fechada para prevenir novas operações
//   - Executa Close() bloqueante (pode exceder deadline do ctx)
//   - Close() NÃO pode ser interrompido uma vez iniciado
//   - É idempotente e thread-safe
//
// IMPORTANTE:
//   - Close() é bloqueante por design no database/sql
//   - Se context expirar DURANTE Close(), a operação continua até completar
//   - Trade-off: Preferimos fechar conexões completamente a deixá-las órfãs
//
// Parâmetros:
//   - ctx: contexto verificado antes de iniciar shutdown
//
// Retorna erro se:
//   - O contexto já estiver expirado antes de iniciar
//   - Ocorrer erro ao fechar a conexão
//
// Exemplo:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//	if err := db.Shutdown(ctx); err != nil {
//	    log.Printf("Erro no shutdown: %v", err)
//	}
func (d *Database) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Idempotência: se já foi fechada, não faz nada
	if d.closed {
		return nil
	}

	// Verifica se context já expirou ANTES de iniciar Close()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("postgres: shutdown abortado (context expirado): %w", err)
	}

	// Marca como fechada para prevenir novas operações
	d.closed = true

	// Close() é bloqueante e NÃO respeita ctx.Done() após iniciar
	// Aguarda todas as conexões ativas finalizarem naturalmente
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("postgres: erro ao fechar conexão: %w", err)
	}

	return nil
}
