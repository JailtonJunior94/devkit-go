# Tratamento de Erros

- Rule ID: R-ERR-001
- Severidade: hard
- Escopo: Todos os arquivos `.go` com criação, wrapping, tratamento ou mapeamento de erros.

## Objetivo
Garantir definição, propagação e mapeamento seguro de erros para o cliente.

## Requisitos

### Erros Sentinela
- Erros de domínio compartilhados: `pkg/custom_errors/errors.go` como `var Err* = errors.New(...)`.
- Erros de domínio do módulo: `internal/{module}/domain/errors.go`.
- Nomes devem usar prefixo `Err`.
- Mensagens devem ser lowercase e concisas.

### Wrapping e Propagação
- Usar `fmt.Errorf(... %w ...)` para wrapping.
- Preservar cadeia de erros para `errors.Is` e `errors.As`.
- Domain retorna erros sentinela ou sentinela wrapped.
- Use case propaga erros de domínio e faz wrap de erros de infra com contexto.
- Handler delega resposta de erro para handler de erro centralizado.
- Repository retorna erros brutos de infraestrutura.

### Mapeamento de Erro para HTTP
- Usar mapper de erro centralizado.
- Ordem de mapeamento: match direto -> `errors.Is` -> erros tipados conhecidos -> fallback 500.
- Formato de resposta de erro deve seguir RFC 7807.
- Nunca expor detalhes internos de erro para clientes (ver R-SEC-001).

### Verificações de Erro
- Usar `errors.Is` e `errors.As`.
- Nunca comparar erros com `==`.

## Proibido
- Engolir erros silenciosamente.
- Expor detalhes internos de erro em respostas HTTP (reforçado por R-SEC-001).
- Definir erros de domínio em camadas de infra/application.
- Usar `panic` para erros recuperáveis.
