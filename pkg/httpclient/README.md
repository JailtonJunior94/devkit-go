# HTTP Client com Observabilidade

Cliente HTTP production-ready para Go com distributed tracing automático, métricas integradas e retry configurável por request.

## Visão Geral

### Propósito

O `pkg/httpclient` oferece um cliente HTTP completo e pronto para produção, eliminando boilerplate e garantindo que todas as requisições sejam automaticamente instrumentadas com observabilidade (tracing e métricas) seguindo as melhores práticas.

### Quando Usar

✅ **Use quando você precisa:**
- Fazer requisições HTTP com observabilidade automática (tracing + métricas)
- Distributed tracing entre microserviços (W3C Trace Context)
- Retry configurável e seguro para operações idempotentes
- Métricas detalhadas de latência, erros e status codes
- Context propagation para cancelamento e timeouts
- Cliente thread-safe reutilizável em toda a aplicação

### Quando NÃO Usar

❌ **Não use quando:**
- Você precisa de controle fino sobre cada detalhe do transporte HTTP (use `net/http` direto)
- Sua aplicação não usa OpenTelemetry e não precisa de observabilidade
- Você está fazendo requisições síncronas em loops tight (prefira connection pooling customizado)
- Você precisa de features específicas de clientes especializados (gRPC, WebSocket, etc.)

### Problemas que Resolve

1. **Observabilidade**: Instrumentação manual é trabalhosa e propensa a erros - este cliente faz automaticamente
2. **Context Propagation**: W3C Trace Context headers são injetados automaticamente
3. **Retry Safety**: Configuração de retry por request evita duplicações acidentais em operações não-idempotentes
4. **Memory Safety**: Proteção contra memory exhaustion via buffering limitado
5. **Boilerplate**: Elimina código repetitivo de instrumentação, headers, retry logic
6. **Production Readiness**: Configurações seguras por padrão (timeouts, pooling, compression)

## Princípios de Design

### Clean Code
- Nomes descritivos e autoexplicativos
- Separação clara entre client options (globais) e request options (por request)
- Validações fail-fast com panic para erros de programação
- Documentação clara sobre idempotência e safety

### SOLID
- **Single Responsibility**: Cada transport tem uma única responsabilidade
  - `observableTransport`: Instrumentação
  - `retryTransport`: Retry logic
  - `http.Transport`: Comunicação HTTP
- **Open/Closed**: Extensível via functional options
- **Liskov Substitution**: Todos os transports implementam `http.RoundTripper`
- **Interface Segregation**: Interface mínima `http.RoundTripper`
- **Dependency Inversion**: Depende de `observability.Observability` abstração

### DRY
- Instrumentation logic centralizada
- Retry policies reutilizáveis
- Configuration via functional options pattern

### Baixo Acoplamento
- Não depende diretamente de OpenTelemetry (usa abstração `pkg/observability`)
- Transport chain permite composição flexível
- Request options permitem configuração granular sem afetar outras requests

### Escalabilidade

#### Projetos Pequenos (< 100 req/s)
- Cliente único compartilhado globalmente
- Configuração padrão funciona bem
- Retry apenas onde necessário (GET, PUT, DELETE)

#### Projetos Médios (100-1000 req/s)
- Ajuste connection pooling (`MaxIdleConns`, `MaxConnsPerHost`)
- Monitore métricas de latência e erros
- Implemente circuit breakers para dependências instáveis

#### Projetos Grandes (> 1000 req/s)
- Múltiplos clients para diferentes dependências
- Tune agressivo de timeouts baseado em SLAs
- Considere service mesh (Istio, Linkerd) para observability e retry avançado
- Implemente rate limiting client-side

## Instalação

```bash
go get github.com/JailtonJunior94/devkit-go
```

```go
import (
    "github.com/JailtonJunior94/devkit-go/pkg/httpclient"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"
)
```

### Dependências

**Core:**
- `net/http` (biblioteca padrão)
- `context` (biblioteca padrão)

**Observabilidade:**
- `github.com/JailtonJunior94/devkit-go/pkg/observability` (abstração de tracing/metrics)

## API Pública

### Cliente Principal

#### `ObservableClient`
Cliente HTTP com observabilidade automática e retry configurável.

```go
type ObservableClient struct { /* private */ }

func NewObservableClient(o11y observability.Observability, opts ...ClientOption) (*ObservableClient, error)

func (c *ObservableClient) Get(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error)
func (c *ObservableClient) Post(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error)
func (c *ObservableClient) Put(ctx context.Context, url string, body io.Reader, opts ...RequestOption) (*http.Response, error)
func (c *ObservableClient) Delete(ctx context.Context, url string, opts ...RequestOption) (*http.Response, error)
func (c *ObservableClient) Do(ctx context.Context, req *http.Request, opts ...RequestOption) (*http.Response, error)
```

**Thread-Safe:** ✅ Todas as operações são seguras para uso concorrente

### Client Options (Configuração Global)

```go
// Timeout padrão para todas as requests (padrão: 30s)
func WithClientTimeout(timeout time.Duration) ClientOption

// Tamanho máximo do body para buffering de retry (padrão: 10MB)
func WithMaxBodySize(size int64) ClientOption

// Transport customizado (para TLS config, proxy, etc.)
func WithBaseTransport(transport http.RoundTripper) ClientOption
```

### Request Options (Configuração por Request)

```go
// Habilita retry para esta request
func WithRetry(maxAttempts int, backoff time.Duration, policy NewRetryPolicy) RequestOption

// Adiciona um header
func WithHeader(key, value string) RequestOption

// Adiciona múltiplos headers
func WithHeaders(headers map[string]string) RequestOption
```

### Retry Policies

```go
type NewRetryPolicy func(err error, resp *http.Response) bool

// Retry em erros de rede e 5xx (padrão)
var DefaultNewRetryPolicy NewRetryPolicy

// Retry apenas para métodos idempotentes + 429 rate limiting
var IdempotentNewRetryPolicy NewRetryPolicy

// Nunca faz retry
var NoNewRetryPolicy NewRetryPolicy
```

### Constantes

```go
const (
    DefaultTimeout            = 30 * time.Second
    DefaultMaxRequestBodySize = 10 * 1024 * 1024 // 10MB
    MaxRetryAttempts          = 10
    MaxRetryBackoff           = 10 * time.Second
)

var ErrRequestBodyTooLarge = errors.New("request body exceeds maximum allowed size for retry buffering")
```

## Exemplos de Uso

### Exemplo 1: Quick Start (GET simples)

```go
package main

import (
    "context"
    "fmt"
    "io"
    "log"

    "github.com/JailtonJunior94/devkit-go/pkg/httpclient"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/otel"
)

func main() {
    ctx := context.Background()

    // 1. Inicializar observabilidade
    obs, err := otel.NewProvider(ctx, &otel.Config{
        ServiceName:  "my-service",
        OTLPEndpoint: "tempo:4317",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer obs.Shutdown(ctx)

    // 2. Criar cliente HTTP (uma vez, reusar globalmente)
    client, err := httpclient.NewObservableClient(obs)
    if err != nil {
        log.Fatal(err)
    }

    // 3. Fazer request (automaticamente traced e metricsado)
    resp, err := client.Get(ctx, "https://api.github.com/users/octocat")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    // 4. Processar resposta
    body, _ := io.ReadAll(resp.Body)
    fmt.Printf("Response: %s\n", body)
}
```

**O que acontece automaticamente:**
- Span `http.client.request` criado
- Atributos: `http.method=GET`, `http.url`, `http.status_code=200`
- Métricas: counter, histogram de latência
- W3C Trace Context headers injetados

### Exemplo 2: GET com Retry (Idempotente)

```go
func FetchUserBalance(ctx context.Context, client *httpclient.ObservableClient, userID string) (*Balance, error) {
    url := fmt.Sprintf("https://api.bank.com/users/%s/balance", userID)

    // GET é idempotente - seguro fazer retry
    resp, err := client.Get(ctx, url,
        httpclient.WithRetry(
            3,                                      // 3 tentativas
            time.Second,                            // backoff inicial de 1s
            httpclient.DefaultNewRetryPolicy,       // retry em erros de rede e 5xx
        ),
        httpclient.WithHeader("Authorization", "Bearer token123"),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to fetch balance: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    var balance Balance
    if err := json.NewDecoder(resp.Body).Decode(&balance); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &balance, nil
}
```

**Comportamento do Retry:**
- Tenta até 3 vezes total (1 original + 2 retries)
- Backoff exponencial com jitter: 1s, 2s, 4s
- Span atualizado com `retry.attempt=N` em cada tentativa
- Respeita context cancellation

### Exemplo 3: POST sem Retry (Não-Idempotente)

```go
func CreateTransaction(ctx context.Context, client *httpclient.ObservableClient, tx *Transaction) error {
    payload, err := json.Marshal(tx)
    if err != nil {
        return err
    }

    // POST NÃO é idempotente - não fazer retry!
    // Retry poderia criar transações duplicadas
    resp, err := client.Post(
        ctx,
        "https://api.bank.com/transactions",
        bytes.NewReader(payload),
        httpclient.WithHeader("Content-Type", "application/json"),
        httpclient.WithHeader("Authorization", "Bearer token123"),
        // SEM WithRetry() - não é seguro
    )
    if err != nil {
        return fmt.Errorf("failed to create transaction: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
    }

    return nil
}
```

### Exemplo 4: PUT com Retry (Idempotente por Design)

```go
func UpdateUser(ctx context.Context, client *httpclient.ObservableClient, userID string, user *User) error {
    payload, _ := json.Marshal(user)

    url := fmt.Sprintf("https://api.example.com/users/%s", userID)

    // PUT é idempotente por design - seguro fazer retry
    resp, err := client.Put(
        ctx,
        url,
        bytes.NewReader(payload),
        httpclient.WithRetry(3, time.Second, httpclient.IdempotentNewRetryPolicy),
        httpclient.WithHeader("Content-Type", "application/json"),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("update failed: %d", resp.StatusCode)
    }

    return nil
}
```

### Exemplo 5: Custom Retry Policy

```go
// Retry apenas em 503 Service Unavailable e erros de rede
func customRetryPolicy(err error, resp *http.Response) bool {
    if err != nil {
        // Não retry em context errors
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            return false
        }
        return true // Retry outros erros (network)
    }

    if resp == nil {
        return false
    }

    // Retry apenas em 503
    return resp.StatusCode == http.StatusServiceUnavailable
}

func FetchWithCustomRetry(ctx context.Context, client *httpclient.ObservableClient) error {
    resp, err := client.Get(ctx, url,
        httpclient.WithRetry(5, 2*time.Second, customRetryPolicy),
    )
    // ...
}
```

### Exemplo 6: Client Compartilhado Globalmente

```go
package main

import (
    "github.com/JailtonJunior94/devkit-go/pkg/httpclient"
)

// Cliente global (thread-safe)
var (
    HTTPClient *httpclient.ObservableClient
)

func InitHTTPClient(obs observability.Observability) error {
    client, err := httpclient.NewObservableClient(obs,
        httpclient.WithClientTimeout(30*time.Second),
        httpclient.WithMaxBodySize(50*1024*1024), // 50MB
    )
    if err != nil {
        return err
    }

    HTTPClient = client
    return nil
}

// Usar em qualquer lugar
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    resp, err := HTTPClient.Get(r.Context(), "https://api.example.com/data")
    // ...
}
```

### Exemplo 7: Timeout por Request

```go
func FetchWithTimeout(ctx context.Context, client *httpclient.ObservableClient) error {
    // Criar contexto com timeout específico para esta request
    requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    resp, err := client.Get(requestCtx, "https://slow-api.com/data")
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return fmt.Errorf("request timed out after 5s")
        }
        return err
    }
    defer resp.Body.Close()

    // ...
    return nil
}
```

### Exemplo 8: Custom Transport (TLS Config)

```go
func NewSecureClient(obs observability.Observability) (*httpclient.ObservableClient, error) {
    // TLS config customizado
    tlsConfig := &tls.Config{
        MinVersion:         tls.VersionTLS13,
        InsecureSkipVerify: false, // NUNCA true em produção
        RootCAs:            customCertPool,
    }

    // Transport customizado
    transport := &http.Transport{
        TLSClientConfig:       tlsConfig,
        MaxIdleConns:          100,
        MaxIdleConnsPerHost:   10,
        IdleConnTimeout:       90 * time.Second,
        ResponseHeaderTimeout: 10 * time.Second,
    }

    return httpclient.NewObservableClient(obs,
        httpclient.WithBaseTransport(transport),
    )
}
```

### Exemplo 9: Trace Propagation entre Microserviços

```go
// Service A - HTTP Handler
func (h *Handler) ProcessOrder(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // Context tem span do HTTP handler

    // Criar pedido
    order := createOrder(ctx)

    // Chamar Service B - trace é propagado automaticamente
    if err := h.notifyInventory(ctx, order); err != nil {
        http.Error(w, "Failed to notify inventory", 500)
        return
    }

    w.WriteHeader(http.StatusCreated)
}

func (h *Handler) notifyInventory(ctx context.Context, order *Order) error {
    payload, _ := json.Marshal(order)

    // HTTPClient injeta automaticamente traceparent header
    resp, err := HTTPClient.Post(
        ctx, // Context com span do handler
        "http://inventory-service/orders",
        bytes.NewReader(payload),
    )
    // ...
}

// Service B - Recebe request com trace context
// OpenTelemetry automaticamente extrai traceparent e cria child span
```

**Trace Hierarchy:**
```
Service A: POST /orders (parent span)
  └─ http.client.request → http://inventory-service/orders
      └─ Service B: POST /orders (child span)
```

## Casos Comuns

### Caso 1: Service com Múltiplas Dependências

```go
type ExternalServices struct {
    paymentClient   *httpclient.ObservableClient
    inventoryClient *httpclient.ObservableClient
    notificationClient *httpclient.ObservableClient
}

func NewExternalServices(obs observability.Observability) (*ExternalServices, error) {
    // Cliente para serviço de pagamento (timeout curto)
    paymentClient, err := httpclient.NewObservableClient(obs,
        httpclient.WithClientTimeout(5*time.Second),
    )
    if err != nil {
        return nil, err
    }

    // Cliente para inventário (timeout médio)
    inventoryClient, err := httpclient.NewObservableClient(obs,
        httpclient.WithClientTimeout(10*time.Second),
    )
    if err != nil {
        return nil, err
    }

    // Cliente para notificações (timeout longo, não crítico)
    notificationClient, err := httpclient.NewObservableClient(obs,
        httpclient.WithClientTimeout(30*time.Second),
    )
    if err != nil {
        return nil, err
    }

    return &ExternalServices{
        paymentClient:      paymentClient,
        inventoryClient:    inventoryClient,
        notificationClient: notificationClient,
    }, nil
}
```

### Caso 2: Batch Requests com Context Propagation

```go
func FetchUsersInParallel(ctx context.Context, client *httpclient.ObservableClient, userIDs []string) ([]*User, error) {
    var wg sync.WaitGroup
    usersChan := make(chan *User, len(userIDs))
    errorsChan := make(chan error, len(userIDs))

    for _, id := range userIDs {
        wg.Add(1)
        go func(userID string) {
            defer wg.Done()

            // Context propagado para cada goroutine
            user, err := fetchUser(ctx, client, userID)
            if err != nil {
                errorsChan <- err
                return
            }

            usersChan <- user
        }(id)
    }

    wg.Wait()
    close(usersChan)
    close(errorsChan)

    // Coletar resultados
    var users []*User
    for user := range usersChan {
        users = append(users, user)
    }

    // Verificar erros
    var errs []error
    for err := range errorsChan {
        errs = append(errs, err)
    }

    if len(errs) > 0 {
        return nil, fmt.Errorf("failed to fetch some users: %v", errs)
    }

    return users, nil
}
```

### Caso 3: Conditional Retry baseado em Headers

```go
func shouldRetryBasedOnHeaders(err error, resp *http.Response) bool {
    if err != nil {
        return true // Retry network errors
    }

    if resp == nil {
        return false
    }

    // Retry apenas se servidor indica que é seguro
    retryAfter := resp.Header.Get("X-Retry-Safe")
    if retryAfter == "true" && resp.StatusCode >= 500 {
        return true
    }

    return false
}
```

### Caso 4: Rate Limiting Client-Side

```go
type RateLimitedClient struct {
    client  *httpclient.ObservableClient
    limiter *rate.Limiter
}

func NewRateLimitedClient(client *httpclient.ObservableClient, reqPerSecond int) *RateLimitedClient {
    return &RateLimitedClient{
        client:  client,
        limiter: rate.NewLimiter(rate.Limit(reqPerSecond), reqPerSecond),
    }
}

func (r *RateLimitedClient) Get(ctx context.Context, url string, opts ...httpclient.RequestOption) (*http.Response, error) {
    // Aguardar rate limiter
    if err := r.limiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limit wait: %w", err)
    }

    return r.client.Get(ctx, url, opts...)
}
```

## Padrões Recomendados

### Padrão 1: Dependency Injection

```go
type OrderService struct {
    httpClient      *httpclient.ObservableClient
    inventoryURL    string
    paymentURL      string
}

func NewOrderService(client *httpclient.ObservableClient, inventoryURL, paymentURL string) *OrderService {
    return &OrderService{
        httpClient:   client,
        inventoryURL: inventoryURL,
        paymentURL:   paymentURL,
    }
}

func (s *OrderService) CreateOrder(ctx context.Context, order *Order) error {
    // Verificar estoque
    if err := s.checkInventory(ctx, order); err != nil {
        return err
    }

    // Processar pagamento
    if err := s.processPayment(ctx, order); err != nil {
        return err
    }

    return nil
}
```

### Padrão 2: Factory para Clientes Especializados

```go
type ClientFactory struct {
    obs observability.Observability
}

func NewClientFactory(obs observability.Observability) *ClientFactory {
    return &ClientFactory{obs: obs}
}

func (f *ClientFactory) NewPaymentClient() (*httpclient.ObservableClient, error) {
    return httpclient.NewObservableClient(f.obs,
        httpclient.WithClientTimeout(5*time.Second),
        httpclient.WithMaxBodySize(1*1024*1024), // 1MB
    )
}

func (f *ClientFactory) NewFileUploadClient() (*httpclient.ObservableClient, error) {
    return httpclient.NewObservableClient(f.obs,
        httpclient.WithClientTimeout(5*time.Minute),
        httpclient.WithMaxBodySize(100*1024*1024), // 100MB
    )
}
```

### Padrão 3: Wrapper para API Específica

```go
type GitHubClient struct {
    httpClient *httpclient.ObservableClient
    baseURL    string
    token      string
}

func NewGitHubClient(client *httpclient.ObservableClient, token string) *GitHubClient {
    return &GitHubClient{
        httpClient: client,
        baseURL:    "https://api.github.com",
        token:      token,
    }
}

func (g *GitHubClient) GetUser(ctx context.Context, username string) (*GitHubUser, error) {
    url := fmt.Sprintf("%s/users/%s", g.baseURL, username)

    resp, err := g.httpClient.Get(ctx, url,
        httpclient.WithHeader("Authorization", "Bearer "+g.token),
        httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var user GitHubUser
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, err
    }

    return &user, nil
}
```

## Anti-Patterns a Evitar

### ❌ Anti-Pattern 1: Criar Cliente por Request

```go
// ERRADO - cria cliente novo a cada request
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    client, _ := httpclient.NewObservableClient(obs) // NÃO FAÇA ISSO
    resp, _ := client.Get(r.Context(), url)
    // ...
}
```

**Solução:**
```go
// CORRETO - criar uma vez, reusar sempre
var HTTPClient *httpclient.ObservableClient

func init() {
    HTTPClient, _ = httpclient.NewObservableClient(obs)
}

func HandleRequest(w http.ResponseWriter, r *http.Request) {
    resp, _ := HTTPClient.Get(r.Context(), url) // Reusar
    // ...
}
```

### ❌ Anti-Pattern 2: Retry em POST não-idempotente

```go
// ERRADO - pode criar transações duplicadas!
resp, err := client.Post(ctx, "https://api.bank.com/transactions", body,
    httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy), // PERIGOSO
)
```

**Solução:**
```go
// CORRETO - POST sem retry (não é idempotente)
resp, err := client.Post(ctx, "https://api.bank.com/transactions", body)

// OU: Use idempotency key
resp, err := client.Post(ctx, "https://api.bank.com/transactions", body,
    httpclient.WithHeader("Idempotency-Key", uuid.NewString()),
    httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy), // Agora é seguro
)
```

### ❌ Anti-Pattern 3: Ignorar Status Code

```go
// ERRADO - não verifica status code
resp, _ := client.Get(ctx, url)
body, _ := io.ReadAll(resp.Body)
fmt.Println(body) // Pode ser página de erro HTML!
```

**Solução:**
```go
// CORRETO
resp, err := client.Get(ctx, url)
if err != nil {
    return fmt.Errorf("request failed: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(resp.Body)
    return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
}

body, _ := io.ReadAll(resp.Body)
// Processar body
```

### ❌ Anti-Pattern 4: Não Fechar Response Body

```go
// ERRADO - memory leak!
resp, _ := client.Get(ctx, url)
body, _ := io.ReadAll(resp.Body) // Não fecha body
```

**Solução:**
```go
// CORRETO
resp, err := client.Get(ctx, url)
if err != nil {
    return err
}
defer resp.Body.Close() // SEMPRE defer Close()

body, _ := io.ReadAll(resp.Body)
```

### ❌ Anti-Pattern 5: Context.Background() em Handlers

```go
// ERRADO - perde trace propagation
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    resp, _ := client.Get(context.Background(), url) // Perde trace!
    // ...
}
```

**Solução:**
```go
// CORRETO - propaga context do request
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    resp, _ := client.Get(r.Context(), url) // Trace propagado automaticamente
    // ...
}
```

## Escalabilidade

### Projetos Pequenos (< 50 req/s)
- Cliente único global com configuração padrão
- Retry apenas em GETs críticos
- Timeout padrão (30s) é suficiente

**Setup:**
```go
client, _ := httpclient.NewObservableClient(obs)
```

### Projetos Médios (50-500 req/s)
- Tune connection pooling:
```go
transport := &http.Transport{
    MaxIdleConns:        200,  // Aumentar para reutilizar mais conexões
    MaxIdleConnsPerHost: 20,   // Conexões idle por host
    IdleConnTimeout:     90 * time.Second,
}

client, _ := httpclient.NewObservableClient(obs,
    httpclient.WithBaseTransport(transport),
)
```
- Monitore métricas:
  - `http.client.request.duration`: P50, P95, P99
  - `http.client.request.errors`: Taxa de erro
- Implemente timeouts agressivos baseados em SLAs

### Projetos Grandes (> 500 req/s)
- Clientes separados por dependência
- Circuit breakers (ex: `github.com/sony/gobreaker`)
- Service mesh (Istio, Linkerd) para retry/timeout avançado
- Rate limiting client-side

**Setup:**
```go
// Cliente para dependência crítica (timeout curto)
criticalClient, _ := httpclient.NewObservableClient(obs,
    httpclient.WithClientTimeout(2*time.Second),
)

// Cliente para dependência não-crítica (timeout longo)
nonCriticalClient, _ := httpclient.NewObservableClient(obs,
    httpclient.WithClientTimeout(30*time.Second),
)
```

**Métricas para monitorar:**
- Request rate (req/s)
- Latência (P50, P95, P99)
- Taxa de erro (%)
- Taxa de retry (%)
- Connection pool stats (idle, in-use)

## Testabilidade

### Mock de ObservableClient

```go
type MockHTTPClient struct {
    GetFunc    func(ctx context.Context, url string, opts ...httpclient.RequestOption) (*http.Response, error)
    PostFunc   func(ctx context.Context, url string, body io.Reader, opts ...httpclient.RequestOption) (*http.Response, error)
    // ...
}

func (m *MockHTTPClient) Get(ctx context.Context, url string, opts ...httpclient.RequestOption) (*http.Response, error) {
    if m.GetFunc != nil {
        return m.GetFunc(ctx, url, opts...)
    }
    return nil, fmt.Errorf("not implemented")
}

// Uso em testes
func TestFetchUser(t *testing.T) {
    mockClient := &MockHTTPClient{
        GetFunc: func(ctx context.Context, url string, opts ...httpclient.RequestOption) (*http.Response, error) {
            return &http.Response{
                StatusCode: 200,
                Body:       io.NopCloser(strings.NewReader(`{"id": "123", "name": "John"}`)),
            }, nil
        },
    }

    user, err := FetchUser(ctx, mockClient, "123")
    if err != nil {
        t.Fatalf("FetchUser failed: %v", err)
    }

    if user.Name != "John" {
        t.Errorf("Expected name John, got %s", user.Name)
    }
}
```

### Testes com httptest

```go
func TestHTTPClient_Integration(t *testing.T) {
    // Criar servidor HTTP de teste
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/users/123" {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte(`{"id": "123", "name": "Test User"}`))
            return
        }
        w.WriteHeader(http.StatusNotFound)
    }))
    defer server.Close()

    // Usar cliente real com observability fake
    obs := fake.NewProvider()
    client, _ := httpclient.NewObservableClient(obs)

    // Fazer request para servidor de teste
    resp, err := client.Get(context.Background(), server.URL+"/users/123")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected status 200, got %d", resp.StatusCode)
    }
}
```

## FAQ / Troubleshooting

### P: Por que `ObservableClient` e não apenas `http.Client`?

**R:** `ObservableClient` adiciona:
1. **Observability automática**: Tracing e métricas sem código adicional
2. **Context propagation**: W3C Trace Context headers injetados automaticamente
3. **Retry safety**: Configuração por request evita acidentes
4. **Memory safety**: Proteção contra body buffering excessivo

Se você não precisa disso, use `net/http` direto.

### P: Retry funciona com request bodies?

**R:** Sim, mas com limitações:
- Body é bufferizado em memória até `maxBodySize` (padrão: 10MB)
- Se body > `maxBodySize`, retorna `ErrRequestBodyTooLarge`
- Para uploads grandes, não use retry ou aumente `WithMaxBodySize()`

```go
client, _ := httpclient.NewObservableClient(obs,
    httpclient.WithMaxBodySize(100*1024*1024), // 100MB
)
```

### P: Como debugar requests?

**R:**

**Opção 1: Ver spans no Jaeger/Tempo**
```bash
# Spans contêm: method, url, status_code, retry_attempt
# Buscar por span "http.client.request"
```

**Opção 2: Logging customizado**
```go
type LoggingTransport struct {
    base http.RoundTripper
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    log.Printf("[HTTP] %s %s", req.Method, req.URL)
    resp, err := t.base.RoundTrip(req)
    if err != nil {
        log.Printf("[HTTP] ERROR: %v", err)
    } else {
        log.Printf("[HTTP] %d", resp.StatusCode)
    }
    return resp, err
}

client, _ := httpclient.NewObservableClient(obs,
    httpclient.WithBaseTransport(&LoggingTransport{base: http.DefaultTransport}),
)
```

### P: Como configurar proxy?

**R:**
```go
transport := &http.Transport{
    Proxy: http.ProxyFromEnvironment, // Usa HTTP_PROXY, HTTPS_PROXY env vars
    // Ou proxy específico:
    // Proxy: http.ProxyURL(&url.URL{
    //     Scheme: "http",
    //     Host:   "proxy.company.com:8080",
    // }),
}

client, _ := httpclient.NewObservableClient(obs,
    httpclient.WithBaseTransport(transport),
)
```

### P: Como configurar TLS/mTLS?

**R:**
```go
cert, _ := tls.LoadX509KeyPair("client.crt", "client.key")
caCert, _ := os.ReadFile("ca.crt")
caCertPool := x509.NewCertPool()
caCertPool.AppendCertsFromPEM(caCert)

tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    RootCAs:      caCertPool,
    MinVersion:   tls.VersionTLS13,
}

transport := &http.Transport{
    TLSClientConfig: tlsConfig,
}

client, _ := httpclient.NewObservableClient(obs,
    httpclient.WithBaseTransport(transport),
)
```

### P: Retry está causando duplicações, o que fazer?

**R:**

1. **Verificar se operação é idempotente:**
   - GET, PUT, DELETE: Geralmente idempotente
   - POST: Geralmente NÃO idempotente

2. **Usar idempotency keys:**
```go
idempotencyKey := uuid.NewString()
resp, err := client.Post(ctx, url, body,
    httpclient.WithHeader("Idempotency-Key", idempotencyKey),
    httpclient.WithRetry(3, time.Second, httpclient.DefaultNewRetryPolicy),
)
```

3. **Remover retry de operações não-idempotentes:**
```go
// POST sem retry
resp, err := client.Post(ctx, url, body) // Sem WithRetry()
```

### P: Como medir latência de requests?

**R:** Métricas automáticas:

**Prometheus:**
```promql
# P95 latency
histogram_quantile(0.95, rate(http_client_request_duration_bucket[5m]))

# Erro rate
rate(http_client_request_errors_total[5m])

# Taxa de sucesso
rate(http_client_request_count{status_code=~"2.."}[5m])
```

**Grafana Dashboard:**
```json
{
  "title": "HTTP Client Metrics",
  "panels": [
    {
      "title": "Request Latency",
      "targets": [{"expr": "histogram_quantile(0.95, rate(http_client_request_duration_bucket[5m]))"}]
    },
    {
      "title": "Error Rate",
      "targets": [{"expr": "rate(http_client_request_errors_total[5m])"}]
    }
  ]
}
```

### P: Cliente funciona com gRPC?

**R:** Não. Este cliente é apenas para HTTP/1.1 e HTTP/2. Para gRPC, use:
```go
import "google.golang.org/grpc"
```

### P: Como implementar circuit breaker?

**R:**
```go
import "github.com/sony/gobreaker"

type CircuitBreakerClient struct {
    client  *httpclient.ObservableClient
    breaker *gobreaker.CircuitBreaker
}

func NewCircuitBreakerClient(client *httpclient.ObservableClient) *CircuitBreakerClient {
    settings := gobreaker.Settings{
        Name:        "http-client",
        MaxRequests: 3,
        Interval:    10 * time.Second,
        Timeout:     60 * time.Second,
    }

    return &CircuitBreakerClient{
        client:  client,
        breaker: gobreaker.NewCircuitBreaker(settings),
    }
}

func (c *CircuitBreakerClient) Get(ctx context.Context, url string, opts ...httpclient.RequestOption) (*http.Response, error) {
    result, err := c.breaker.Execute(func() (any, error) {
        return c.client.Get(ctx, url, opts...)
    })

    if err != nil {
        return nil, err
    }

    return result.(*http.Response), nil
}
```

## Licença

Este pacote faz parte do projeto devkit-go.
