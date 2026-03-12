# Tarefa 6.0: Validação final do pacote

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Gate de qualidade final: verificar que o pacote `pkg/nullable` está completo, correto, sem corridas de dados e sem dependências externas. Esta tarefa não implementa código novo — apenas valida o que foi entregue nas tarefas 2.0 a 5.0 e corrige qualquer problema encontrado.

<requirements>
- `go test ./pkg/nullable/... -race` passa sem falhas
- `go vet ./pkg/nullable/...` passa sem avisos
- `go build ./pkg/nullable/...` passa
- Nenhuma dependência externa além da stdlib (verificar `go mod tidy` não adiciona entradas)
- Todos os 6 tipos implementados e testados: String, Int, Int64, Float32, Float64, Time
- Todos os critérios de aceitação do PRD (CA-01 a CA-08) verificados
</requirements>

## Subtarefas

- [ ] 6.1 Executar `go test ./pkg/nullable/... -race -v` e registrar resultado
- [ ] 6.2 Executar `go vet ./pkg/nullable/...` e corrigir eventuais avisos
- [ ] 6.3 Executar `go build ./pkg/nullable/...` e confirmar sucesso
- [ ] 6.4 Verificar ausência de dependências externas: `go list -m -json all | grep -v github.com/JailtonJunior94` não deve listar nenhum módulo novo introduzido por `pkg/nullable`
- [ ] 6.5 Verificar CA-07: `XxxOf(zero-value).IsNull() == false` para cada tipo (String, Int, Int64, Float32, Float64, Time)
- [ ] 6.6 Verificar CA-02: round-trip JSON para cada tipo
- [ ] 6.7 Verificar CA-03: marshal de `XxxEmpty()` produz `null`; unmarshal de `null` produz `IsNull() == true`
- [ ] 6.8 Verificar CA-04 e CA-05: Scan/Value para cada tipo
- [ ] 6.9 Confirmar que nenhum arquivo do pacote usa `fmt.Println`, `log.Println` ou qualquer output não estruturado (R-O11Y-001)
- [ ] 6.10 Confirmar que nenhum arquivo do pacote tem acesso a banco de dados real, rede ou I/O externo

## Detalhes de Implementação

Esta tarefa é exclusivamente de validação. Se qualquer verificação falhar:
1. Identificar a tarefa de origem do problema (2.0–5.0)
2. Corrigir o problema no arquivo correspondente
3. Re-executar o gate completo
4. Limite de remediação: 2 ciclos por problema encontrado

**Checklist de Critérios de Aceitação do PRD:**

| CA | Critério | Como verificar |
|---|---|---|
| CA-01 | Todos os 6 tipos compilam, passam go vet | `go build` + `go vet` |
| CA-02 | Round-trip JSON para todos os tipos | Testes `TestXxx_JSONRoundTrip` |
| CA-03 | JSON null: marshal de Empty → `null`; unmarshal de `null` → IsNull | Testes `TestXxx_MarshalNull`, `TestXxx_UnmarshalNull` |
| CA-04 | SQL: Scan(nil) → IsNull; Scan(v) → Get correto | Testes `TestXxx_ScanNil`, `TestXxx_ScanValue` |
| CA-05 | Value() null → nil; Value() presente → valor correto | Testes `TestXxx_ValueNull`, `TestXxx_ValuePresent` |
| CA-06 | Equal reflexivo e simétrico | Testes `TestXxx_EqualReflexive`, `TestXxx_EqualSymmetric` |
| CA-07 | Of(zero-value) ≠ Empty() para todos os tipos | Testes `TestXxx_ZeroValueIsNotNull` |
| CA-08 | Nenhuma dependência externa | `go list -m` |

## Critérios de Sucesso

- `go test ./pkg/nullable/... -race` passa com 0 falhas e 0 corridas de dados
- `go vet ./pkg/nullable/...` sem saída (0 avisos)
- Todos os 8 critérios de aceitação do PRD verificados e passando
- Nenhuma dependência externa introduzida

## Testes da Tarefa

- [ ] Suite completa: `go test ./pkg/nullable/... -race -v -count=1`
- [ ] Sem testes de integração adicionais nesta tarefa

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO DONE</critical>

## Arquivos Relevantes
- `pkg/nullable/*.go` — todos os arquivos do pacote (verificação)
- `pkg/nullable/*_test.go` — todos os arquivos de teste (execução)
- `tasks/prd-nullable-vos/prd.md` — critérios de aceitação CA-01 a CA-08
- `go.mod` — verificar ausência de novas dependências
