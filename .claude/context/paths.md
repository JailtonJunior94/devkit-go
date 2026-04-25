# Caminhos do Projeto

- **Fonte canônica de governança:** `AGENTS.md`
- **Skills canônicas:** `.agents/skills/`
- **Pacotes públicos:** `pkg/`
  - `pkg/observability/`: contratos + implementações OTel/noop/fake
  - `pkg/database/`: abstrações de banco, postgres, uow e otelsql
  - `pkg/messaging/`: contratos base e adapters Kafka/RabbitMQ
  - `pkg/http_server/`: adapters HTTP Chi/Fiber e código compartilhado
  - `pkg/httpserver/`: servidor HTTP Chi-based alternativo
  - `pkg/httpclient/`: cliente HTTP observável
  - `pkg/migration/`, `pkg/events/`, `pkg/vos/`, `pkg/entity/`, `pkg/logger/`
- **Infra auxiliar:** `deployment/` e `docker-compose.yml`
- **Scripts auxiliares:** `scripts/lib/`
- **Regras da Claude:** `.claude/rules/`
- **Contexto da Claude:** `.claude/context/`

Não assumir `internal/{module}` como layout principal deste repositório.
