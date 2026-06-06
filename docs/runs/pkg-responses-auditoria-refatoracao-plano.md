<proposed_plan>
# Auditoria e Plano de Refatoração de `pkg/responses`

## Veredito

**Classificação: `Aprovado com restrições`**

`pkg/responses` é simples, portável e funciona no caminho feliz, mas **não deve ser chamado de production-ready sem restrições**. O principal problema é que `JSON` envia o status HTTP antes de confirmar que o payload é serializável; em erro de `json.Encoder`, ele tenta enviar `500` tarde demais, gerando risco de status enganoso, resposta parcial e contrato inconsistente.

Validações executadas:
- `go test ./pkg/responses -count=1 -race`: passou.
- `go test ./pkg/responses -cover`: 100% statements.
- `go vet ./pkg/responses`: passou.
- `go test ./pkg/responses -bench . -benchmem -run '^$'`: passou.
- `bash scripts/verify-go-mod.sh`: falhou porque o arquivo não existe.
- `bash .agents/skills/go-implementation/scripts/verify-go-mod.sh`: passou.

## Matriz de avaliação

| Dimensão | Nota | Evidências concretas | Riscos | Impacto por porte |
|---|---:|---|---|---|
| API e ergonomia | 3 | API mínima: `JSON`, `Error`, `ErrorWithDetails` em [responses.go](/Users/jailtonjunior/Git/devkit-go/pkg/responses/responses.go:12) | Não retorna erro ao caller; contrato rígido de erro | Pequeno: bom; médio/grande: limitado |
| Corretude HTTP/JSON | 2 | `WriteHeader` ocorre antes de `Encode` em [responses.go](/Users/jailtonjunior/Git/devkit-go/pkg/responses/responses.go:14) | Falha de encode não consegue trocar status para 500 com segurança | Pequeno: tolerável; médio/grande: problemático |
| Robustez operacional | 2 | Usa `log.Printf` em biblioteca em [responses.go](/Users/jailtonjunior/Git/devkit-go/pkg/responses/responses.go:18) | Efeito colateral global, sem `context`, sem logger injetado, sem correlação | Pequeno: aceitável; médio/grande: fraco |
| Performance e eficiência | 3 | Benchmarks: `JSON` 876.9 ns/op, 1152 B/op, 14 allocs/op | Bench usa `httptest.NewRecorder`, não isola custo real de encode/helper | Pequeno/médio: ok; grande: inconclusivo |
| Testabilidade | 3 | Cobertura 100%, race passa | Teste concorrente não espera goroutines; falha de encode não valida corpo/status final | Pequeno: suficiente; médio/grande: lacunas |
| Portabilidade | 4 | Só depende de stdlib: `encoding/json`, `net/http` | Acopla ao `http.ResponseWriter`; não cobre Fiber/gRPC diretamente | Pequeno/médio: bom; grande: parcial |
| Manutenibilidade | 3 | Código curto, pouca superfície | Comentários afirmam produção/thread-safe sem prova; duplicação pequena nos envelopes | Pequeno: bom; médio/grande: precisa contrato mais claro |
| Prontidão para produção | 2 | Testes passam, mas falha operacional central permanece | Pode gerar falso positivo de robustez | Pequeno: restrito; médio/grande: não cravar |

## Evidências objetivas

- `JSON` define `Content-Type` e status antes de serializar. Se `Encode` falhar, `http.Error` é chamado após o status já ter sido escrito, como visto em [responses.go](/Users/jailtonjunior/Git/devkit-go/pkg/responses/responses.go:13).
- O comentário diz que em erro retorna HTTP 500, mas a própria ordem de escrita impede garantir isso depois de `WriteHeader`.
- O teste de dado não serializável valida apenas ausência de panic e mantém expectativa de status original em [responses_test.go](/Users/jailtonjunior/Git/devkit-go/pkg/responses/responses_test.go:94), o que normaliza o comportamento inseguro.
- Há comentários no código e nos testes, violando o requisito futuro de remoção total em `pkg/responses`.
- O README declara RFC 7807 para `Responses`, mas a implementação retorna `{ "message": ... }`, não `type`, `title`, `status`, `detail`, `instance`.
- O pacote não é usado internamente fora dos próprios testes, então não há evidência de integração real com handlers do projeto.

## Lacunas que impedem falso positivo

- Falta teste que prove resposta íntegra quando `json.Marshal`/`Encode` falha.
- Falta decisão explícita: helper deve engolir erro e responder 500, ou deve retornar erro ao handler?
- Falta contrato documentado para envelope de erro: formato atual não é RFC 7807.
- Falta observabilidade adequada para biblioteca reutilizável; `log.Printf` é efeito colateral global.
- Benchmarks atuais medem também `httptest.NewRecorder`, então não provam eficiência do helper isoladamente.
- Testes concorrentes disparam goroutines sem aguardar conclusão.

## Plano de refatoração

| Fase | Objetivo | Mudanças principais | Ganho esperado | Risco | Esforço | Dependências |
|---|---|---|---|---|---|---|
| 1 | Corrigir contrato de serialização | Serializar em buffer antes de `WriteHeader`; só escrever status/corpo após sucesso | Evita status enganoso e resposta parcial | Baixo | Baixo | Nenhuma |
| 2 | Remover efeito colateral de logging | Remover `log.Printf` do pacote ou expor variante que retorna erro | Biblioteca mais portável e previsível | Médio se API mudar | Baixo/médio | Decisão de compatibilidade |
| 3 | Clarificar contrato de erro | Escolher entre manter `{message, details}` ou implementar RFC 7807 de verdade | Alinha README, API e consumidores | Médio | Médio | Decisão de contrato público |
| 4 | Fortalecer testes | Testar falha de serialização, corpo final, headers, goroutines com `sync.WaitGroup` | Reduz falso positivo | Baixo | Baixo | Nenhuma |
| 5 | Limpeza obrigatória | Remover todos os comentários em Go alterado | Cumpre requisito do prompt/governança local | Baixo | Baixo | Nenhuma |

## Refatorações concretas recomendadas

- Antes: `JSON` escreve status e depois codifica direto no `ResponseWriter`.
  Depois: codifica em buffer; se falhar, escreve resposta controlada de erro ou retorna erro ao caller.

- Antes: biblioteca chama `log.Printf` internamente.
  Depois: biblioteca não loga; caller decide logging com contexto, trace e política própria.

- Antes: `Error` e `ErrorWithDetails` usam structs anônimas repetidas.
  Depois: usar tipo interno/privado de envelope se isso reduzir duplicação sem expor abstração desnecessária.

- Antes: teste concorrente não espera as goroutines.
  Depois: usar `sync.WaitGroup` e validar status/corpo por goroutine.

- Antes: README sugere RFC 7807, implementação não entrega RFC 7807.
  Depois: corrigir README ou implementar envelope RFC 7807 explicitamente.

## Críticos para production-ready

- Nenhuma resposta parcial ou status incorreto em falha de serialização.
- Contrato público claro para sucesso e erro, incluindo decisão sobre RFC 7807.
- Sem logging global dentro do pacote reutilizável.
- Testes cobrindo caminho feliz, falha de encode, headers, status, corpo e concorrência com sincronização real.
- Comentários removidos dos arquivos Go alterados, conforme requisito.
- Benchmarks ajustados para medir custo relevante sem vender eficiência além da evidência.

## Ordem sugerida de execução

1. Adicionar testes de regressão para falha de serialização e concorrência sincronizada.
2. Refatorar `JSON` para serializar antes de escrever header/status.
3. Remover `log.Printf` e o import `log`.
4. Consolidar o envelope de erro sem ampliar a API desnecessariamente.
5. Remover todos os comentários de `pkg/responses`.
6. Rodar `gofmt` nos arquivos alterados.
7. Validar com `go test ./pkg/responses -count=1 -race`, `go vet ./pkg/responses` e benchmark direcionado.
</proposed_plan>
