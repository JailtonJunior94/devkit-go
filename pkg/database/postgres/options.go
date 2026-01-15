package postgres

import "time"

// Option é uma função que modifica a configuração da Database.
// Segue o padrão functional options para APIs flexíveis e extensíveis.
type Option func(*Database)

// WithMaxOpenConns define o número máximo de conexões abertas ao banco.
// Inclui conexões em uso + conexões idle.
//
// Valores especiais:
//   - n > 0: Limita a n conexões máximas (recomendado em produção)
//   - n = 0: Sem limite (unlimited) - NÃO RECOMENDADO em produção
//   - n < 0: Tratado como 0 (sem limite)
//
// Quando usar:
//   - Aplicações com alto throughput: aumente para 50-100
//   - Aplicações com baixo throughput: mantenha 10-25
//   - Ambientes com limitação de conexões no PostgreSQL
//
// Impacto:
//   - Muito baixo: latência aumenta (contention no pool)
//   - Muito alto: exaustão de recursos no servidor PostgreSQL
//   - Sweet spot: monitorar "wait_count" nas métricas do pool
//
// Padrão: 25 conexões.
func WithMaxOpenConns(n int) Option {
	return func(d *Database) {
		d.db.SetMaxOpenConns(n)
	}
}

// WithMaxIdleConns define o número máximo de conexões idle no pool.
// Conexões idle ficam prontas para uso imediato sem handshake.
//
// Valores especiais:
//   - n > 0: Mantém até n conexões idle
//   - n = 0: Não mantém conexões idle (fecha imediatamente após uso)
//   - n < 0: Tratado como 0 (nenhuma conexão idle)
//
// Quando usar:
//   - Alto throughput com tráfego constante: mantenha próximo de MaxOpenConns
//   - Tráfego esporádico: reduza para liberar recursos
//
// Impacto:
//   - Muito baixo: latência em picos de requisições (handshake repetido)
//   - Muito alto: consumo desnecessário de memória e conexões no PostgreSQL
//   - Sweet spot: 25-50% de MaxOpenConns para tráfego variável
//
// Importante: deve ser <= MaxOpenConns
// Padrão: 6 conexões (25% de 25).
func WithMaxIdleConns(n int) Option {
	return func(d *Database) {
		d.db.SetMaxIdleConns(n)
	}
}

// WithConnMaxLifetime define o tempo máximo de vida de uma conexão.
// Após este período, a conexão é fechada e recriada.
//
// Valores especiais:
//   - d > 0: Conexão expira após d tempo
//   - d = 0: Conexões são reutilizadas indefinidamente (sem expiração)
//   - d < 0: Tratado como 0 (sem expiração)
//
// Quando usar:
//   - Ambientes com proxies/load balancers: reduza para 3-5 min
//   - Conexões diretas estáveis: pode aumentar para 10-15 min
//
// Impacto na prevenção de problemas:
//   - Memory leaks: rotação periódica libera memória acumulada
//   - Conexões stale: previne problemas após mudanças de rede
//   - Problemas intermitentes: isola bugs temporários no driver
//
// Performance:
//   - Muito baixo: overhead de reconnect frequente
//   - Muito alto: acúmulo de problemas em conexões antigas
//   - Sweet spot: 5-10 minutos para maioria dos casos
//
// Padrão: 5 minutos.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(db *Database) {
		db.db.SetConnMaxLifetime(d)
	}
}

// WithConnMaxIdleTime define quanto tempo uma conexão idle pode ficar no pool.
// Conexões idle por mais tempo que isso são fechadas.
//
// Valores especiais:
//   - d > 0: Conexão idle expira após d tempo
//   - d = 0: Conexões idle não expiram por tempo (podem ficar indefinidamente)
//   - d < 0: Tratado como 0 (sem expiração)
//
// Quando usar:
//   - Tráfego variável: reduza para 1-2 min (libera recursos rápido)
//   - Tráfego constante: aumente para 5 min (mantém conexões prontas)
//
// Impacto na prevenção de memory leaks:
//   - Libera recursos automaticamente durante períodos ociosos
//   - Previne acúmulo de conexões idle desnecessárias
//   - Reduz footprint de memória em períodos de baixa carga
//
// Performance:
//   - Muito baixo: handshake frequente em apps com tráfego esporádico
//   - Muito alto: mantém conexões abertas indefinidamente
//   - Sweet spot: 2-3 minutos para tráfego variável
//
// Padrão: 2 minutos.
func WithConnMaxIdleTime(d time.Duration) Option {
	return func(db *Database) {
		db.db.SetConnMaxIdleTime(d)
	}
}

// WithPoolConfig configura todos os parâmetros do pool de uma vez.
// Útil quando você tem um conjunto de configurações pré-definido.
//
// Exemplo:
//
//	postgres.WithPoolConfig(25, 10, 5*time.Minute, 2*time.Minute)
func WithPoolConfig(maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) Option {
	return func(sql *Database) {
		sql.db.SetMaxOpenConns(maxOpen)
		sql.db.SetMaxIdleConns(maxIdle)
		sql.db.SetConnMaxLifetime(maxLifetime)
		sql.db.SetConnMaxIdleTime(maxIdleTime)
	}
}
