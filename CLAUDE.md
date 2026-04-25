# Claude Code

Use `AGENTS.md` como fonte canônica de instruções deste repositório.

## Instruções

1. Ler `AGENTS.md` no início da sessão.
2. Antes de editar código, carregar `.agents/skills/agent-governance/SKILL.md`.
3. Ao alterar código Go, carregar também `.agents/skills/go-implementation/SKILL.md`.
4. Carregar skills de planejamento apenas quando a tarefa pedir explicitamente análise de projeto, PRD, especificação técnica ou decomposição em tarefas.
5. Tratar o repositório como toolkit Go em monolito modular, com fronteiras relevantes entre pacotes de `pkg/`.
6. Preservar a API pública dos pacotes salvo quando a mudança explicitar alteração contratual.
7. Validar alterações com os comandos reais detectados no projeto, proporcionalmente ao risco.

## Validação

- `make lint`
- `make test`
- `make test-integration`
- `make vulncheck`

## Observação

Arquivos em `.claude/context/` e `.claude/rules/` são auxiliares. Em caso de conflito, prevalece `AGENTS.md`.
