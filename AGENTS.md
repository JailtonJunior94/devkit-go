# Regras para Agentes de IA

Este diretório centraliza regras para uso com agentes de IA em tarefas reais de análise, alteração e validação de código.

## Objetivo

Use estas instruções para manter consistência, segurança e qualidade ao trabalhar com código, configuração, validação e evolução de sistemas.

## Arquitetura

### Classificação

O repositório deve ser tratado como **monolito modular orientado a toolkit**.

Isso significa:

1. Existe um único módulo Go na raiz (`go.mod`) e um pipeline único de validação.
2. O repositório publica um conjunto de pacotes reutilizáveis sob `pkg/`, não múltiplos serviços independentes.
3. As fronteiras relevantes são **entre pacotes e componentes reutilizáveis**, não entre deploys ou workspaces.

### Evidências

- `go.mod` único na raiz.
- `pkg/` concentra componentes independentes por capacidade: `observability`, `database`, `messaging`, `httpserver`, `http_server`, `vos`, `events`, `migration`.
- Não há `go.work`, múltiplos `go.mod`, `apps/`, `services/` ou `cmd/` de aplicações independentes.
- O `README.md` descreve o projeto como uma coleção de pacotes Go reutilizáveis.

### Padrão arquitetural predominante

O padrão predominante é **package-by-component**, com uso localizado de **ports/adapters** dentro de alguns componentes.

Exemplos:

- `pkg/observability` expõe contratos centrais, enquanto `pkg/observability/otel`, `pkg/observability/noop` e `pkg/observability/fake` são implementações.
- `pkg/database` expõe a abstração `DBTX`, enquanto `pkg/database/postgres`, `pkg/database/postgres_otelsql` e `pkg/database/uow` a especializam.
- `pkg/messaging` define contratos base, enquanto `pkg/messaging/kafka` e `pkg/messaging/rabbitmq` implementam adapters concretos.
- `pkg/http_server/common` concentra configuração e comportamento compartilhado entre `chi_server` e `server_fiber`.

Não tratar este repositório como Clean Architecture global em `internal/{module}`. Essa premissa não descreve a estrutura atual e introduz regras erradas.

## Stack detectada

- Linguagem principal: Go
- HTTP: Chi e Fiber
- Observabilidade: OpenTelemetry, Prometheus, Jaeger, Grafana, Loki
- Banco e persistência: `database/sql`, pgx, otelsql, golang-migrate
- Mensageria: Kafka e RabbitMQ
- Logging: Zap
- Testes: Go test, Testify, Testcontainers
- Qualidade: golangci-lint, govulncheck, mockery
- Infra auxiliar: Docker Compose e Terraform

## Mapa das Pastas Mais Importantes

```text
.
├── AGENTS.md
├── .agents/skills/              # skills e referências canônicas
├── .claude/                     # contexto, regras, commands, agents e hooks para Claude
├── .codex/config.toml           # metadados de skills para Codex
├── .github/                     # workflows, agents, skills e instruções do Copilot
├── deployment/
│   ├── iac/                     # Terraform
│   ├── observability/           # collector, Prometheus, Grafana
│   └── strimzi/                 # manifests Kafka
├── pkg/
│   ├── observability/           # contratos + impls OTel/noop/fake
│   ├── database/                # DBTX + postgres/uow/otelsql
│   ├── messaging/               # contratos + kafka/rabbitmq
│   ├── http_server/             # adapters HTTP Chi/Fiber + shared common
│   ├── httpserver/              # servidor HTTP Chi-based alternativo
│   ├── httpclient/              # cliente HTTP observável
│   ├── migration/               # migrações
│   ├── events/                  # eventos in-process
│   ├── vos/                     # value objects
│   ├── entity/                  # base entity
│   ├── logger/                  # logger abstraído
│   ├── responses/               # helpers HTTP
│   ├── encrypt/                 # hashing e segurança
│   ├── nullable/                # tipos nullable
│   └── linq/                    # utilitários genéricos
└── scripts/lib/                 # scripts auxiliares
```

## Fluxo de Dependências

As dependências devem respeitar o seguinte sentido:

1. Pacotes de contrato e shared kernel não devem depender de adapters concretos.
2. Adapters concretos podem depender de contratos base e de utilitários compartilhados.
3. Exemplos e testes de integração podem depender dos pacotes publicados, mas não devem definir a arquitetura principal.

Fluxos principais:

- `pkg/observability` -> base para `pkg/httpclient`, `pkg/http_server/*`, `pkg/messaging/rabbitmq`, `pkg/observability/otel`, `pkg/observability/noop`, `pkg/observability/fake`
- `pkg/database` -> base para `pkg/database/uow`
- `pkg/messaging` -> base para `pkg/messaging/kafka`
- `pkg/vos` -> shared kernel leve para `pkg/entity`, `pkg/logger`, `pkg/httpserver`
- `pkg/http_server/common` -> base compartilhada para `pkg/http_server/chi_server` e `pkg/http_server/server_fiber`

Regras de dependência:

1. `pkg/observability`, `pkg/database`, `pkg/messaging` e `pkg/vos` devem permanecer independentes de adapters de transporte e deploy.
2. Evite dependências circulares entre pacotes de `pkg/`.
3. Se um pacote existe para expor contrato público, prefira depender dele em vez de depender de um subpacote concreto.
4. Infra local em `deployment/` e `docker-compose.yml` não deve ditar desenho interno dos pacotes.

## Modo de trabalho

1. Entender o contexto antes de editar qualquer arquivo.
2. Preferir a menor mudança segura que resolva a causa raiz.
3. Preservar arquitetura, convenções e fronteiras já existentes no contexto analisado.
4. Não introduzir abstrações, camadas ou dependências sem demanda concreta.
5. Atualizar ou adicionar testes quando houver mudança de comportamento.
6. Rodar validações proporcionais à mudança.
7. Registrar bloqueios e suposições explicitamente quando o contexto estiver incompleto.

## Diretrizes de Estrutura

1. Priorize entendimento do código e do contexto atual antes de propor refatorações.
2. Respeite padrões existentes de nomenclatura, organização e tratamento de erro.
3. Defina estrutura simples, evolutiva e com defaults explícitos.
4. Evite reescritas amplas quando uma alteração localizada resolver o problema.
5. Estabeleça contratos, testes e comandos de validação cedo quando eles ainda não existirem.
6. Considere risco de regressão como restrição principal.
7. Evite overengineering disfarçado de arquitetura futura.

## Regras Contextuais para Este Repositório

1. Trate `AGENTS.md` como fonte canônica de governança; arquivos de `.claude/`, `.github/` e `.codex/` devem delegar para ele em vez de duplicar regras.
2. Não assumir `internal/{module}` ou uma divisão global `domain/application/infrastructure`; isso não reflete a estrutura atual.
3. Ao editar `pkg/`, preserve a API pública do pacote salvo quando a mudança declarar explicitamente quebra ou evolução contratual.
4. Antes de criar um novo pacote de topo em `pkg/`, verifique se a mudança cabe em um componente existente.
5. Ao adicionar dependência entre componentes de `pkg/`, explicite por que a direção é necessária e verifique risco de acoplamento indevido.
6. `pkg/httpserver` e `pkg/http_server` coexistem no repositório; antes de alterar ou expandir comportamento HTTP, confirme em qual dos dois componentes a mudança deve viver.
7. Infraestrutura local em `deployment/` e `docker-compose.yml` serve a desenvolvimento, observabilidade e testes; não trate esses diretórios como boundary de aplicação.

## Contrato de carga base

Toda skill que altera código deve carregar, como primeiro passo, a seguinte base obrigatória:

1. Ler este `AGENTS.md`.
2. Ler `.agents/skills/agent-governance/SKILL.md`.

Essa base define governança para análise, alteração e validação, carregamento sob demanda de regras de DDD, erros, segurança e testes, e critérios mínimos de preservação arquitetural, risco e validação proporcional.

Skills individuais devem declarar apenas cargas adicionais específicas ao seu contexto.

## Regras por Linguagem

Para tarefas que alteram código Go, carregar também:

- `.agents/skills/go-implementation/SKILL.md`

Para tarefas que alteram código Node/TypeScript, carregar também:

- `.agents/skills/node-implementation/SKILL.md`

Para tarefas que alteram código Python, carregar também:

- `.agents/skills/python-implementation/SKILL.md`

Para tarefas de revisão ou refatoração incremental de design em Go guiadas por heurísticas de object calisthenics, carregar também:

- `.agents/skills/object-calisthenics-go/SKILL.md`

Para tarefas de correção de bugs com remediação e teste de regressão, carregar também:

- `.agents/skills/bugfix/SKILL.md`

## Referências

Cada skill lista suas próprias referências em `references/` com gatilhos de carregamento no respectivo `SKILL.md`. Não duplicar a listagem aqui. Consultar o `SKILL.md` da skill ativa para saber quais referências carregar e em que condição.

## Ferramentas de IA Detectadas

- Claude Code: presente em `.claude/`
- Codex: presente em `.codex/config.toml`
- GitHub/Copilot: presente em `.github/`

Ferramentas detectadas devem delegar para `AGENTS.md` como fonte canônica. Se existir divergência entre arquivos auxiliares e este documento, prevalece `AGENTS.md`.

## Convenções de Teste

O documento canônico de convenções de teste unitário está em `docs/testing/unit_test.md`. Ele define:
- Framework (testify/require, testify/suite, testify/mock)
- Estrutura AAA e table-driven
- Naming em inglês e comportamental
- Uso de mocks gerados por mockery v3
- Obrigatoriedade de `-race`
- Exemplos de referência (`pkg/observability`, `pkg/database`)

## Validação

Antes de concluir uma alteração:

1. Seguir a Etapa 4 de `.agents/skills/agent-governance/SKILL.md`.
2. Rodar formatter nos arquivos alterados quando aplicável.
3. Rodar testes direcionados primeiro, depois ampliar o escopo se o risco justificar.
4. Registrar ausência de comando quando o projeto não oferecer o passo esperado.

Comandos reais detectados neste repositório:

- `make lint`
- `make test`
- `make test-integration`
- `make vulncheck`
- `go test ./...`
- `go test -tags=integration ./...`

## Restrições

1. Não inventar contexto ausente.
2. Não assumir versão de linguagem, framework ou runtime sem verificar.
3. Não alterar comportamento público sem deixar isso explícito.
4. Não usar exemplos como cópia cega; adaptar ao contexto real.
5. Não impor arquitetura diferente da atual sem demanda explícita.
6. Não tratar helpers, exemplos ou infraestrutura local como prova de que existe uma aplicação única em produção neste repositório.
