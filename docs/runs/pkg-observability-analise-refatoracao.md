# Análise e Plano de Refatoração de `pkg/observability`

## 1. Veredito Executivo

Classificação: `parcialmente pronto`.

O pacote tem uma base útil para projetos pequenos e médios: contratos simples (`Observability`, `Logger`, `Metrics`, `Tracer`), providers `noop` e `fake`, implementação OTel, shutdown idempotente e testes direcionados passando com `-race`.
Não é seguro chamar de `pronto para producao` sem ressalvas: há opções públicas não conectadas ao comportamento real, tratamento silencioso de falhas em métricas, dependência de estado global OTel e muitos comentários em código que o prompt exige remover.
Para projetos grandes, o pacote ainda precisa reduzir ambiguidade de configuração, fortalecer falhas operacionais e separar melhor o que é contrato público do que é detalhe de provider.

Validações executadas:
- `ai-spec version`: `ai-spec-harness 0.27.1`
- `ai-spec doctor .`: OK
- `go test ./pkg/observability/...`: OK
- `go test ./pkg/observability/... -race`: OK

## 2. Matriz de Prontidão

| Critério | Status | Evidência | Impacto | Risco de falso positivo |
|---|---|---|---|---|
| API pública | Parcial | Interfaces pequenas em `observability.go`, `logger.go`, `metrics.go`, `tracer.go` | Boa ergonomia inicial | Médio: contratos cresceram com `Shutdown`, `SpanContext`, `SpanOption`, `HTTPInstrumentation` |
| Segurança por padrão | Parcial | `Insecure` bloqueado em `prod/production`; `Sanitize` default `false` | Pode expor campos sensíveis se usuário esquecer config | Alto |
| Cardinalidade de métricas | Não pronto | `EnableCardinalityCheck` existe, mas `otel/metrics.go` registra que `CardinalityValidator` foi removido do caminho de gravação | Risco real de explosão de séries | Alto |
| Falhas operacionais | Parcial | criação inválida de instrumento retorna noop silencioso em `Counter`, `Histogram`, `UpDownCounter` | Perda invisível de métrica | Alto |
| Concorrência | Bom | shutdown usa mutex/canal; tracer usa `sync.Pool` + `atomic`; `go test -race` passou | Base razoável | Médio: race não prova lifecycle sob carga real |
| Portabilidade | Parcial | provider chama `otel.SetTracerProvider`, `otel.SetMeterProvider`, `otel.SetTextMapPropagator` | Pode afetar apps que já configuram OTel global | Alto em libs e monolitos grandes |
| Eficiência | Parcial | `Field` evita boxing comum; pools em logger/tracer/conversion | Boa intenção e algum teste | Médio: benchmarks existem, mas regressão não foi validada nesta execução |
| Testabilidade | Bom | `fake`, `noop`, mocks e testes do pacote passam | Adoção fácil em testes | Baixo |
| Remoção de comentários | Não pronto | `rg "^\\s*//"` encontrou muitos comentários em arquivos hand-written e gerados | Bloqueia requisito explícito do prompt | Alto |

## 3. Achados Principais

**Alto**
- Configuração pública de cardinalidade não é aplicada no provider OTel.
  Importa porque o usuário pode acreditar que `EnableCardinalityCheck` protege métricas, mas `otelMetrics` registra diretamente os fields. Evidência: `Config.EnableCardinalityCheck` e `CustomBlockedLabels`; comentário em `otel/metrics.go` dizendo que o validator foi removido. Recomendação: remover as opções públicas ou reconectar validação em ponto previsível e testado.

- Falha de criação de instrumentos vira noop silencioso.
  Importa porque produção pode perder métricas sem alerta. Evidência: `Counter`, `Histogram`, `HistogramWithBuckets`, `UpDownCounter` retornam `noop*` quando `meter.*` falha. Recomendação: expor erro na criação do instrumento ou registrar falha por canal observável sem mascarar comportamento.

- `NewProvider` configura estado global OTel.
  Importa para projetos grandes, testes paralelos e bibliotecas importáveis. Evidência: `otel.SetTracerProvider`, `otel.SetMeterProvider`, `otel.SetTextMapPropagator`. Recomendação: tornar global opt-in ou documentar/encapsular como runtime de aplicação, não biblioteca neutra.

**Médio**
- `Sanitize` vem desabilitado por padrão.
  Evidência: `DefaultConfig` não habilita sanitização; `sanitizeFields` existe e cobre chaves sensíveis. Impacto: projetos pequenos podem usar defaults inseguros. Recomendação: tornar sanitização default em produção ou fornecer preset seguro.

- Há código e conceitos que parecem desconectados.
  Evidência: `ServiceDescriptor`, `BenchmarkBudget`, `CardinalityValidator` têm testes, mas não aparecem no wiring principal do provider. Recomendação: manter apenas contratos usados ou mover para camada de validação explícita.

- Mensagens de erro do provider estão em inglês e não usam os sentinels do próprio pacote.
  Evidência: `fmt.Errorf("ServiceName cannot be empty")`, etc. Recomendação: padronizar com `NewInvalidConfigError` e wrapping comparável.

**Baixo**
- Muitos comentários explicam micro-otimizações e decisões internas.
  Como o requisito exige remover todos os comentários, o plano deve substituir comentários por nomes, testes e estrutura mais claros.

## 4. Portabilidade e Escalabilidade

Projetos pequenos: pode ser usado com `noop`/`fake` e API básica sem grande custo, mas a configuração OTel completa é pesada e defaults como `Sanitize=false` exigem atenção.

Projetos médios: serve parcialmente, principalmente quando o app aceita OTel global e tem disciplina de labels. O risco principal é acreditar que cardinalidade está protegida por config quando não está.

Projetos grandes: não está pronto sem refatoração. Estado global, falhas silenciosas de métrica, knobs públicos não aplicados e mistura de contrato, runtime, HTTP, validação e benchmarks tornam a adoção arriscada.

## 5. Oportunidades de Simplificação

Simplificação segura:
- Remover ou conectar `EnableCardinalityCheck`, `CustomBlockedLabels` e `CardinalityValidator`; não manter opção pública sem efeito.
- Padronizar erros de config com os constructors de erro já existentes.
- Reduzir comentários substituindo por nomes e testes.
- Consolidar no provider apenas configuração realmente usada.
- Manter `noop` e `fake`, pois eles reduzem custo de adoção e teste.

Simplificação arriscada:
- Remover `Field` discriminado e voltar para `any`: reduz código, mas pode piorar hot path.
- Remover `sync.Pool` do tracer/logger sem benchmark comparativo.
- Alterar assinatura de `Metrics` para retornar erro: melhora robustez, mas quebra API pública.

## 6. Plano de Refatoração

Fase 1: alinhar contrato público com comportamento real.
Mudanças: decidir se cardinalidade será aplicada ou removida da config; preferir aplicar em `otelMetrics` com testes de bloqueio e custom labels.
Ganho: elimina falso positivo crítico.
Risco: overhead em métricas; mitigar mantendo check opt-in.
Compatibilidade: se apenas conectar opção existente, compatível.

Fase 2: tornar falhas de métrica observáveis.
Mudanças: evitar noop silencioso em criação de instrumento; introduzir logging interno ou erro explícito em factory de instrumento.
Ganho: produção não perde métrica sem sinal.
Risco: possível quebra se assinatura mudar; preferir alternativa compatível primeiro.

Fase 3: separar runtime global de provider portável.
Mudanças: tornar `otel.Set*` comportamento explícito via config, factory ou preset de aplicação.
Ganho: pacote mais seguro como biblioteca importável.
Risco: quebra operacional se apps dependem do global implícito; manter default atual numa fase inicial e adicionar opção clara.

Fase 4: endurecer defaults e validação.
Mudanças: usar sentinels de erro do pacote, validar `Environment`, normalizar `ServiceName`, revisar `Sanitize` para preset seguro de produção.
Ganho: menor chance de configuração insegura.
Risco: configs hoje aceitas podem passar a falhar; tratar como mudança documentada.

Fase 5: remover comentários e reduzir código morto.
Mudanças: remover comentários de arquivos hand-written; para mocks gerados, preservar apenas marcador obrigatório de geração se necessário ou ajustar template/regeneração conforme política do repositório.
Ganho: cumpre requisito explícito e força nomes/testes melhores.
Risco: remover comentários de generated files pode prejudicar tooling; registrar exceção técnica se o marcador `Code generated` for obrigatório.

## 7. Remoção de Comentários

Existem muitos comentários em `pkg/observability`, incluindo produção, testes, exemplos e mocks gerados.
Plano: remover comentários de código hand-written em `observability`, `noop`, `fake` e `otel`; substituir explicações por nomes mais claros e testes.
Para mocks gerados, a decisão segura é não editar manualmente: ajustar geração se a política realmente exigir zero comentários, preservando compatibilidade com `mockery` e detecção de arquivos gerados.

## 8. Test Plan

- `go test ./pkg/observability/...`
- `go test ./pkg/observability/... -race`
- Testes novos para cardinalidade aplicada em `Counter`, `Histogram`, `UpDownCounter` e custom labels.
- Testes para falha de criação de instrumento não ser silenciosa.
- Testes para provider com estado global opt-in/off, se essa interface for alterada.
- Benchmark comparativo antes/depois para tracer/logger/metrics caso remova pools ou altere `Field`.

## 9. Assumptions

- Go efetivo do repo: `go.mod` declara `go 1.26.2`; `go env GOVERSION` retornou `go1.26.4`.
- O escopo é apenas `pkg/observability`; integrações em HTTP, database e messaging entram apenas como consumidores.
- O plano não implementa nada nesta etapa.
- Veredito evita falso positivo: testes passando comprovam regressão básica, não comprovam prontidão total de produção.
