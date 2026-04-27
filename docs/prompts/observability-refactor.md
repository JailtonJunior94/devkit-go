# Prompt Enriquecido: Refatoração do Módulo de Observabilidade

Use a skill `go-implementation` para definir a **Refatoração do Módulo de Observabilidade (pkg/observability)**.

**Entradas:**
- **Problema:** O módulo atual de observabilidade carece de uma integração nativa e fluida entre Tracing, Logging (slog) e Métricas seguindo as especificações mais recentes do OpenTelemetry. É necessário garantir alocação zero em caminhos críticos, eliminar riscos de vazamento de memória em spans não finalizados e simplificar o Graceful Shutdown unificado. A correlação entre logs e traces (trace_id nos logs) e a propagação de headers (correlation-id) não estão padronizadas.
- **Persona afetada:** Desenvolvedores de Backend e Engenheiros de SRE que precisam de rastreabilidade total e baixo overhead operacional.
- **Restrições de escopo:** Refatoração limitada ao diretório `pkg/observability` e suas implementações em `pkg/observability/otel`. Não inclui a migração de serviços legados que não utilizam este pacote.
- **Restrições técnicas:** Go 1.21+, OpenTelemetry Go SDK (versão estável), `go-slog/otelslog` para ponte de logs, conformidade com Object Calisthenics (apenas um nível de indentação por método, sem `else`, envolver primitivos).

**Saídas esperadas obrigatórias:**
- **Problema claro e verificável:** Descrição detalhada do acoplamento atual entre provedores, falta de suporte nativo a `otelslog` e falhas na propagação automática de contexto.
- **Objetivos mensuráveis:**
    - Redução verificável de alocações em heap para logging/tracing via benchmarks.
    - Cobertura de testes unitários superior a 80%.
    - Tempo de shutdown controlado por contexto em todos os exporters.
- **Não objetivos explícitos:** Não criar novos protocolos de telemetria; não implementar exporters customizados para vendors específicos (foco total em OTLP).
- **Requisitos funcionais numerados (RF-01, RF-02...):**
    - **RF-01:** Implementar `Observability Facade` que exponha `Tracer`, `Meter` e `Logger` (slog) de forma coesa.
    - **RF-02:** Integrar `otelslog` para inclusão automática de `trace_id` e `span_id` nos logs via contexto.
    - **RF-03:** Criar middlewares/interceptors padrão para HTTP (Chi/Standard), Banco de Dados (sql/pgx) e Workers para coleta automática.
    - **RF-04:** Implementar propagadores OTel (W3C TraceContext + Baggage) para `correlation-id` e `x-request-id`.
    - **RF-05:** Garantir Graceful Shutdown para finalização segura de todos os exporters.
    - **RF-06:** Disponibilizar `docker-compose.yml` com a stack `grafana/otel-lgtm:0.7.5`.
- **Requisitos não funcionais (RNF-01, RNF-02...):**
    - **RNF-01 (Performance):** Aplicar Object Calisthenics para reduzir complexidade e evitar alocações (ex: structs de valor para Fields).
    - **RNF-02 (Concorrência):** Coleta de métricas e spans thread-safe e não bloqueante.
    - **RNF-03 (Generics):** Uso de Generics para métricas tipadas onde aplicável.
    - **RNF-04 (Interfaces):** Segregação de interfaces para facilitar Mocks e testabilidade.
- **Critérios de aceite por requisito funcional:**
    - **AC-RF-01:** Inicialização da stack via chamada única `obs.New(cfg)`.
    - **AC-RF-02:** Logs dentro de spans ativos devem conter `trace_id` e `span_id`.
    - **AC-RF-03:** Métricas de latência e erro automáticas para DB e HTTP no Grafana.
    - **AC-RF-04:** Propagação de `correlation-id` de headers HTTP para chamadas downstream.
    - **AC-RF-05:** Log de confirmação de flush de telemetria no encerramento.
    - **AC-RF-06:** Stack LGTM funcional via `docker-compose up`.
- **Riscos com probabilidade e impacto:**
    - **Risco 01:** Sobrecarga de memória por buffer de spans excessivo. (Probabilidade: Média | Impacto: Alto). *Mitigação: Limites de batching e timeouts.*
    - **Risco 02:** Quebra de compatibilidade com interfaces legadas. (Probabilidade: Alta | Impacto: Médio). *Mitigação: Uso de aliases e guia de migração.*

---

## Instruções de Implementação Detalhadas
1. **Análise de Object Calisthenics:** Revise `pkg/observability` eliminando `else`, mantendo classes/structs pequenas (max 50 linhas) e pacotes pequenos (max 10 arquivos).
2. **Ponte de Logs:** Utilize `otelslog.NewHandler` como handler global do `slog`.
3. **Métricas de Banco de Dados:** Utilize `otelpgx` ou similar para instrumentar o pool de conexões.
4. **Readme:** Crie um `README.md` em `pkg/observability` com exemplos claros de:
    - Como iniciar a stack.
    - Como criar um span e logar nele.
    - Como adicionar métricas customizadas.
    - Como configurar o Grafana para visualizar os dados da stack LGTM.
