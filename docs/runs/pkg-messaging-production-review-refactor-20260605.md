# Auditoria e Plano de Refatoracao de `pkg/messaging`

## Veredito final

**Nao aprovado para production-ready.** O pacote tem boas bases em contratos simples, retries, DLQ, tracing, metricas e shutdown, mas ha riscos reais que impedem afirmar robustez para producao sem falso positivo. Os principais bloqueadores sao: publish RabbitMQ quebrado quando confirms sao desabilitados, shutdown Kafka com risco de envio em channel fechado, reconexao Kafka que nao altera estado antes de tentar reconnect, e retry RabbitMQ que pode nao incrementar contagem ao usar `Nack(..., true)`. Validacoes executadas: `ai-spec inspect/doctor/verify` OK; `go test ./pkg/messaging/... -count=1` OK; `go test ./pkg/messaging/... -race -count=1` OK. Esses testes nao cobrem broker real de RabbitMQ e cobrem Kafka principalmente em concorrencia/error-channel.

## Matriz de avaliacao

| Dimensao | Nota (0-5) | Evidencias | Risco | Confianca |
|---|---:|---|---|---|
| Robustez | 2 | DLQ Kafka so commita apos DLQ publish em [consumer_dlq.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/consumer_dlq.go:134), mas ha P0/P1 abaixo | Alto | Alta |
| Eficiencia | 3 | Kafka batch aloca com capacidade em [new_producer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/new_producer.go:114); Rabbit batch e sequencial em [publisher.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/rabbitmq/publisher.go:154) | Medio | Alta |
| Ergonomia | 2 | Interface raiz e simples em [publisher.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/publisher.go:6), mas RabbitMQ nao implementa esse contrato | Medio | Alta |
| Concorrencia | 2 | Ha mutex/atomic/wg, mas shutdown Kafka pode fechar `errorCh` antes de desbloquear reader | Alto | Alta |
| Observabilidade | 4 | OTel e propagacao `traceparent` existem em Kafka/RabbitMQ | Medio | Alta |
| Seguranca operacional | 2 | Defaults duraveis existem, mas opcoes perigosas e invalidas passam silenciosamente | Alto | Alta |
| Testabilidade | 2 | Race tests Kafka existem; RabbitMQ so tem `example_test` sem execucao real | Medio | Alta |
| Portabilidade | 3 | APIs nao exigem app framework; acoplamento a `observability` no RabbitMQ reduz reuso | Medio | Media |
| Simplicidade | 2 | 8.623 linhas no pacote; muitos comentarios e duplicacao OTel/DLQ | Medio | Alta |
| Pequena/media/grande escala | 2 | Serve como base para pequenos projetos; nao comprovado para media/grande escala por lacunas de retry, reconnect e testes de broker real | Alto | Alta |

## Achados priorizados

- **P0 - RabbitMQ publish quebra com `WithPublisherConfirms(false)`**: `newChannelPool` nao cria `publisherCh` quando confirms esta off, mas `publishInternal` sempre chama `GetPublisherChannel`; evidencias [channel_pool.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/rabbitmq/channel_pool.go:58), [publisher.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/rabbitmq/publisher.go:104). Impacto: opcao publica valida torna publish inutilizavel. Risco de falso positivo: baixo.
- **P0 - Kafka consumer pode enviar erro para `errorCh` fechado durante shutdown**: `Close` fecha `errorCh` antes de fechar o reader; se `FetchMessage` desbloquear depois, `sendError` usa o channel fechado; evidencias [new_consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/new_consumer.go:276), [new_consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/new_consumer.go:280), [new_consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/new_consumer.go:299), [new_consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/new_consumer.go:522). Impacto: panic em shutdown. Risco: baixo.
- **P1 - Kafka reconnect nao reconecta apos health check falhar**: `reconnectWorker` chama `attemptReconnect`, mas `attemptReconnect` retorna se `connected` ainda esta true; evidencias [client.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/client.go:323), [client.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/client.go:341). Impacto: falsa recuperacao automatica. Risco: baixo.
- **P1 - Kafka DLQ com multiplos handlers pode commitar antes de todos os handlers terminarem**: cada handler bem-sucedido commita dentro de `handleMessageWithRetry`, enquanto o loop continua para outros handlers; evidencias [consumer_dlq.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/consumer_dlq.go:48), [new_consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/kafka/new_consumer.go:442). Impacto: falha posterior pode nao ser redeliverable. Risco: baixo.
- **P1 - RabbitMQ retry pode nao atingir `MaxRetries`**: retry count le `x-death`, mas erro usa `Nack(false, true)` para requeue direto; evidencias [consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/rabbitmq/consumer.go:655), [consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/rabbitmq/consumer.go:623). Impacto: retry potencialmente infinito e worker preso durante backoff. Risco: medio.
- **P2 - Validacao publica incompleta**: `WithQueue`, `WithWorkerPool`, `WithPrefetchCount` aceitam valores invalidos sem falhar cedo; evidencias [consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/rabbitmq/consumer.go:51), [consumer.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/rabbitmq/consumer.go:81). Impacto: erro tardio ou comportamento degradado. Risco: baixo.
- **P2 - Contrato raiz nao cobre RabbitMQ**: `pkg/messaging.Publisher` e `Consumer` nao sao implementados pelo RabbitMQ atual; evidencias [publisher.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/publisher.go:6), [rabbitmq/publisher.go](/Users/jailtonjunior/Git/devkit-go/pkg/messaging/rabbitmq/publisher.go:52). Impacto: menor portabilidade entre brokers. Risco: baixo.

## Comentarios a remover

Prioridade alta: comentarios em codigo Go de `pkg/messaging/kafka/otel.go`, `pkg/messaging/rabbitmq/otel.go`, `pkg/messaging/rabbitmq/client.go`, `pkg/messaging/kafka/options.go`, `pkg/messaging/rabbitmq/consumer.go`, `pkg/messaging/kafka/new_consumer.go`. O inventario por `rg` mostra maior concentracao nesses arquivos, com mais de 80 comentarios em varios deles. A remocao deve preservar apenas nomes, tipos e testes; nao remover README neste refactor, salvo pedido explicito.

## Plano de refatoracao

1. **Fase 1 - Corrigir bloqueadores P0 sem mudar API publica**: criar canal publisher mesmo sem confirms ou separar caminho sem confirms; ajustar shutdown Kafka para fechar/cancelar reader antes de fechar `errorCh`, aguardar goroutines e so entao fechar o channel. Validar com testes unitarios novos e `go test ./pkg/messaging/... -race -count=1`.
2. **Fase 2 - Corrigir lifecycle e commit semantics Kafka**: em health failure, marcar `connected=false` antes do reconnect; trocar sleeps de reconnect por espera cancelavel; mover commit com DLQ para ocorrer uma unica vez apos todos handlers terem sucesso ou apos DLQ confirmada. Validar com testes de health/reconnect via fake ou testcontainer Kafka quando viavel.
3. **Fase 3 - Corrigir retry RabbitMQ**: nao usar `x-death` como contador para requeue direto; escolher um unico modelo: DLX/TTL com `x-death` ou header proprio incrementado antes de re-publicar. Para v1, preferir DLX/TTL ja presente em `dlq.go`; remover bloqueio do worker durante backoff. Validar com teste de unidade do contador e integracao RabbitMQ.
4. **Fase 4 - Ergonomia e contratos**: preservar APIs Kafka; para RabbitMQ, introduzir constructor validado `NewConsumerChecked(...)(*Consumer,error)` e marcar `NewConsumer` atual como compatibilidade, ou preparar breaking change v2 para `NewConsumer(...)(*Consumer,error)`. Adicionar adapter opcional para `pkg/messaging.Publisher` sem forcar unificacao dos modelos.
5. **Fase 5 - Reducao de codigo e comentarios**: remover comentarios Go do escopo alterado, deduplicar conversao de headers e caminhos comuns de instrumentation, sem remover telemetria. Criterio de conclusao: menor LOC nos arquivos tocados, sem perda de comportamento testado.

## Recomendacoes de desenho

- Manter Kafka e RabbitMQ como adapters separados; unificar apenas contratos minimos realmente comuns.
- Usar retries broker-aware: Kafka pode bloquear particao com cautela; RabbitMQ deve preferir DLX/TTL ou republish controlado, nao sleep segurando delivery.
- Tratar observabilidade como capacidade opcional, mas testavel; manter labels com cardinalidade baixa.
- Para production-ready real, adicionar testes de integracao com broker para publish confirm, DLQ, reconnect, shutdown e retry count.

## Nao fazer

- Nao reescrever `pkg/messaging` inteiro.
- Nao remover OTel, DLQ ou retries apenas para reduzir LOC.
- Nao tratar `LogOnlyStrategy` ou `DiscardStrategy` como producao segura.
- Nao afirmar readiness para grande escala sem teste de integracao, race test e cenarios de broker real.
- Nao mudar assinatura publica em massa sem fase v2 explicita.
