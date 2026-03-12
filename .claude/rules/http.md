# Camada HTTP

- Rule ID: R-HTTP-001
- Severidade: hard para correção/segurança, guideline para escolhas de estilo.
- Escopo: `internal/*/infrastructure/http/` e pacotes HTTP compartilhados.

## Objetivo
Garantir roteamento HTTP consistente, design de handlers e convenções de API.

## Requisitos

### Framework e Handlers (hard)
- Usar `go-chi/chi` para roteamento.
- Usar assinaturas de handler `net/http`.
- Cada módulo deve expor um router dedicado com método `Register(chi.Router)`.

### Convenções de Roteamento (guideline)
- Paths de recurso em inglês, plural e kebab-case.
- Profundidade de aninhamento de recursos não deve exceder 3 níveis sem justificativa.
- Ações mutáveis que não são CRUD padrão (ex.: `POST /resources/{id}/approve`) devem usar `POST` com path de verbo.

### Fluxo do Handler
- **(hard)** Erros devem delegar para handler de erro centralizado (ver `error-handling.md`).
- **(guideline)** Ordem preferida: iniciar span -> extrair contexto de auth -> decodificar input -> validar -> executar use case -> responder.
- **(guideline)** Respostas de sucesso devem usar o utilitário compartilhado de resposta JSON.

### Contrato de API (hard)
- Payloads de request/response devem ser JSON.
- Payloads de erro devem seguir RFC 7807.
- Semântica de status code deve permanecer estável e documentada.

### Cross-Cutting (hard)
- Verificações de auth e ownership devem ser baseadas em middleware.
- Requisitos de logging e tracing estão definidos em `o11y.md`.

## Proibido
- Lógica de negócio em handlers.
- Acesso direto a banco de dados a partir de handlers.
- `http.Error` ad-hoc para erros de aplicação.
