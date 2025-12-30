package rabbitmq

import (
	"context"
)

// HealthCheck retorna uma função de health check para integração com HTTP server.
// Compatível com pkg/http_server/server_fiber/health.go
//
// Uso:
//
//	healthChecks := map[string]serverfiber.HealthCheckFunc{
//	    "rabbitmq": client.HealthCheck(),
//	}
//	server := serverfiber.New(
//	    o11y,
//	    serverfiber.WithHealthChecks(healthChecks),
//	)
func (c *Client) HealthCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return c.Ping(ctx)
	}
}
