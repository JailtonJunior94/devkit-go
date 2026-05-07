# HTTP Server Toolkit

![devkit-go banner](https://raw.githubusercontent.com/JailtonJunior94/devkit-go/main/assets/banner.png)

O `pkg/http_server` é um componente unificado de toolkit HTTP para o `devkit-go`. Ele fornece dois adaptadores concretos — `chi_server` (baseado em net/http + go-chi/chi) e `server_fiber` (baseado em gofiber/fiber/v2) — que compartilham configurações, segurança e auxiliares de observabilidade.

## Índice

- [Segurança](#segurança)
- [Contexto](#contexto)
- [Instalação](#instalação)
- [Uso](#uso)
    - [Escolhendo o Adaptador](#escolhendo-o-adaptador)
    - [Chi Server](#chi-server)
    - [Fiber Server](#fiber-server)
- [Configuração](#configuração)
- [API](#api)
    - [Opções Comuns](#opções-comuns)
- [Comportamentos Específicos](#comportamentos-específicos)
    - [Diferenças no ErrorHandler](#diferenças-no-errorhandler)
    - [Gestão de Timeouts](#gestão-de-timeouts)
- [Contribuição](#contribuição)
- [Licença](#licença)

## Segurança

Este toolkit foi desenhado com segurança em mente (R-SEC-001):
- **Sanitização de Erros**: O `ErrorHandler` padrão registra o erro original detalhadamente mas retorna apenas uma resposta RFC 7807 (`application/problem+json`) para o cliente, ocultando detalhes sensíveis do sistema.
- **Limite de Corpo (Body Limit)**: Proteção contra ataques de negação de serviço (DoS) através da imposição de limites no tamanho do corpo das requisições (padrão 4MB).
- **Headers de Segurança**: Inclusão automática de headers de segurança e suporte robusto para CORS.
- **Timeouts**: Configurações rigorosas de Read, Write e Idle timeouts para evitar vazamento de recursos e conexões pendentes.

## Contexto

O objetivo deste pacote é fornecer uma interface consistente para servidores HTTP, permitindo que desenvolvedores alternem entre `chi` (mais próximo da biblioteca padrão) e `fiber` (focado em performance) com alterações mínimas no código de inicialização.

## Instalação

```bash
go get github.com/JailtonJunior94/devkit-go/pkg/http_server
```

## Uso

### Escolhendo o Adaptador

A escolha entre Chi e Fiber geralmente depende dos requisitos de performance e compatibilidade:
- **Chi**: Ideal se você precisa de compatibilidade total com `net/http` e middlewares padrão da comunidade Go.
- **Fiber**: Ideal para aplicações que exigem altíssima performance e baixa alocação de memória, utilizando uma API inspirada no Express.js.

### Chi Server

```go
import (
	"github.com/JailtonJunior94/devkit-go/pkg/http_server/chi_server"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

func main() {
	o11y := observability.New(...) // Inicialize sua observabilidade
	
	srv, err := chiserver.New(o11y,
		chiserver.WithPort(":3000"),
		chiserver.WithServiceName("my-service"),
	)
	if err != nil {
		panic(err)
	}

	srv.RegisterHandler("GET", "/hello", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("Hello World"))
		return nil
	})

	srv.Run()
}
```

### Fiber Server

```go
import (
	"github.com/JailtonJunior94/devkit-go/pkg/http_server/server_fiber"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/gofiber/fiber/v2"
)

func main() {
	o11y := observability.New(...)
	
	srv, err := serverfiber.New(o11y,
		serverfiber.WithPort(":3000"),
		serverfiber.WithServiceName("my-service"),
	)
	if err != nil {
		panic(err)
	}

	srv.RegisterHandler("GET", "/hello", func(c *fiber.Ctx) error {
		return c.SendString("Hello World")
	})

	srv.Run()
}
```

## Configuração

Ambos os adaptadores utilizam a estrutura `common.Config`:

| Opção | Descrição | Padrão |
|-------|-----------|---------|
| `Address` | Endereço de escuta do servidor | `:8080` |
| `ReadTimeout` | Tempo limite para leitura da requisição | `30s` |
| `WriteTimeout` | Tempo limite para escrita da resposta | `30s` |
| `IdleTimeout` | Tempo limite para conexões ociosas | `120s` |
| `BodyLimit` | Tamanho máximo do corpo da requisição | `4MB` |
| `ServiceName` | Nome do serviço para logs e traces | `unknown-service` |
| `EnableCORS` | Habilita suporte a CORS | `false` |
| `EnableMetrics` | Habilita endpoint `/metrics` | `false` |
| `EnableTracing` | Habilita tracing distribuído OTel | `false` |

## API

### Opções Comuns

Abaixo as opções disponíveis em ambos os adaptadores com semântica idêntica:

- `WithConfig(cfg common.Config)`
- `WithPort(port string)`
- `WithReadTimeout(d time.Duration)`
- `WithWriteTimeout(d time.Duration)`
- `WithIdleTimeout(d time.Duration)`
- `WithShutdownTimeout(d time.Duration)`
- `WithBodyLimit(limit int)`
- `WithCORS(origins string)`
- `WithMetrics()`
- `WithHealthChecks(checks map[string]HealthCheckFunc)`
- `WithTracing()`
- `WithOTelMetrics()`
- `WithErrorHandler(fn ...)` (Veja diferenças abaixo)
- `WithMiddleware(mw ...)`
- `WithRouteTimeout(path string, d time.Duration)`

#### Opções Específicas do Chi
- `WithTimeoutCleanup(d time.Duration)`: Configura quanto tempo o middleware de timeout espera para que a goroutine do handler seja drenada após um 408 ser enviado.

## Comportamentos Específicos

### Diferenças no ErrorHandler

A assinatura do manipulador de erros difere para respeitar o idioma de cada framework:

- **Chi**: `func(ctx context.Context, w http.ResponseWriter, err error)`
- **Fiber**: `func(*fiber.Ctx, error) error` (Padrão nativo do Fiber)

### Gestão de Timeouts

- **Chi**: Implementa timeouts em dois níveis. O `RegisterHandler` aplica um envelope de timeout por rota, permitindo controle granular e suporte a rotas paramétricas.
- **Fiber**: Delega o controle de timeout para o middleware oficial do Fiber. É crucial que os handlers respeitem o `c.UserContext().Done()` para que o timeout funcione corretamente, pois o Fiber não interrompe goroutines pendentes por padrão.

## Contribuição

Sinta-se à vontade para abrir issues ou enviar PRs. Para mudanças maiores, por favor, abra uma issue primeiro para discutir o que você gostaria de mudar.

## Licença

[MIT](LICENSE)
