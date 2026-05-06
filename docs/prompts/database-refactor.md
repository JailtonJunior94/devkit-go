Use a skill **create-prd** para definir a **Refatoração do Database Manager e Unit of Work**.

Entradas:
- **Problema:** O `pkg/database` atual possui uma implementação acoplada ao Postgres e uma estrutura de Unit of Work que precisa de maior maturidade para suportar múltiplos bancos de dados (CockroachDB, MySQL, MSSQL, Postgres) de forma transparente. Há necessidade de garantir a eliminação de vazamentos de memória, evitar race conditions em operações concorrentes e garantir que o graceful shutdown interrompa operações de forma segura e atômica, permitindo a troca de banco na `main` sem efeitos colaterais.
- **Persona afetada:** Desenvolvedores Backend que utilizam o `devkit-go` como fundação para microserviços e precisam alternar entre diferentes provedores de banco de dados sem alterar a lógica de negócio.
- **Restrições de escopo:** Focar exclusivamente no `pkg/database` e subpacotes. Não alterar a lógica de negócio de outros pacotes que dependem deste (apenas garantir compatibilidade via interfaces). Não incluir migrações de dados reais, apenas o mecanismo de execução via `golang-migrate`.
- **Restrições técnicas:** Linguagem Go (v1.21+). Drivers: `pgx/v5` (Postgres/CockroachDB), `go-sql-driver/mysql`, `go-mssqldb`. Bibliotecas: `sqlx`, `golang-migrate/migrate/v4`. Padrões: Object Calisthenics (9 regras), Functional Options, Generics, Unit of Work Atômico, Graceful Shutdown.

Saidas esperadas obrigatorias:
- **problema claro e verificavel:** A arquitetura atual viola o Dependency Inversion Principle (DIP), impedindo a substituição do banco de dados na `main` de forma transparente. O Unit of Work não é agnóstico ao driver e carece de garantias de atomicidade em cenários de `panic` ou desligamento abrupto.
- **objetivos mensuráveis:** 
    1. Cobertura de testes de 100% nos métodos públicos.
    2. Zero alertas no `go test -race`.
    3. Zero memory leaks detectados em benchmarks de estresse.
    4. Troca de driver na `main` realizada em apenas 1 linha de código/configuração.
- **nao objetivos explicitos:** 
    1. Implementação de repositórios de domínio ou lógica de negócio.
    2. Configuração de infraestrutura (Docker Compose/K8s) externa ao ambiente de teste.
    3. Suporte a bancos NoSQL ou drivers não listados.
- **requisitos funcionais numerados (RF-01, RF-02...):**
    - **RF-01:** Implementar interface `Manager` que abstraia a conexão para Postgres, CockroachDB, MySQL e MSSQL.
    - **RF-02:** Integrar `golang-migrate` para execução automática de migrações no startup baseado no driver ativo.
    - **RF-03:** Refatorar `Unit of Work` para garantir atomicidade total: Rollback obrigatório em Panic/Erro e Commit apenas em sucesso.
    - **RF-04:** Implementar `Graceful Shutdown` que bloqueia novas transações e aguarda as existentes (timeout configurável).
    - **RF-05:** Criar Factory para instanciar o banco de dados dinamicamente na `main`.
- **requisitos nao funcionais (RNF-01, RNF-02...):**
    - **RNF-01:** Seguir rigorosamente as 9 regras do Object Calisthenics (ex: um nível de indentação, sem `else`, métodos curtos).
    - **RNF-02:** Utilizar Generics para abstrair a manipulação de transações e pools de conexão.
    - **RNF-03:** Otimizar pools de conexão (MaxOpen, MaxIdle, LifeTime) por driver para alta performance.
    - **RNF-04:** Instrumentação com OpenTelemetry para métricas de latência e saúde do pool.
- **criterios de aceite por requisito funcional:**
    - **CA (RF-01):** Interface única deve funcionar para todos os 4 bancos sem type assertions no código cliente.
    - **CA (RF-02):** O startup deve falhar se a migração falhar, impedindo estado inconsistente.
    - **CA (RF-03):** Testes unitários devem provar que um `panic` dentro do UoW resulta em `rollback` e re-emissão do `panic`.
    - **CA (RF-04):** O shutdown deve respeitar o `context.Context` e retornar erro se o timeout expirar antes das queries terminarem.
- **riscos com probabilidade e impacto:**
    - **Risco 1:** Incompatibilidade de sintaxe de migração (Impacto: Alto | Probabilidade: Média). Mitigação: Pastas de migração isoladas por driver.
    - **Risco 2:** Deadlocks em transações concorrentes (Impacto: Alto | Probabilidade: Baixa). Mitigação: Implementar timeouts de lock e testes de race.
    - **Risco 3:** Overhead de abstração (Impacto: Médio | Probabilidade: Baixa). Mitigação: Benchmarks comparativos para validar que a latência é negligenciável.
