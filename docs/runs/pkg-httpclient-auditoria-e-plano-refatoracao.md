<proposed_plan>
# Auditoria do pkg/httpclient

## 1. Resumo Executivo

Veredito: `reprovado para uso production-ready`.

O pacote tem partes boas, mas a API pública está dividida em dois clientes incompatíveis, há opção pública de timeout sem efeito no `ObservableClient`, retry duplicado com comportamentos diferentes, limite de resposta declarado mas não aplicado como erro tipado, e testes ainda dependem de rede externa. Para projetos pequenos pode servir com cautela; para médio/grande, eu não recomendaria no estado atual.

Validação executada: `go test ./pkg/httpclient -count=1` passou.

## 2. Mapa do Pacote

- `client.go`: cliente simples baseado em `http.Client`, com interface `HTTPClient`.
- `request.go`: helper genérico para criar request, executar `HTTPClient.Do` e decodificar JSON.
- `retryable.go`: retry legado via `NewHTTPClientRetryable`, `RetryPolicy`, `Option`.
- `observableclient.go`: cliente observável com métodos `Get/Post/Put/Delete/Do`, headers, retry por request e transport chain.
- `transport_retry.go`: retry novo para `ObservableClient`, com backoff, jitter, drain e buffering.
- `transport_observable.go` + `instrumentation.go`: tracing e métricas acoplados ao pacote `observability`.

Sobreposições reais: existem dois modelos de retry (`RetryPolicy` e `NewRetryPolicy`) e dois clientes que não compõem entre si. O helper JSON usa `HTTPClient`, mas `ObservableClient` não implementa essa interface porque seu `Do` recebe `context.Context` e opções extras.

## 3. Avaliação por Critério

- robustez: 2/5. APIs públicas incompatíveis entre si: `HTTPClient.Do(req)` em `client.go:8-9` versus `ObservableClient.Do(ctx, req, opts...)` em `observableclient.go:190`.
- eficiência: 3/5. Há pooling configurado em `observableclient.go:75-95` e drain em `transport_retry.go:186-197`, mas há alocação de `http.Client` e wrappers por request em `observableclient.go:201-209`.
- ergonomia: 2/5. `WithClientTimeout` parece configurar execução, mas o valor não é usado pelo `http.Client` criado em `observableclient.go:203-207`.
- portabilidade: 2/5. O pacote público `pkg/httpclient` importa `pkg/observability` em `observableclient.go:11`, então consumidores simples carregam esse acoplamento ao importar o pacote.
- testabilidade: 3/5. Há testes relevantes com `httptest`, mas `request_test.go:70-79` chama `https://viacep.com.br`, criando dependência externa.
- prontidão para produção: 2/5. O retry novo é mais sólido, mas a API, timeout e duplicidade tornam o pacote imprevisível para SDKs e serviços de maior volume.

## 4. Achados Confirmados

- **`WithClientTimeout` não afeta requests do `ObservableClient`**
  - Evidência: `client_options.go:22-25`, `observableclient.go:96`, `observableclient.go:203-207`.
  - Impacto real: o usuário configura timeout global, mas a execução depende apenas do contexto do request.
  - Por que importa: chamadas sem deadline podem ficar mais tempo que o esperado, apesar da opção pública sugerir proteção.
  - Severidade: alta. Escala afetada: todas.

- **`ObservableClient` não implementa `HTTPClient`, quebrando composição interna do pacote**
  - Evidência: `client.go:8-9`, `request.go:23-24`, `observableclient.go:190`.
  - Impacto real: `MakeRequest` não pode usar o cliente observável/retry do mesmo pacote.
  - Por que importa: força consumidores a escolher entre helper JSON ou observabilidade/retry.
  - Severidade: alta. Escala afetada: todas.

- **Dois sistemas de retry coexistem com contratos diferentes**
  - Evidência: `retryable.go:11-23`, `retry_policy.go:9-16`, `observableclient.go:238-247`.
  - Impacto real: consumidores têm `RetryPolicy` e `NewRetryPolicy`, `Option` e `RequestOption`, `NewHTTPClientRetryable` e `ObservableClient`.
  - Por que importa: aumenta superfície pública e risco de comportamento divergente.
  - Severidade: média. Escala afetada: média e grande.

- **Retry legado não respeita cancelamento durante backoff**
  - Evidência: `retryable.go:72-80`, especialmente `retryable.go:73`; retry novo usa `sleepWithContext` em `transport_retry.go:93` e `transport_retry.go:151-160`.
  - Impacto real: request cancelado pode aguardar `time.Sleep` antes de retornar.
  - Por que importa: degrada shutdown, timeouts e controle de latência.
  - Severidade: média. Escala afetada: média e grande.

- **Limite de resposta declara erro sentinel, mas nunca o retorna**
  - Evidência: `request.go:17-18`, `request.go:50-65`; único uso encontrado de `ErrResponseTooLarge` é a declaração.
  - Impacto real: caller não consegue usar `errors.Is(err, ErrResponseTooLarge)`.
  - Por que importa: falhas por payload grande ficam indistinguíveis de erro JSON/truncamento.
  - Severidade: média. Escala afetada: todas.

- **Teste unitário depende de rede externa**
  - Evidência: `request_test.go:70-79`.
  - Impacto real: teste pode falhar por DNS, rede, rate limit ou indisponibilidade de terceiro.
  - Por que importa: CI fica não determinístico.
  - Severidade: média. Escala afetada: todas.

- **Comentários atuais mascaram desenho inflado**
  - Evidência: blocos extensos em `observableclient.go:14-40`, `request_options.go:32-59`, `transport_retry.go:15-30`, `retry_policy.go:18-68`.
  - Impacto real: documentação tenta explicar complexidade que deveria estar expressa por uma API menor.
  - Por que importa: dificulta manutenção e contraria a diretriz de remover comentários em Go alterado.
  - Severidade: baixa. Escala afetada: todas.

## 5. Pontos Aceitáveis

- `ObservableClient` usa `context.Context` nas entradas de IO: `observableclient.go:119`, `135`, `151`, `166`, `190`.
- Retry novo drena e fecha bodies antes de nova tentativa: `transport_retry.go:75-85`, `186-197`.
- Retry novo evita retry em cancelamento/deadline via policy default: `retry_policy.go:33-41`, `69-76`.
- Configuração de transporte no cliente observável tem pooling e timeouts de transporte razoáveis: `observableclient.go:75-95`.
- Headers por request são simples e previsíveis: `request_options.go:103-127`.

## 6. Riscos que Exigem Validação Adicional

- Overhead real de criar `http.Client`, `observableTransport`, `retryTransport` e `rand.Rand` por request em `observableclient.go:201-209` e `235-255`; precisa benchmark.
- Cardinalidade e sensibilidade de `http.url` em spans: `transport_observable.go:47-50`; precisa política de atributos e dados sensíveis.
- Adequação de `MaxIdleConnsPerHost: 10` e `MaxConnsPerHost: 0` para alto volume: `observableclient.go:77-80`; precisa carga real.
- Concorrência de métricas/tracer fake ou real; código pressupõe thread-safety da interface `observability`.

## 7. Plano de Refatoração

1. **Escolher uma API pública única**
   - Objetivo: fazer `ObservableClient` e helpers JSON comporem.
   - Mudança: alinhar `Do` ao contrato padrão `Do(req *http.Request)` ou criar helpers que aceitem o cliente observável sem interface paralela.
   - Compatibilidade: potencial quebra pública se `ObservableClient.Do` mudar.
   - Pronto quando `MakeRequest` puder usar o cliente principal do pacote.

2. **Remover ou corrigir `WithClientTimeout`**
   - Objetivo: eliminar opção enganosa.
   - Mudança: aplicar timeout via `http.Client.Timeout` de forma explícita ou remover a opção e exigir deadline no contexto.
   - Compatibilidade: se remover, quebra pública; se aplicar, muda comportamento.
   - Pronto quando houver teste provando timeout global ou ausência da opção.

3. **Consolidar retry**
   - Objetivo: manter só um mecanismo.
   - Mudança: migrar ou descontinuar `retryable.go`, `RetryPolicy`, `Option`, `NewHTTPClientRetryable`.
   - Compatibilidade: preferir fase de depreciação antes de remoção pública.
   - Pronto quando existir um único tipo de policy e testes de retry/cancelamento/body.

4. **Corrigir limite de resposta**
   - Objetivo: tornar `ErrResponseTooLarge` observável pelo caller.
   - Mudança: detectar leitura acima do limite antes/depois do decode e retornar o sentinel.
   - Compatibilidade: mudança de erro retornado em payload grande.
   - Pronto quando houver teste com resposta acima do limite e `errors.Is`.

5. **Desacoplar observabilidade do cliente básico**
   - Objetivo: aumentar portabilidade.
   - Mudança: manter cliente base sem dependência de `observability` e aplicar observabilidade como decorator opcional.
   - Compatibilidade: pode preservar construtores atuais como wrappers temporários.
   - Pronto quando consumidor puder usar HTTP client sem provider observability.

6. **Limpar testes e comentários**
   - Objetivo: remover dependência externa e reduzir ruído.
   - Mudança: trocar ViaCEP por `httptest`; remover comentários de código Go alterado, incluindo blocos explicativos extensos.
   - Compatibilidade: sem mudança pública.
   - Pronto quando testes forem determinísticos e arquivos alterados não tiverem comentários Go desnecessários.

## 8. Estado Alvo

- API pública pequena: um cliente principal, um contrato `Do(req *http.Request)`, opções claras e helpers JSON compatíveis com ele.
- Retry como decorator/transport único, context-aware, com política única e nomes sem prefixo `New`.
- Observabilidade opcional, aplicada por composição, sem obrigar projetos pequenos a dependerem dela para uso básico.
- Request helpers com limite real de resposta, erro sentinel retornável e testes locais.
- Arquivos com responsabilidades nítidas: cliente base, retry, observability, JSON helpers e opções.
- Todos os comentários em código Go alterado removidos, exceto documentação pública que a equipe decidir manter explicitamente.

## 9. Recomendação Final

- Projetos pequenos: usável apenas no cliente simples, sem depender da parte observável como API final.
- Projetos médios: não recomendo até corrigir timeout, composição entre APIs e retry duplicado.
- Projetos grandes: não recomendo; a superfície pública atual é inconsistente para adoção como biblioteca compartilhada.
</proposed_plan>
