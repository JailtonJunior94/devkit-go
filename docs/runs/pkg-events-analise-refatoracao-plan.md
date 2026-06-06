# Analise e Plano de Refatoracao de `pkg/events`

## Resumo Executivo

Veredito: `Aprovado com ressalvas`.

`pkg/events` esta adequado para uso sincrono simples em projetos pequenos e medios: nao ha dependencia externa, `go test ./pkg/events -count=1 -race` passou, a estrutura usa `sync.RWMutex`, copia defensiva de handlers no dispatch e cobertura reportada de `95.6%`.

Nao classifico como `production-proof` generico para grande porte ainda. As ressalvas principais sao API publica com option falsamente extensivel, semantica publica inconsistente em entradas invalidas, custo linear/copia por dispatch com muitos handlers, benchmark de `Remove` enganoso em execucao completa e comentarios em testes/exemplos conflitantes com a exigencia de remover todos os comentarios.

## Scorecard

- Robustez: `4/5` - race test passou; validacoes basicas existem em `pkg/events/event_dispatcher.go:42` e `pkg/events/event_dispatcher.go:78`.
- Eficiencia: `3/5` - dispatch copia handlers em `pkg/events/event_dispatcher.go:59`; multiplos handlers geraram `1 alloc/op` no benchmark.
- Escalabilidade: `3/5` - bom para poucos handlers; registro, busca e remocao sao lineares por event type em `pkg/events/event_dispatcher.go:89`, `pkg/events/event_dispatcher.go:110`, `pkg/events/event_dispatcher.go:126`.
- Portabilidade: `5/5` - pacote sem acoplamento a framework ou infra.
- Clareza da API: `3/5` - `DispatcherOption` exportado depende de tipo nao exportado em `pkg/events/event_dispatcher.go:22`.
- Testabilidade: `4/5` - cobertura alta e exemplos passam, mas testes usam doubles manuais e muitos comentarios.

## Inventario do Pacote

Arquivos relevantes:
- `pkg/events/interface.go`: contratos publicos `Event`, `EventHandler`, `EventDispatcher`.
- `pkg/events/event_dispatcher.go`: implementacao concreta, erros sentinel, options, sincronizacao.
- `pkg/events/event_dispatcher_test.go`: testes funcionais, concorrencia, stress e benchmarks.
- `pkg/events/example_test.go`: exemplos blackbox da API publica.

Contratos publicos:
- `NewEventDispatcher(opts ...DispatcherOption) EventDispatcher`
- `WithCapacity(capacity int) DispatcherOption`
- `ErrHandlerAlreadyRegistered`, `ErrEventNil`, `ErrHandlerNil`, `ErrEventTypeEmpty`
- Metodos `Register`, `Dispatch`, `Remove`, `Has`, `Clear`.

## Achados com Evidencia

- Severidade: `medio`; classificacao: `Fato comprovado`.
  Evidencia: `pkg/events/event_dispatcher.go:22`, `pkg/events/event_dispatcher.go:24`, `pkg/events/event_dispatcher.go:30`.
  Impacto: `DispatcherOption` e publico, mas recebe `*eventDispatcher`, tipo nao exportado; consumidores conseguem usar `WithCapacity`, mas nao conseguem criar options proprias sem acessar tipo privado. Risco de falso positivo: baixo.

- Severidade: `medio`; classificacao: `Risco plausivel`.
  Evidencia: `pkg/events/event_dispatcher.go:24`, `pkg/events/event_dispatcher.go:26`.
  Impacto: `WithCapacity` nao valida capacidade negativa; `make(map[string][]EventHandler, capacity)` pode causar panic se receber valor negativo em runtime. Risco de falso positivo: baixo.

- Severidade: `baixo`; classificacao: `Fato comprovado`.
  Evidencia: `pkg/events/event_dispatcher.go:113`, `pkg/events/event_dispatcher.go:114`, `pkg/events/event_dispatcher.go:115`; comparado com `pkg/events/event_dispatcher.go:78`, `pkg/events/event_dispatcher.go:79`, `pkg/events/event_dispatcher.go:82`.
  Impacto: `Register` retorna erro para event type vazio e handler nil, enquanto `Remove` ignora os mesmos inputs. Contrato publico fica menos previsivel. Risco de falso positivo: medio, porque pode ser escolha deliberada de idempotencia.

- Severidade: `oportunidade`; classificacao: `Oportunidade de melhoria`.
  Evidencia: `pkg/events/event_dispatcher.go:59`, `pkg/events/event_dispatcher.go:60`; benchmark direcionado: `BenchmarkDispatch_MultipleHandlers` ficou em `426.1 ns/op`, `1099 B/op`, `1 alloc/op`.
  Impacto: a copia protege contra mutacao concorrente durante dispatch, mas vira custo recorrente para event types com muitos handlers. Risco de falso positivo: baixo.

- Severidade: `oportunidade`; classificacao: `Oportunidade de melhoria`.
  Evidencia: `pkg/events/event_dispatcher.go:126`, `pkg/events/event_dispatcher.go:133`.
  Impacto: `Remove` faz busca duplicada; pode virar uma passagem unica com `slices.Index` e reduzir codigo. Risco de falso positivo: baixo.

- Severidade: `baixo`; classificacao: `Fato comprovado`.
  Evidencia: `pkg/events/event_dispatcher_test.go:12`, `pkg/events/event_dispatcher_test.go:62`, `pkg/events/example_test.go:11`, `pkg/events/example_test.go:79`.
  Impacto: existem comentarios em testes e exemplos. A remocao literal de todos os comentarios quebraria exemplos se remover `// Output:`. Risco de falso positivo: baixo.

- Severidade: `baixo`; classificacao: `Fato comprovado`.
  Evidencia: `.mockery.yml:7`, `.mockery.yml:33`; `pkg/events/interface.go:12`, `pkg/events/interface.go:20`.
  Impacto: `.mockery.yml` nao declara interfaces de `pkg/events`, enquanto os testes usam doubles manuais. Pela governanca da skill, se forem tratados como mocks de interfaces, precisam entrar no mockery ou ser reclassificados como fakes locais simples. Risco de falso positivo: medio.

## Analise de Production-Readiness

Sustenta uso em producao:
- Sem acoplamento a HTTP, banco, filas ou observabilidade.
- `Dispatch` nao segura lock durante execucao dos handlers: copia em `pkg/events/event_dispatcher.go:59` e libera em `pkg/events/event_dispatcher.go:61`.
- Testes com `-race` passaram e cobrem concorrencia basica.

Bloqueia classificacao `production-proof` ampla:
- API de options exportada com tipo privado.
- Falta validacao defensiva em `WithCapacity`.
- Semantica de erro/idempotencia nao esta totalmente coerente entre metodos publicos.
- Benchmarks atuais nao sao todos confiaveis para decisao; a execucao completa ficou presa em `BenchmarkRemove`.

Aceitavel em cenarios pequenos:
- Busca linear por handlers.
- Dispatch sincrono sequencial e stop no primeiro erro.

Para cenarios medios/grandes:
- Documentar ou tornar explicito que o dispatcher e sincrono e sequencial.
- Reduzir custo de remocao e clarificar politica de erro.
- Manter copia defensiva ou trocar por estrategia imutavel apenas se benchmarks provarem ganho real.

## Plano de Refatoracao Incremental

1. Ajustar superficie publica minima.
   Mudanca: tornar `DispatcherOption` nao exportado, mantendo `WithCapacity` publico e `NewEventDispatcher(events.WithCapacity(n))` funcionando.
   Beneficio: remove falsa extensibilidade sem quebrar uso comum.
   Risco: baixo.
   Validacao: exemplos e testes blackbox.

2. Validar `WithCapacity`.
   Mudanca: tratar capacidade negativa como zero ou alterar construtor para retornar erro. Default escolhido: capacidade negativa vira zero para preservar assinatura atual.
   Beneficio: elimina panic por input invalido sem quebra de API.
   Risco: baixo.
   Validacao: teste unitario para `WithCapacity(-1)`.

3. Simplificar `Remove`.
   Mudanca: substituir `slices.Contains` + loop por `slices.Index` + remocao por slice.
   Beneficio: uma busca, menos codigo, mesmo comportamento.
   Risco: baixo.
   Validacao: testes de remocao existente e caso de handler unico.

4. Clarificar semantica de inputs invalidos.
   Mudanca: manter `Remove` idempotente se essa for a decisao, mas cobrir explicitamente event type vazio e handler nil em testes; nao alterar comportamento publico sem necessidade.
   Beneficio: contrato fica verificavel.
   Risco: baixo.
   Validacao: testes table-driven.

5. Limpar comentarios sem quebrar exemplos.
   Mudanca: remover comentarios ornamentais de testes; preservar `// Output:` ou converter exemplos para testes comuns antes de remover esses comentarios.
   Beneficio: atende a diretriz de limpeza sem perder validacao dos exemplos.
   Risco: medio se remover `// Output:` diretamente.
   Validacao: `go test ./pkg/events`.

6. Reestruturar testes apenas no escopo necessario.
   Mudanca: converter testes repetitivos para tabelas e decidir se `testHandler`/`testEvent` sao fakes locais ou mocks gerados por mockery.
   Beneficio: reduz volume e alinha governanca.
   Risco: medio por tocar muitos testes.
   Validacao: `go test ./pkg/events -count=1 -race`, cobertura e exemplos.

## Quick Wins

- Validar `WithCapacity(-1)`.
- Unexportar `DispatcherOption`.
- Trocar `Remove` para `slices.Index`.
- Remover comentarios decorativos em `event_dispatcher_test.go`.
- Corrigir benchmark de `Remove` para nao crescer estado indefinidamente durante a medicao.

## Nao Fazer

- Nao transformar o pacote em event bus assincrono sem requisito concreto.
- Nao adicionar goroutines, channels ou worker pool ao dispatch atual.
- Nao introduzir generics para payload sem demanda real de tipagem.
- Nao trocar `RWMutex` por `sync.Map` sem perfil de acesso que justifique.
- Nao remover `// Output:` dos examples mantendo-os como examples executaveis.

## Validacoes Executadas

- `ai-spec version`: `ai-spec-harness 0.27.1`.
- `ai-spec inspect .`: OK.
- `ai-spec doctor .`: OK.
- `ai-spec verify . --tools all --langs go --by-cli`: `96 current`, `0 missing`, `0 drifted`.
- `go test ./pkg/events -count=1 -race`: OK.
- `go test ./pkg/events -cover`: `95.6%`.
- `go test ./pkg/events -run 'Example' -count=1`: OK.
- Benchmark direcionado com `-benchtime=100ms`: OK.
