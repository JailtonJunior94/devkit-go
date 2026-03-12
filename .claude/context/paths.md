# Caminhos do Projeto

- **Core:** `internal/{module}/`
  - `domain/`: Entidades, VOs, Interfaces, Factories
  - `application/`: Use cases, DTOs
  - `infrastructure/`: Handlers HTTP, Repositórios, Adapters
- **Compartilhado:** `pkg/` (Auth, DB, O11y, Messaging)
- **Tarefas e PRDs:** `./tasks/prd-[nome-da-feature]/`
  - PRD: `prd.md`
  - Tech Spec: `techspec.md`
  - Tarefas: `tasks.md`
  - Bugs: `bugs.md`
- **Regras:** `.claude/rules/` (arquitetura, segurança, http, tratamento de erros, o11y, testes, padrões de código)
- **Templates:** `.claude/templates/`
