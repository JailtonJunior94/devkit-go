# Copilot Instructions

Use `AGENTS.md` como fonte canônica de instruções deste repositório.

## Carregamento de contexto

1. Ler `AGENTS.md` no início da tarefa.
2. Antes de editar código, carregar `.agents/skills/agent-governance/SKILL.md`.
3. Ao alterar código Go, carregar `.agents/skills/go-implementation/SKILL.md`.
4. Carregar skills de planejamento apenas quando a tarefa pedir explicitamente análise de projeto, PRD, especificação técnica ou decomposição em tarefas.

## Regras essenciais

1. Tratar o projeto como toolkit Go em monolito modular, com organização predominante por componente em `pkg/`.
2. Não assumir `internal/{module}` ou Clean Architecture global como layout canônico.
3. Preservar contratos públicos dos pacotes salvo quando a mudança declarar explicitamente quebra ou evolução de API.
4. Validar mudanças com comandos proporcionais ao risco e registrar ausências de tooling em vez de inventar substitutos.

## Validação

- `make lint`
- `make test`
- `make test-integration`
- `make vulncheck`
