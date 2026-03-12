# Tarefa 1.0: Scaffold do pacote e erros sentinela

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a estrutura inicial do pacote `pkg/nullable/` com os erros sentinela que serão usados pelos demais tipos. Esta tarefa não implementa nenhum tipo nulável — apenas estabelece a base do pacote.

<requirements>
- Criar o diretório `pkg/nullable/`
- Criar `pkg/nullable/errors.go` com o pacote `nullable` e os erros sentinela
- O arquivo deve declarar `ErrInvalidScan` conforme techspec.md
- Zero dependências externas — apenas `errors` da stdlib
- O pacote deve compilar com `go build ./pkg/nullable/...`
</requirements>

## Subtarefas

- [ ] 1.1 Criar `pkg/nullable/errors.go` com `package nullable`
- [ ] 1.2 Declarar `ErrInvalidScan = errors.New("nullable: unsupported scan source type")`
- [ ] 1.3 Verificar que `go build ./pkg/nullable/...` passa sem erros

## Detalhes de Implementação

Ver seção **Erros sentinela** em `techspec.md`:
- Apenas `ErrInvalidScan` é necessário neste pacote
- Erros de parse de `Time` são wrappados inline com `fmt.Errorf(...%w...)` — não precisam de sentinela
- Padrão de nomenclatura: prefixo `Err`, mensagem lowercase, sem ponto final

## Critérios de Sucesso

- `pkg/nullable/errors.go` existe com `package nullable`
- `ErrInvalidScan` está declarado e exportado
- `go build ./pkg/nullable/...` passa
- `go vet ./pkg/nullable/...` passa
- Nenhuma dependência além de `errors` da stdlib

## Testes da Tarefa

- [ ] Nenhum teste unitário para esta tarefa (sem lógica implementada)
- [ ] Verificar compilação: `go build ./pkg/nullable/...`
- [ ] Verificar vet: `go vet ./pkg/nullable/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO DONE</critical>

## Arquivos Relevantes
- `pkg/nullable/errors.go` — a criar
- `pkg/vos/errors.go` — referência de padrão do projeto
