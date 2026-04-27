# Ambiente de validacao observavel

Esta pasta provisiona uma stack LGTM local dedicada para validar a telemetria do modulo `pkg/observability`. O arquivo de entrada e `docker-compose.observability.yml`; ele nao depende do `docker-compose.yml` da raiz.

## Componentes

- `lgtm`: imagem `grafana/otel-lgtm:0.7.5`, com Grafana, Tempo, Loki e Prometheus.
- `otel-collector`: collector auxiliar que recebe OTLP do app e encaminha traces, metricas e logs para o LGTM.
- `postgres`: banco PostgreSQL local usado pelo demo para exercitar instrumentacao real de DB.
- `lgtm-demo`: app HTTP em `pkg/observability/examples/lgtm-demo` que emite span HTTP, span DB via `pgxpool_manager`, metricas e logs correlacionados.

## Iniciar

```bash
docker compose -f deployment/observability/docker-compose.observability.yml up --build -d
```

Servicos publicados:

- Grafana: `http://localhost:3000`
- Demo HTTP: `http://localhost:8088`
- OTLP collector dedicado: `localhost:4319` para gRPC e `localhost:4320` para HTTP/protobuf
- Metricas Prometheus exportadas pelo collector: `http://localhost:8889/metrics`

## Gerar telemetria

Execute requests no app de demonstracao:

```bash
curl -H 'x-request-id: demo-req-1' -H 'correlation-id: demo-corr-1' 'http://localhost:8088/users?id=42'
curl -H 'x-request-id: demo-req-2' -H 'correlation-id: demo-corr-2' 'http://localhost:8088/users?id=missing'
curl 'http://localhost:8088/fail'
```

Esses fluxos produzem:

- traces para `GET /users`, `GET /fail` e `db.client.operation SELECT`;
- metricas `lgtm_demo.http.requests`, `lgtm_demo.http.errors`, `lgtm_demo.http.duration`, `db.client.operation.duration` e `db.client.operation.count`;
- logs com `component=lgtm-demo` e os ids de correlacao quando propagados pelo contexto.

## Inspecionar

No Grafana (`http://localhost:3000`):

- Traces: use Explore com a fonte Tempo e filtre pelo service `devkit-go-lgtm-demo`.
- Logs: use Explore com Loki e procure por `lgtm-demo`.
- Metricas: use Explore com Prometheus e consulte `lgtm_demo_http_requests`.

O collector tambem expoe as metricas convertidas em `http://localhost:8889/metrics`, util para validar rapidamente se o pipeline recebeu dados.

## Encerrar

```bash
docker compose -f deployment/observability/docker-compose.observability.yml down
```

Para remover volumes criados pelo Docker durante testes locais:

```bash
docker compose -f deployment/observability/docker-compose.observability.yml down -v
```

## Operacao

- Mantenha esta stack isolada do compose raiz. Ela serve apenas para validacao observavel local.
- Configure apps externos para enviar OTLP gRPC para `localhost:4319` ou OTLP HTTP para `localhost:4320`.
- Se a UI do Grafana iniciar antes dos dados aparecerem, gere novos requests e aguarde alguns segundos pelo batch do collector.
