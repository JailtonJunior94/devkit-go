# devkit-go — Memory

## Project Layout
- Módulo: `github.com/JailtonJunior94/devkit-go`, Go 1.26
- Observability em `pkg/observability/` (interfaces) + `pkg/observability/otel/` (impl)
- Noop provider: `pkg/observability/noop/`
- Fake (testes): `pkg/observability/fake/`

## Observability — Arquitetura
- `Field`: discriminated union (64 bytes, 1 cache line), zero boxing para string/int/int64/float64/bool
- `SpanOption`: concrete value types `spanKindOpt` / `spanAttrsOpt` (sem closure)
- `otelSpanImpl`: poolado via `otelSpanPool` (sync.Pool); `End()` retorna ao pool via CAS atômico
- `CardinalityValidator`: removido do hot path (Add/Record); disponível em `validation.go` para uso externo
- `Shutdown`: idempotente via `sync.Once`

## Otimizações Críticas Aplicadas (2026-02-24)
1. `isSensitiveKey` → `asciiContainsFold` zero-alloc (sem strings.ToLower por chamada)
2. TraceID/SpanID → stack buffer [32]byte/[16]byte + string() copia 1x (era 2x alloc)
3. `*otelSpanImpl` → sync.Pool + CAS atômico para double-End safety
4. `Start()` fast path quando len(opts)==0 (elimina *spanConfig + []SpanStartOption allocs)
5. `SpanOption` → concrete types (sem closure heap alloc)
6. `CardinalityValidator` removido de counter.Add/histogram.Record/updown.Add
7. `otelAttrPool` cap 8→16 (alinhado com slogAttrPool)
8. `Shutdown` idempotente com sync.Once

## Otimizações Críticas Aplicadas (2026-02-25)
9. `SpanFromContext` fast path via `otelSpanContextKey` no context → **0 allocs** (era 1 alloc/chamada)
   - `Start()` armazena `*otelSpanImpl` sob key própria em context.WithValue
   - `SpanFromContext` recupera wrapper diretamente → zero alloc no hot path
   - Fallback para spans externos (1 alloc) e noSpan (0 alloc via globalNoopOtelSpan)
   - Benchmark: 4.5 ns/op, 0 B/op, 0 allocs/op
10. `spanOptsPool` → pool do []SpanStartOption no slow path de Start (evita make por chamada)
11. `Span` interface ampliada: `TraceID() string`, `SpanID() string`, `IsSampled() bool`
    - `otelSpanImpl`: stack buf [32]/[16]byte + 1 string alloc (era 2 allocs via Context())
    - `noopSpan`, `FakeSpan`: implementados, zero alloc
    - Benchmark TraceID: 30ns, 1 alloc, 32B (vs Context().TraceID(): 54ns, 2 allocs, 96B)
12. `CardinalityValidator.Validate/IsBlocked` → `strings.EqualFold` (zero alloc) em vez de `strings.ToLower` (alloc por campo)

## Padrões de Teste
- `go test ./pkg/observability/... -race` — suite completa passa
- Fake provider: não usar em benchmarks (mutex em cada log/span para captura)
- Noop provider: custo ~zero, zero-size types, globais

## pkg/events
- [Refatoração 2026-06-05](project_pkg_events_refactored.md) — production-ready: dispatcherOption unexported, WithCapacity valida negativo, Remove usa slices.Index/Delete, testes testify/suite 100% cobertura

## pkg/observability — Refatoração Production-Ready
- [Refatoração 2026-06-05](project_observability_refactor.md) — 5 fases: cardinalidade conectada, falhas observáveis, RegisterGlobal opt-in, erros padronizados, zero comentários

## pkg/messaging — Refatoração Production-Ready (2026-06-05)
- [Refatoração](project_pkg_messaging_refactor.md) — P0/P1/P2 bugs corrigidos, 0 comentários, testes de regressão adicionados

## pkg/worker — Port Production-Ready (2026-06-05)
- Port de `mecontrola/internal/platform/worker` com fixes de race conditions e observabilidade integrada
- `job.Runner` interface com Name/Schedule/Run/OverlapPolicy; `worker.Job = job.Runner` (type alias)
- Scheduler: cleanup do cron gerenciado dentro de `Start()` (não em `Stop()`); `Stop()` é no-op
- Testes: `TestMain` + `goleak.VerifyTestMain` para evitar falsos positivos entre subtests
- `robfig/cron/v3` adicionado como dependência

## Convenções do Projeto
- Sem emoji em código
- Pool: acquire+release pattern com `*[]T` para evitar double-indirect
- Campos de struct: exported Key, unexported numVal/strVal/anyVal/kind
- [Sem prefixo _ para constantes não exportadas](feedback_no_underscore_prefix.md) — usar camelCase simples
