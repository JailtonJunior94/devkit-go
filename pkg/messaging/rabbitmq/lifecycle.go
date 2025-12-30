package rabbitmq

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

// Shutdown encerra o cliente RabbitMQ de forma graciosa.
// Aguarda operações em andamento finalizarem antes de fechar a conexão.
// Respeita o timeout definido no contexto.
//
// Comportamento:
//   - Marca o cliente como fechado para prevenir novas operações
//   - Aguarda publishers/consumers finalizarem (respeitando ctx)
//   - Fecha channels e conexão de forma ordenada
//   - Fecha apenas uma vez (idempotente)
//   - É thread-safe e pode ser chamado concorrentemente
//
// Parâmetros:
//   - ctx: contexto com timeout/deadline para o shutdown
//
// Retorna erro se:
//   - O contexto expirar antes de completar shutdown
//   - Ocorrer erro ao fechar conexão/channel
//
// Exemplo:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	if err := client.Shutdown(ctx); err != nil {
//	    log.Printf("Erro no shutdown: %v", err)
//	}
func (c *Client) Shutdown(ctx context.Context) error {
	var shutdownErr error

	c.shutdownOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()

		c.observability.Logger().Info(ctx, "initiating RabbitMQ client shutdown",
			observability.String("service", c.config.ServiceName),
		)

		if err := c.connMgr.close(ctx); err != nil {
			c.observability.Logger().Error(ctx, "error shutting down connection manager",
				observability.Error(err),
			)
			shutdownErr = err
		}

		if shutdownErr == nil {
			c.observability.Logger().Info(ctx, "RabbitMQ client shutdown completed successfully")
		} else {
			c.observability.Logger().Error(ctx, "RabbitMQ client shutdown completed with errors",
				observability.Error(shutdownErr),
			)
		}
	})

	return shutdownErr
}
