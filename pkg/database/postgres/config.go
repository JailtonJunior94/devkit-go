package postgres

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultMaxOpenConns é o número máximo padrão de conexões no pool.
	DefaultMaxOpenConns = 25
	// DefaultMaxIdleConns é o número mínimo padrão de conexões mantidas prontas (ociosas).
	DefaultMaxIdleConns = 6
	// DefaultConnMaxLife é o tempo máximo de vida padrão de uma única conexão.
	DefaultConnMaxLife = 30 * time.Minute
	// DefaultConnMaxIdle é o tempo ocioso máximo padrão antes de uma conexão ser fechada.
	DefaultConnMaxIdle = 5 * time.Minute

	defaultPort    = 5432
	defaultSSLMode = "disable"

	// DefaultPort é a porta padrão exportada para integrações que precisam serializar
	// a configuração em formatos alternativos sem duplicar valores mágicos.
	DefaultPort = defaultPort
	// DefaultSSLMode é o sslmode padrão exportado para integrações auxiliares.
	DefaultSSLMode = defaultSSLMode
)

// PostgresConfig contém a configuração de conexão para o adaptador Postgres.
// Quando o DSN não está vazio, ele tem precedência sobre os campos individuais.
type PostgresConfig struct {
	// DSN é a string de conexão completa. Tem precedência sobre todos os outros campos quando definida.
	DSN string

	// Campos individuais — usados apenas quando o DSN está vazio.
	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	SSLMode    string
	SearchPath string

	// Configuração do pool — valores zero usam os padrões do pacote (aplicados pelo applyDefaults).
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLife  time.Duration
	ConnMaxIdle  time.Duration
}

// driverConfig espelha a interface de marcação manager.DriverConfig (definida na task 3.0).
// Declará-la aqui mantém o linter feliz e garante que PostgresConfig satisfaça o
// contrato antes que o pacote manager exista.
type driverConfig interface {
	driverConfigMarker()
	Validate() error
}

// Asserção em tempo de compilação de que PostgresConfig satisfaz driverConfig.
var _ driverConfig = PostgresConfig{}

// driverConfigMarker satisfaz manager.DriverConfig (definida na task 3.0).
func (PostgresConfig) driverConfigMarker() {}

// Validate retorna um erro agregado listando todos os campos obrigatórios ausentes.
// Quando o DSN é definido, os campos de conexão individuais não são validados.
func (c PostgresConfig) Validate() error {
	if c.DSN != "" {
		return nil
	}

	var errs []error
	if c.Host == "" {
		errs = append(errs, errors.New("postgres: host is required"))
	}
	if c.User == "" {
		errs = append(errs, errors.New("postgres: user is required"))
	}
	if c.Database == "" {
		errs = append(errs, errors.New("postgres: database is required"))
	}
	return errors.Join(errs...)
}

// ResolveDSN retorna a string de conexão a ser usada.
// O campo DSN tem precedência; caso contrário, uma é construída a partir dos campos individuais.
func (c PostgresConfig) ResolveDSN() string {
	if c.DSN != "" {
		return c.DSN
	}

	port := c.Port
	if port == 0 {
		port = defaultPort
	}

	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = defaultSSLMode
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		quoteLibpqValue(c.Host), port,
		quoteLibpqValue(c.User), quoteLibpqValue(c.Password),
		quoteLibpqValue(c.Database), quoteLibpqValue(sslMode),
	)

	if c.SearchPath != "" {
		dsn += " search_path=" + quoteLibpqValue(c.SearchPath)
	}

	return dsn
}

// quoteLibpqValue escapa um valor para o formato libpq key=value.
// Valores vazios ou contendo whitespace, aspas simples ou backslash são
// envolvidos em aspas simples; aspas e backslashes internos são escapados com
// barra invertida, conforme a especificação do libpq.
func quoteLibpqValue(v string) string {
	if v != "" && !strings.ContainsAny(v, " \t\n\r\v\f'\\") {
		return v
	}
	var b strings.Builder
	b.Grow(len(v) + 2)
	b.WriteByte('\'')
	for _, r := range v {
		if r == '\\' || r == '\'' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}
