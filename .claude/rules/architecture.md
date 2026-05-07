# Arquitetura do Toolkit

- Rule ID: R-ARCH-001
- Severidade: hard
- Escopo: Código-fonte Go em `pkg/*`, exemplos e artefatos de governança relacionados à arquitetura.

## Objetivo

Garantir que mudanças preservem a arquitetura real do repositório: um toolkit Go organizado por componente, com contratos públicos em `pkg/` e adapters concretos localizados.

## Fonte Canônica

- `AGENTS.md`
- `.agents/skills/agent-governance/SKILL.md`
- `.agents/skills/go-implementation/SKILL.md` quando a tarefa alterar código Go

## Requisitos

### Estrutura do Repositório

- Tratar o projeto como **monolito modular orientado a toolkit**, não como aplicação única em `internal/{module}`.
- Cada pacote de topo em `pkg/` representa um componente público ou um shared kernel reutilizável.
- Antes de criar um novo pacote de topo em `pkg/`, verificar se a responsabilidade cabe em componente já existente.

### Fronteiras de Dependência

- Pacotes de contrato e base (`observability`, `database`, `messaging`, `vos`) não devem depender de adapters concretos de transporte, deploy ou integração específica.
- Adapters concretos podem depender de contratos compartilhados, nunca o contrário.
- Não introduzir dependências circulares entre pacotes de `pkg/`.
- Se a dependência for nova entre componentes, justificar a direção e avaliar impacto na API pública.

### Contratos Públicos

- Alterações em `pkg/` devem presumir impacto em consumidores externos.
- Não quebrar API pública sem explicitar a mudança.
- Quando existir pacote raiz com contrato público, preferir depender dele em vez de depender diretamente de subpacote concreto.

### HTTP e Transporte

- `pkg/http_server` é o único componente HTTP do toolkit. Alterações de comportamento HTTP devem viver aqui.
- Adapters disponíveis: `pkg/http_server/chi_server` (Chi) e `pkg/http_server/server_fiber` (Fiber), com código compartilhado em `pkg/http_server/common`.

### Infra Local

- `deployment/` e `docker-compose.yml` servem para infraestrutura local, observabilidade e suporte operacional.
- Não usar diretórios de infraestrutura como justificativa para impor fronteiras de aplicação inexistentes no código.

### Modelagem Local

- Dentro de cada componente, contracts-first e ports/adapters são aceitáveis quando já adotados pelo pacote.
- Não impor uma camada global `domain/application/infrastructure` onde ela não existe.
- Reutilizar abstrações já expostas, como `database.DBTX` e `observability.Observability`, antes de criar novas interfaces.

## Proibido

- Assumir que o layout canônico do projeto é `internal/{module}/domain/application/infrastructure`.
- Reescrever componentes para Clean Architecture global sem demanda explícita.
- Introduzir dependência de adapter concreto em pacote base.
- Criar pacote novo em `pkg/` para responsabilidade já coberta por componente existente.
