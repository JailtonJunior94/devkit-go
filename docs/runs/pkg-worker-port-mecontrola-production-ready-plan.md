# `pkg/worker` — Plano de Port Production-Proof do `mecontrola/internal/platform/worker`

Plano completo, auditavel e pronto para execucao do port de `internal/platform/worker` (LimaTeixeiraTecnologia/mecontrola)
para `pkg/worker` (devkit-go), com bar **100% production-ready / production-proof** exigido pelo prompt
`docs/prompts/pkg-worker-implementacao-clone-mecontrola.md`.

Data do plano: 2026-06-05
Origem: https://github.com/LimaTeixeiraTecnologia/mecontrola/tree/main/internal/platform/worker
Mecanismo de inspecao: `gh` CLI (api repos/.../contents) — obrigatorio pelo prompt.

---

## 1. Context

A copia byte-a-byte da origem **nao** atende ao bar pedido por tres motivos concretos identificados via inspecao:

1. **Logger**: a origem usa `*slog.Logger` direto. Este repo possui `pkg/observability` e a regra `R-O11Y-001` (severidade hard) exige a abstracao. Manter `slog` puro violaria a governanca canonica.
2. **Idioma**: simbolos (`errDuplicateName`, mensagens `"worker: nome duplicado"`) e nomes de teste (`TestStart_NomeDuplicadoEmJobs`) em portugues violam `R-CODE-001` / `00-governance.md` (politica de idioma hard: simbolos em ingles).
3. **Defeitos latentes vs. "production-proof"**: a origem tem racewindows reais (Registry sem mutex; Manager nao-idempotente; race em `errs` no Stop com timeout; Scheduler.Register sem lock). Sem fixes, o port quebraria `R-TEST-001` (race detector limpo) e contradiria o pedido textual "sem race conditions, memory leak ou alocacao desnecessaria".

**Decisoes confirmadas pelo usuario neste turno:**

| Tema | Decisao |
|---|---|
| Logger | Dois construtores (`NewManager` com `observability.Observability`; `NewManagerWithSlog` com `*slog.Logger`) |
| Idioma | Traduzir simbolos e mensagens para ingles |
| Fixes | Aplicar correcoes com testes de regressao (race + goleak) |

---

## 2. Fonte inspecionada (`gh` CLI)

Comando usado (gravado para auditoria):

```bash
gh api repos/LimaTeixeiraTecnologia/mecontrola/contents/internal/platform/worker
gh api repos/LimaTeixeiraTecnologia/mecontrola/contents/internal/platform/worker/job
gh api repos/LimaTeixeiraTecnologia/mecontrola/contents/internal/platform/worker/consumer
gh api repos/LimaTeixeiraTecnologia/mecontrola/contents/internal/platform/worker/consumer/database
# conteudos individuais:
gh api repos/LimaTeixeiraTecnologia/mecontrola/contents/internal/platform/worker/<file> --jq '.content' | base64 -d
```

Mapa de arquivos da origem (14 arquivos: 9 codigo, 5 teste):

```
internal/platform/worker/
  config.go               174 B   Config{ShutdownTimeout}; defaultConfig() = 30s
  errors.go               169 B   errDuplicateName, errStopTimeout (PT)
  types.go                243 B   interfaces Job, Consumer
  manager.go            2 653 B   orquestracao Start/Stop + ShutdownTimeout + WaitGroup
  manager_test.go       3 221 B   testify/suite (PT)
  job/
    types.go               99 B   OverlapPolicy (Skip/Allow)
    adapter.go            902 B   Adapter (Name/Schedule/Run/OverlapPolicy)
    scheduler.go        1 941 B   wrapper robfig/cron/v3 + atomic.Bool (Skip) + WaitGroup (Allow)
    scheduler_test.go   3 115 B   schedule invalido, overlap, cancelamento (PT)
    adapter_test.go       959 B   policy default + delegacao (PT)
  consumer/
    types.go              678 B   Handler, HandlerFunc, Message, Source, Runner
    registration.go       102 B   Registration{Name, EventType, Handler}
    registry.go         1 139 B   Registry: map[string]Handler — SEM mutex
    registry_test.go    2 347 B   register/dispatch (PT)
    runner.go             749 B   Source -> Registry.Dispatch + logging de erro
    runner_test.go      2 440 B   propaga erros, despacha mensagens (PT)
    adapter.go            561 B   Adapter (Name/Technology + delega Start/Stop)
    adapter_test.go     1 295 B   delegacao (PT)
    database/
      adapter.go          623 B   adapter database — retorna *adapter nao exportado
```

Conteudo integral dos arquivos foi obtido e analisado neste turno (registro no transcript da sessao).

---

## 3. Plano de port (estrutura alvo)

```
pkg/worker/
  config.go                # Config{ShutdownTimeout} + defaults
  errors.go                # ErrDuplicateName, ErrStopTimeout, ErrAlreadyStarted, ErrSchedulerStarted, ErrNilHandler, ErrDuplicateEventType, ErrUnknownEventType
  types.go                 # Job, Consumer interfaces (semantica fiel)
  internal_obs.go          # adapter interno *slog.Logger -> observability.Observability (tracer/metrics via noop)
  manager.go               # Manager + atomic.Int32 state machine + dual-constructor
  manager_test.go          # testify/suite + goleak + tests -race; idempotencia; race em Stop; nomes EN
  job/
    types.go               # OverlapPolicy
    adapter.go             # Adapter
    scheduler.go           # Scheduler + sync.Mutex em Register (proibe Register apos Start)
    adapter_test.go        # nomes EN
    scheduler_test.go      # nomes EN + goleak; teste Register-after-Start
  consumer/
    types.go               # Handler, HandlerFunc, Message, Source, Runner
    registration.go
    registry.go            # sync.RWMutex em handlers
    runner.go              # observability span por dispatch + log estruturado + counter
    adapter.go             # Adapter generico
    registry_test.go       # nomes EN + teste concorrente (race)
    runner_test.go         # nomes EN
    adapter_test.go        # nomes EN
    database/
      adapter.go           # tipo *Adapter exportado retornado por NewAdapter
      adapter_test.go      # adicionado (origem nao tinha)
```

### 3.1 Mapeamento de adaptacoes (todas minimas e justificadas)

| Origem | Destino | Justificativa |
|---|---|---|
| `errDuplicateName = errors.New("worker: nome duplicado")` | `ErrDuplicateName = errors.New("worker: duplicate name")` | Hard rule R-CODE-001 (idioma ingles) + exportado para `errors.Is` por consumidores |
| `errStopTimeout = errors.New("worker: timeout de shutdown excedido")` | `ErrStopTimeout = errors.New("worker: shutdown timeout exceeded")` | Idem |
| Construtor unico `NewManager(cfg, jobs, consumers, logger *slog.Logger)` | `NewManager(cfg, jobs, consumers, obs observability.Observability)` + `NewManagerWithSlog(cfg, jobs, consumers, *slog.Logger)` | R-O11Y-001 (hard) + decisao do usuario |
| Registry: `map[string]Handler` sem sync | `sync.RWMutex` envolvendo `handlers` | Production-proof: race entre Register e Dispatch concorrentes |
| Manager start/stop sem idempotencia | `atomic.Int32` state machine (idle, running, stopping, stopped) | Double-Start retorna `ErrAlreadyStarted`; double-Stop e no-op seguro |
| Manager.Stop: `errs` mutado por goroutine + `append(errs, errStopTimeout)` no select sem sync | `sync.Mutex` protege append; snapshot antes do `errors.Join` | Race real detectada por `go test -race`: goroutines ainda escrevendo em `errs` quando timeout dispara |
| Scheduler.Register sem lock; sem proibicao apos Start | `sync.Mutex` + flag started; Register apos Start retorna `ErrSchedulerStarted` | Evita race no slice `s.jobs` |
| `consumer/database.NewAdapter` retorna `*adapter` (nao exportado) | Tipo `*Adapter` exportado | Tipo nao exportado retornado por func exportada confunde consumidores e linter (R-CODE-001 guideline mas importante) |
| Tests em portugues | Tests em ingles (ex.: `TestStart_DuplicateNamesInJobs`, `TestOverlapAllow_NoGoroutineLeak`) | Hard rule R-CODE-001 |
| Sem cobertura de race/goleak | `go.uber.org/goleak` em lifecycle tests; `-race` em todos | Production-proof obrigatorio (regra do prompt) |
| Sem metrics | Counters: `worker.jobs.executions_total{job, result}`, `worker.jobs.errors_total{job}`, `worker.consumers.dispatches_total{consumer, event_type, result}`, `worker.consumers.errors_total{consumer, event_type}` | R-O11Y-001 guideline + production observability |
| Sem spans | Span por execucao (`worker.job.run`) e por dispatch (`worker.consumer.dispatch`) com atributos | R-O11Y-001 guideline |
| Logs em portugues e mensagens livres | Mensagens em ingles com chaves padronizadas (`operation`, `name`, `event_type`, `error`) | R-O11Y-001 guideline |

### 3.2 Comentarios em Go

**Regra inegociavel do prompt e do AGENTS.md**: zero comentarios em codigo Go novo ou alterado, inclusive godoc.

### 3.3 Dependencia nova

- `github.com/robfig/cron/v3 v3.0.1` — `go get github.com/robfig/cron/v3@v3.0.1` + `go mod tidy`.

---

## 4. Detalhes criticos

### 4.1 Manager (`pkg/worker/manager.go`)

Esqueleto operacional:

```go
const (
    stateIdle int32 = iota
    stateRunning
    stateStopping
    stateStopped
)

type Manager struct {
    cfg       Config
    jobs      []Job
    consumers []Consumer
    obs       observability.Observability
    scheduler *job.Scheduler
    cancel    context.CancelFunc
    state     atomic.Int32
    wg        sync.WaitGroup
    errsMu    sync.Mutex
}

func NewManager(cfg Config, jobs []Job, consumers []Consumer, obs observability.Observability) *Manager
func NewManagerWithSlog(cfg Config, jobs []Job, consumers []Consumer, logger *slog.Logger) *Manager
```

`Start(ctx)`:
1. CAS `stateIdle -> stateRunning`. Se falhar e estado for `stateRunning|stateStopping`, retorna `ErrAlreadyStarted`. Se for `stateStopped`, retorna `ErrAlreadyStarted` tambem (Manager nao reusa).
2. `validateNames()` — fail-fast antes de mutar mais estado; se falhar, CAS de volta para `stateIdle`.
3. `runCtx, cancel := context.WithCancel(ctx)`; `m.cancel = cancel`; `m.scheduler = job.NewScheduler(m.obs)`.
4. Registra jobs (`scheduler.Register`) — se algum falhar, cancela, CAS volta para `stateIdle` e retorna erro wrapped.
5. `m.wg.Go(func(){ m.scheduler.Start(runCtx) })`.
6. Para cada consumer, goroutine que invoca `c.Start(runCtx)`; ignora `context.Canceled`/`DeadlineExceeded`; loga erro real via `obs.Logger().Error`.

`Stop(ctx)`:
1. CAS `stateRunning -> stateStopping`. Se ja `stateStopped`, retorna nil (idempotente). Se nunca rodou (`stateIdle`), retorna nil tambem.
2. `m.cancel()`; `m.scheduler.Stop()`.
3. `stopCtx, stopCancel := context.WithTimeout(ctx, m.cfg.ShutdownTimeout)`; defer `stopCancel`.
4. `errs := make([]error, 0, len(m.consumers))`; mutex `m.errsMu`.
5. Para cada consumer, goroutine que chama `c.Stop(stopCtx)`; em erro, `errsMu.Lock(); errs = append(...); errsMu.Unlock()`.
6. `done := make(chan struct{})`; goroutine que `m.wg.Wait(); swg.Wait(); close(done)`.
7. `select` entre `done` e `stopCtx.Done()`:
   - `done`: snapshot sob lock; `errors.Join(snapshot...)`.
   - `stopCtx.Done()`: snapshot sob lock; `errors.Join(append(snapshot, ErrStopTimeout)...)`. Nao mexe mais em `errs`.
8. `state.Store(stateStopped)`.

### 4.2 Scheduler (`pkg/worker/job/scheduler.go`)

```go
type Scheduler struct {
    cron    *cron.Cron
    obs     observability.Observability
    mu      sync.Mutex
    started bool
    jobs    []registeredJob
    allowWg sync.WaitGroup
}
```

- `Register(j runner) error` adquire `mu`; se `started`, retorna `ErrSchedulerStarted`; valida schedule com `cron.ParseStandard`; append em `jobs`.
- `Start(ctx)`:
  - `mu.Lock(); started = true; jobs := s.jobs; mu.Unlock()`.
  - Para cada `rj` (closure-safe), `s.cron.AddFunc(rj.schedule, ...)` com a logica original:
    - `OverlapSkip`: `CompareAndSwap(false, true)`; defer `Store(false)`.
    - `OverlapAllow`: `allowWg.Add(1)` antes de `go func(){...}()`.
  - Para cada execucao real: abrir span `worker.job.run` com atributos; counter `worker.jobs.executions_total{job, result}`; counter `worker.jobs.errors_total{job}` em erro.
  - `s.cron.Start()`; bloqueia em `<-ctx.Done()`.
- `Stop()`: `stopCtx := s.cron.Stop(); <-stopCtx.Done(); s.allowWg.Wait()`.

### 4.3 Registry (`pkg/worker/consumer/registry.go`)

```go
type registry struct {
    mu       sync.RWMutex
    handlers map[string]Handler
}
```

- `Register`: write-lock; checa `errDuplicateEventType` (-> `ErrDuplicateEventType`); checa `ErrNilHandler`.
- `Dispatch`: read-lock; recupera handler; libera lock; chama `handler.Handle(...)` fora do lock (evita deadlock se handler chamar Register).
- Padrao identico a `pkg/events/event_dispatcher.go:18` (reutilizacao de estilo confirmada na inspecao do repo).

### 4.4 Runner (`pkg/worker/consumer/runner.go`)

- Span `worker.consumer.dispatch` com atributos `consumer.name`, `event.type`.
- Counter `worker.consumers.dispatches_total{consumer, event_type, result}`.
- Em erro: span `RecordError` + `SetStatus(error)`; counter `worker.consumers.errors_total{consumer, event_type}`; log estruturado.

### 4.5 Internal obs adapter (`pkg/worker/internal_obs.go`)

- Pacote interno (no path do pkg/worker, sem subpacote `internal/`): adapter pequeno que implementa `observability.Logger` delegando para `*slog.Logger`. Tracer/metrics vem de `pkg/observability/noop.NewProvider()`. Resultado: `obs := newObsFromSlog(logger)` que monta um `Provider{tracer: noop, logger: slogAdapter, metrics: noop}`.

### 4.6 Reutilizacao confirmada

- `pkg/observability/noop`: usado para tracer/metrics quando o construtor `NewManagerWithSlog` e escolhido.
- `pkg/observability` interfaces: contratos primarios.
- `pkg/events/event_dispatcher.go`: referencia de estilo para `RWMutex + copy-out` (nao e dep direta).

### 4.7 Restricoes de qualidade

- Zero comentarios em Go (regra inegociavel).
- Zero alocacao evitavel no hot path: counters reusam labels; spans com option types do `pkg/observability`; sem `fmt.Sprintf` em loops de dispatch.
- Sem goroutine leak: todas as goroutines tracked por `wg`/`swg`/`allowWg`; testes de leak com `goleak.VerifyNone`.
- Sem race: `-race` obrigatorio nos testes; mutexes em todos os pontos identificados.

---

## 5. Lista exata de arquivos

### Criar

```
pkg/worker/config.go
pkg/worker/errors.go
pkg/worker/types.go
pkg/worker/internal_obs.go
pkg/worker/manager.go
pkg/worker/manager_test.go
pkg/worker/job/types.go
pkg/worker/job/adapter.go
pkg/worker/job/scheduler.go
pkg/worker/job/adapter_test.go
pkg/worker/job/scheduler_test.go
pkg/worker/consumer/types.go
pkg/worker/consumer/registration.go
pkg/worker/consumer/registry.go
pkg/worker/consumer/runner.go
pkg/worker/consumer/adapter.go
pkg/worker/consumer/registry_test.go
pkg/worker/consumer/runner_test.go
pkg/worker/consumer/adapter_test.go
pkg/worker/consumer/database/adapter.go
pkg/worker/consumer/database/adapter_test.go
```

### Modificar

```
go.mod                    # + github.com/robfig/cron/v3 v3.0.1
go.sum                    # gerado por go mod tidy
```

### Nao criar

- READMEs (regra: nao criar docs sem pedido explicito).
- Quaisquer arquivos sob `pkg/worker/internal/` (manter layout flat conforme origem).

---

## 6. Validacao (comandos exatos)

Rodar na raiz do repositorio, na ordem:

1. **Governanca:** `ai-spec verify . --tools all --langs go --by-cli`
2. **Format:** `gofmt -l pkg/worker` (deve sair vazio)
3. **Vet:** `go vet ./pkg/worker/...`
4. **Build:** `go build ./pkg/worker/...`
5. **Testes com race:** `go test ./pkg/worker/... -race -count=1`
6. **Testes globais (regressao):** `go test ./... -count=1` quando proporcional
7. **Lint (se disponivel):** `golangci-lint run ./pkg/worker/...`
8. **Drift de spec (se aplicavel):** `ai-spec lint .` + `ai-spec doctor .`

### 6.1 Testes obrigatorios (evidencia de production-proof)

| Teste | O que comprova |
|---|---|
| `manager_test.go::TestStartStop_Success` | start + stop limpos, goleak.VerifyNone |
| `manager_test.go::TestStart_DuplicateNamesInJobs` | `errors.Is(err, worker.ErrDuplicateName)` |
| `manager_test.go::TestStart_DuplicateNamesInConsumers` | Idem para consumers |
| `manager_test.go::TestStart_DuplicateAcrossJobAndConsumer` | Cobre o cruzamento (gap da origem) |
| `manager_test.go::TestStart_IsIdempotent` | Segundo Start -> `ErrAlreadyStarted` |
| `manager_test.go::TestStop_IsIdempotent` | Duplo Stop seguro |
| `manager_test.go::TestStop_AggregatesConsumerErrorsWithoutRace` | N consumers retornando erro; `-race` limpo |
| `manager_test.go::TestStop_TimeoutAppendsSentinel` | `errors.Is(err, worker.ErrStopTimeout)` com consumer travado |
| `manager_test.go::TestNewManagerWithSlog_Works` | Paridade do construtor alternativo |
| `manager_test.go::TestStop_NoGoroutineLeak` | `goleak.VerifyNone(t)` apos Stop |
| `job/scheduler_test.go::TestRegister_InvalidSchedule` | erro de parse de cron |
| `job/scheduler_test.go::TestRegister_ValidSchedule` | happy path |
| `job/scheduler_test.go::TestRegister_AfterStartReturnsError` | `ErrSchedulerStarted` |
| `job/scheduler_test.go::TestOverlapSkip_NoConcurrentExecution` | maxConcurrent <= 1 |
| `job/scheduler_test.go::TestOverlapAllow_NoGoroutineLeak` | goleak limpo |
| `job/scheduler_test.go::TestCancellation_StopsExecution` | Encerra apos ctx cancel |
| `job/adapter_test.go::TestNewAdapter_DefaultPolicyIsSkip` | default policy |
| `job/adapter_test.go::TestNewAdapterWithPolicy_AllowPolicy` | policy explicita |
| `job/adapter_test.go::TestAdapter_RunDelegates` | delega para fn |
| `consumer/registry_test.go::TestRegister_Success` | happy path |
| `consumer/registry_test.go::TestRegister_NilHandler` | `ErrNilHandler` |
| `consumer/registry_test.go::TestRegister_DuplicateEventType` | `ErrDuplicateEventType` |
| `consumer/registry_test.go::TestDispatch_Success_PassesParamsAndBody` | params/body integros |
| `consumer/registry_test.go::TestDispatch_UnknownEventType` | `ErrUnknownEventType` |
| `consumer/registry_test.go::TestDispatch_PropagatesHandlerError` | wrap correto |
| `consumer/registry_test.go::TestRegistry_ConcurrentRegisterAndDispatch` | N goroutines com `-race` |
| `consumer/runner_test.go::TestStart_PropagatesSourceError` | propaga |
| `consumer/runner_test.go::TestStart_DispatchesMessages` | dispatch ok |
| `consumer/runner_test.go::TestStop_CallsSourceStop` | delega |
| `consumer/runner_test.go::TestStop_PropagatesSourceStopError` | propaga |
| `consumer/adapter_test.go::TestAdapter_NameAndTechnology` | getters |
| `consumer/adapter_test.go::TestAdapter_DelegatesStart` | delega |
| `consumer/adapter_test.go::TestAdapter_DelegatesStop` | propaga erro |
| `consumer/database/adapter_test.go::TestDatabaseAdapter_TechnologyIsDatabase` | constante correta |
| `consumer/database/adapter_test.go::TestDatabaseAdapter_DelegatesLifecycle` | delegacao |

---

## 7. Sequencia de execucao

1. `go get github.com/robfig/cron/v3@v3.0.1` + `go mod tidy`.
2. Criar `pkg/worker/types.go`, `errors.go`, `config.go`, `internal_obs.go` (base).
3. Criar `pkg/worker/job/{types,adapter,scheduler}.go`.
4. Criar `pkg/worker/consumer/{types,registration,registry,runner,adapter}.go`.
5. Criar `pkg/worker/consumer/database/adapter.go`.
6. Criar `pkg/worker/manager.go`.
7. Criar todos os `*_test.go` (em ingles, com `-race` + `goleak`).
8. `gofmt -l pkg/worker` (vazio), `go vet ./pkg/worker/...`, `go build ./pkg/worker/...`.
9. `go test ./pkg/worker/... -race -count=1`.
10. `ai-spec verify . --tools all --langs go --by-cli`.
11. `go test ./... -count=1` para checar regressao global.
12. (Opcional) `golangci-lint run ./pkg/worker/...`.

---

## 8. Criterios de aceitacao

- `pkg/worker` reproduz fielmente estrutura, contratos, semantica e fluxos da origem.
- Uso de `gh` CLI registrado neste documento.
- Zero comentarios em Go (verificado por inspecao + lint).
- Skill `go-implementation` carregada e exemplos consultados sob demanda.
- Sem divergencia arquitetural alem das adaptacoes desta tabela.
- `go test ./pkg/worker/... -race -count=1` verde.
- `goleak.VerifyNone` verde nos testes de lifecycle.
- Erros sentinelas exportados em ingles e cobertos por `errors.Is` nos testes.
- Construtor primario aceita `observability.Observability`; construtor alternativo aceita `*slog.Logger`.
- `ai-spec verify` e `gofmt`/`go vet` verdes.
- Veredito final embasado em logs reais dos comandos, sem linguagem vaga.

---

## 9. Riscos residuais conhecidos (a confirmar na execucao)

- Dependencia nova `robfig/cron/v3` adiciona transitivos potenciais — monitorar diff de `go.sum`.
- Spans/metrics adicionam custo no hot path; benchmarks nao sao obrigatorios mas devem ser considerados se houver alvo de zero-alloc rigoroso.
- A origem nao tem teste para `consumer/database/adapter`; o teste novo aqui e adicao (nao port) — declarado como tal.

---

## 10. Veredito esperado pos-execucao

Port fiel a estrutura e semantica da origem, elevado para production-ready conforme exigencia explicita do prompt:
sincronizacao correta, idempotencia, observabilidade integrada, simbolos em ingles, zero comentarios, testes com
`-race` + `goleak`. Nada e declarado pronto sem evidencia concreta dos comandos da Secao 6.
