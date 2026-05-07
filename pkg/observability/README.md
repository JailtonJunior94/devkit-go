# Observability Toolkit

![devkit-go banner](https://raw.githubusercontent.com/JailtonJunior94/devkit-go/main/assets/banner.png)

O `pkg/observability` é o coração da telemetria no `devkit-go`. Ele fornece uma interface unificada para **Logs Estruturados**, **Métricas** e **Distributed Tracing**, integrada nativamente com o ecossistema OpenTelemetry (OTel).

## Índice

- [Segurança](#segurança)
- [Contexto](#contexto)
- [Instalação](#instalação)
- [Uso](#uso)
    - [Inicializando o Provider](#inicializando-o-provider)
    - [Tracer (Tracing)](#tracer-tracing)
    - [Logger (Logging Estruturado)](#logger-logging-estruturado)
    - [Metrics (Métricas)](#metrics-métricas)
- [Configuração](#configuração)
    - [Segurança e Compliance](#segurança-e-compliance)
- [Performance](#performance)
- [API](#api)
- [Contribuição](#contribuição)
- [Licença](#licença)

## Segurança

Este pacote impõe regras rigorosas para proteção de dados e integridade do sistema (R-SEC-001):
- **TLS Obrigatório em Produção**: Conexões inseguras (`Insecure: true`) são bloqueadas se o ambiente for detectado como `production` ou `prod`.
- **Sanitização de PII**: Opcionalmente permite o redaction de campos sensíveis (senhas, tokens) e truncamento de valores excessivamente longos.
- **TLS Mínimo 1.2**: Se um `TLSConfig` customizado for fornecido, a versão mínima de TLS permitida é 1.2.

## Contexto

A observabilidade moderna exige correlação entre logs, métricas e traces. Este toolkit abstrai a complexidade do OpenTelemetry SDK, oferecendo uma API simplificada que garante que todas as telemetrias compartilhem o mesmo contexto de recurso (Resource Attributes) e propagação de contexto.

## Instalação

```bash
go get github.com/JailtonJunior94/devkit-go/pkg/observability
```

## Uso

### Inicializando o Provider

O `Provider` é o ponto de entrada principal que configura os exportadores OTLP (gRPC ou HTTP).

```go
import (
	"context"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

func main() {
	ctx := context.Background()
	cfg := otel.DefaultConfig("meu-servico")
	cfg.OTLPEndpoint = "otel-collector:4317"
	cfg.Environment = "production"

	provider, err := otel.NewProvider(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer provider.Shutdown(ctx)
	
	// Use provider.Logger(), provider.Tracer(), provider.Metrics()
}
```

### Tracer (Tracing)

Inicie spans para rastrear a execução de funções e chamadas remotas.

```go
tracer := provider.Tracer()
ctx, span := tracer.Start(ctx, "processar-pedido", observability.WithAttributes(
	observability.String("pedido.id", "123"),
))
defer span.End()

// ... lógica de negócio
```

### Logger (Logging Estruturado)

Logs ricos em contexto e com alta performance.

```go
logger := provider.Logger()
logger.Info(ctx, "pedido processado com sucesso",
	observability.String("pedido.id", "123"),
	observability.Float64("pedido.valor", 99.90),
)
```

### Metrics (Métricas)

Colete indicadores chave de performance (KPIs).

```go
metrics := provider.Metrics()
counter := metrics.Counter("pedidos_total", "Total de pedidos processados", "1")
counter.Increment(ctx, observability.String("status", "sucesso"))
```

## Configuração

| Campo | Descrição | Padrão |
|-------|-----------|---------|
| `ServiceName` | Nome do serviço | Obrigatório |
| `OTLPEndpoint`| Endereço do Collector | `localhost:4317` |
| `OTLPProtocol`| Protocolo (`grpc` ou `http`) | `grpc` |
| `Insecure`    | Permite conexão sem TLS (não em prod) | `false` |
| `TraceSampleRate`| Taxa de amostragem (0.0 a 1.0) | `1.0` |
| `ConsoleLog`  | Escreve JSON no stdout (apenas dev) | `false` |
| `MetricExportInterval` | Intervalo de exportação (segundos) | `60` |

### Segurança e Compliance

Para garantir a conformidade com padrões de segurança, o toolkit valida o `TLSConfig`. Recomenda-se o uso de gRPC (porta 4317) para melhor performance em redes internas.

## Performance

O toolkit utiliza um sistema de **Fields com Union Discriminada**. Isso significa que para os tipos mais comuns (`string`, `int`, `int64`, `float64`, `bool`), não há alocação na heap (boxing de interface{}), reduzindo drasticamente a pressão no Garbage Collector em caminhos críticos (hot paths).

## API

A interface `Observability` fornece acesso aos três pilares:
- `Tracer()`: Interface para gestão de spans.
- `Logger()`: Interface para logs estruturados com suporte a níveis (Debug, Info, Warn, Error).
- `Metrics()`: Interface para criação de instrumentos (Counters, Gauges, Histograms).
- `Shutdown(ctx)`: Garante o flush de todos os dados pendentes antes da aplicação encerrar.

## Contribuição

Contribuições são bem-vindas! Certifique-se de que novos recursos incluam testes de validação e sigam os padrões de performance estabelecidos.

## Licença

[MIT](LICENSE)
