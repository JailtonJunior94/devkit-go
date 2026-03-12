# Resumo das Tarefas de Implementação — pkg/nullable

## Metadados
- **PRD:** `tasks/prd-nullable-vos/prd.md`
- **Tech Spec:** `tasks/prd-nullable-vos/techspec.md`
- **Total de tarefas:** 6
- **Tarefas paralelizáveis:** 2.0, 3.0, 4.0, 5.0 (após 1.0 concluída)

## Tarefas

| # | Título | Status | Dependências | Paralelizável |
|---|--------|--------|-------------|---------------|
| 1.0 | Scaffold do pacote e erros sentinela | done | — | — |
| 2.0 | nullable.String | done | 1.0 | Com 3.0, 4.0, 5.0 |
| 3.0 | nullable.Int e nullable.Int64 | done | 1.0 | Com 2.0, 4.0, 5.0 |
| 4.0 | nullable.Float64 e nullable.Float32 | done | 1.0 | Com 2.0, 3.0, 5.0 |
| 5.0 | nullable.Time | done | 1.0 | Com 2.0, 3.0, 4.0 |
| 6.0 | Validação final do pacote | done | 2.0, 3.0, 4.0, 5.0 | — |

## Dependências Críticas
- **1.0 bloqueia todas as demais:** `errors.go` deve existir antes de qualquer tipo importar erros sentinela.
- **6.0 bloqueia merge:** gate de qualidade com `-race`, `go vet` e verificação de dependências externas.

## Riscos de Integração
- `Float32.Scan` converte `float64 → float32` internamente; perda de precisão deve ser coberta por teste explícito.
- `Time.UnmarshalJSON` com layout customizado requer que o receptor já tenha o layout definido; cenário de "layout mismatch" deve ser testado.
- Nomes dos tipos (`nullable.String`, `nullable.Int`) podem sombrear builtins em linters antigos — verificar no gate da 6.0.

## Legenda de Status
- `pending`: aguardando execução
- `in_progress`: em execução
- `needs_input`: aguardando informação do usuário
- `blocked`: bloqueado por dependência ou falha externa
- `failed`: falhou após limite de remediação
- `done`: completado e aprovado
